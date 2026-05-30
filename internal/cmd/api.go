package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agynio/outline-cli/internal/outline"
	"github.com/spf13/cobra"
)

type methodSpec struct {
	Group       string
	Action      string
	Method      string
	Use         string
	Short       string
	Flags       []fieldSpec
	Required    []string
	Destructive bool
	Binary      binarySpec
	Multipart   multipartSpec
	Alias       string
	Args        []argBinding
	Transform   payloadTransform
}

type fieldSpec struct {
	Name        string
	PayloadName string
	Type        fieldType
	Usage       string
	Aliases     []string
}

type fieldType string

const (
	fieldString     fieldType = "string"
	fieldBool       fieldType = "bool"
	fieldInt        fieldType = "int"
	fieldStringList fieldType = "stringList"
	fieldJSON       fieldType = "json"
)

type binarySpec struct {
	Enabled bool
	Accept  string
}

type multipartSpec struct {
	Enabled     bool
	FileField   string
	FileFlag    string
	ContentFlag string
}

type argBinding struct {
	Name        string
	PayloadName string
}

type methodValues struct {
	strings     map[string]*string
	bools       map[string]*bool
	ints        map[string]*int
	stringLists map[string]*[]string
}

type confirmationValues struct {
	yes bool
}

type payloadTransform func(*cobra.Command, methodSpec, map[string]any) (map[string]any, error)

var outlineMethods = []methodSpec{
	{Group: "access-requests", Action: "create", Method: "accessRequests.create", Short: "Create an access request", Flags: fields(s("document", "documentId", "Document ID")), Required: []string{"document"}},
	{Group: "access-requests", Action: "info", Method: "accessRequests.info", Short: "Retrieve an access request", Flags: fields(s("id", "id", "Access request ID"), s("document", "documentId", "Document ID"))},
	{Group: "access-requests", Action: "approve", Method: "accessRequests.approve", Short: "Approve an access request", Flags: fields(s("id", "id", "Access request ID"), s("permission", "permission", "Permission")), Required: []string{"id"}},
	{Group: "access-requests", Action: "dismiss", Method: "accessRequests.dismiss", Short: "Dismiss an access request", Flags: fields(s("id", "id", "Access request ID")), Required: []string{"id"}},

	{Group: "attachments", Action: "create", Method: "attachments.create", Short: "Create signed attachment upload instructions", Flags: fields(s("name", "name", "Attachment filename"), s("document", "documentId", "Document ID"), s("content-type", "contentType", "MIME content type"), i("size", "size", "Attachment size in bytes"), s("preset", "preset", "Attachment preset")), Required: []string{"name", "content-type", "size"}},
	{Group: "attachments", Action: "redirect", Method: "attachments.redirect", Short: "Retrieve an attachment", Flags: fields(s("id", "id", "Attachment ID")), Required: []string{"id"}},
	{Group: "attachments", Action: "delete", Method: "attachments.delete", Short: "Delete an attachment", Flags: fields(s("id", "id", "Attachment ID")), Required: []string{"id"}, Destructive: true},

	{Group: "collections", Action: "create", Method: "collections.create", Short: "Create a collection", Flags: fields(s("name", "name", "Collection name"), s("description", "description", "Description"), s("permission", "permission", "Permission"), s("icon", "icon", "Icon"), s("color", "color", "Color"), b("sharing", "sharing", "Allow sharing")), Required: []string{"name"}},
	{Group: "collections", Action: "update", Method: "collections.update", Short: "Update a collection", Flags: fields(s("id", "id", "Collection ID"), s("name", "name", "Collection name"), s("description", "description", "Description"), s("permission", "permission", "Permission"), s("icon", "icon", "Icon"), s("color", "color", "Color"), b("sharing", "sharing", "Allow sharing")), Required: []string{"id"}},
	{Group: "collections", Action: "add-user", Method: "collections.add_user", Short: "Add a collection user", Flags: fields(s("id", "id", "Collection ID"), s("user", "userId", "User ID"), s("permission", "permission", "Permission")), Required: []string{"id", "user"}},
	{Group: "collections", Action: "remove-user", Method: "collections.remove_user", Short: "Remove a collection user", Flags: fields(s("id", "id", "Collection ID"), s("user", "userId", "User ID")), Required: []string{"id", "user"}, Destructive: true},
	{Group: "collections", Action: "memberships", Method: "collections.memberships", Short: "List collection memberships", Flags: fields(s("id", "id", "Collection ID"), s("query", "query", "Query"), s("permission", "permission", "Permission"), limitFlag(), offsetFlag()), Required: []string{"id"}},
	{Group: "collections", Action: "add-group", Method: "collections.add_group", Short: "Add a group to a collection", Flags: fields(s("id", "id", "Collection ID"), s("group", "groupId", "Group ID"), s("permission", "permission", "Permission")), Required: []string{"id", "group"}},
	{Group: "collections", Action: "remove-group", Method: "collections.remove_group", Short: "Remove a collection group", Flags: fields(s("id", "id", "Collection ID"), s("group", "groupId", "Group ID")), Required: []string{"id", "group"}, Destructive: true},
	{Group: "collections", Action: "group-memberships", Method: "collections.group_memberships", Short: "List collection group memberships", Flags: fields(s("id", "id", "Collection ID"), s("query", "query", "Query"), s("permission", "permission", "Permission"), limitFlag(), offsetFlag()), Required: []string{"id"}},
	{Group: "collections", Action: "delete", Method: "collections.delete", Short: "Delete a collection", Flags: fields(s("id", "id", "Collection ID")), Required: []string{"id"}, Destructive: true},
	{Group: "collections", Action: "export", Method: "collections.export", Short: "Export a collection", Flags: fields(s("id", "id", "Collection ID"), s("format", "format", "Export format")), Required: []string{"id"}},
	{Group: "collections", Action: "export-all", Method: "collections.export_all", Short: "Export all collections", Flags: fields(s("format", "format", "Export format"), b("include-attachments", "includeAttachments", "Include attachments"), b("include-private", "includePrivate", "Include private collections"))},

	{Group: "comments", Action: "info", Method: "comments.info", Short: "Retrieve a comment", Flags: fields(s("id", "id", "Comment ID"), b("include-anchor-text", "includeAnchorText", "Include anchor text")), Required: []string{"id"}},
	{Group: "comments", Action: "update", Method: "comments.update", Short: "Update a comment", Flags: fields(s("id", "id", "Comment ID"), j("data-json", "data", "Valid ProseMirror comment document JSON")), Required: []string{"id", "data-json"}},
	{Group: "comments", Action: "delete", Method: "comments.delete", Short: "Delete a comment", Flags: fields(s("id", "id", "Comment ID")), Required: []string{"id"}, Destructive: true},
	{Group: "data-attributes", Action: "info", Method: "dataAttributes.info", Short: "Retrieve a data attribute", Flags: fields(s("id", "id", "Data attribute ID")), Required: []string{"id"}},
	{Group: "data-attributes", Action: "list", Method: "dataAttributes.list", Short: "List data attributes", Flags: fields(limitFlag(), offsetFlag())},
	{Group: "data-attributes", Action: "create", Method: "dataAttributes.create", Short: "Create a data attribute", Flags: fields(s("name", "name", "Name"), s("description", "description", "Description"), s("data-type", "dataType", "Data type"), j("options-json", "options", "Options JSON"), b("pinned", "pinned", "Pinned")), Required: []string{"name", "data-type"}},
	{Group: "data-attributes", Action: "update", Method: "dataAttributes.update", Short: "Update a data attribute", Flags: fields(s("id", "id", "Data attribute ID"), s("name", "name", "Name"), s("description", "description", "Description"), j("options-json", "options", "Options JSON"), b("pinned", "pinned", "Pinned")), Required: []string{"id", "name"}},
	{Group: "data-attributes", Action: "delete", Method: "dataAttributes.delete", Short: "Delete a data attribute", Flags: fields(s("id", "id", "Data attribute ID")), Required: []string{"id"}, Destructive: true},

	{Group: "documents", Action: "archived", Method: "documents.archived", Short: "List archived documents", Flags: commonListFields()},
	{Group: "documents", Action: "deleted", Method: "documents.deleted", Short: "List deleted documents", Flags: commonListFields()},
	{Group: "documents", Action: "viewed", Method: "documents.viewed", Short: "List recently viewed documents", Flags: commonListFields()},
	{Group: "documents", Action: "drafts", Method: "documents.drafts", Short: "List draft documents", Flags: commonListFields()},
	{Group: "documents", Action: "insights", Method: "documents.insights", Short: "Retrieve document insights", Flags: fields(s("id", "id", "Document ID"), s("start-date", "startDate", "Start date"), s("end-date", "endDate", "End date")), Required: []string{"id"}},
	{Group: "documents", Action: "users", Method: "documents.users", Short: "List document users", Flags: fields(s("id", "id", "Document ID"), s("query", "query", "Query"), s("user", "userId", "User ID"), limitFlag(), offsetFlag()), Required: []string{"id"}},
	{Group: "documents", Action: "documents", Method: "documents.documents", Short: "Retrieve document child structure", Flags: fields(s("id", "id", "Document ID")), Required: []string{"id"}},
	{Group: "documents", Action: "export", Method: "documents.export", Short: "Export a document", Flags: fields(s("id", "id", "Document ID"), s("paper-size", "paperSize", "PDF paper size"), i("signed-urls", "signedUrls", "Signed URL lifetime seconds"), b("include-child-documents", "includeChildDocuments", "Include child documents"), s("out", "out", "Output file"), s("accept", "accept", "Accept header")), Required: []string{"id"}, Binary: binarySpec{Enabled: true, Accept: "application/json"}},
	{Group: "documents", Action: "restore", Method: "documents.restore", Short: "Restore a document", Flags: fields(s("id", "id", "Document ID"), s("collection", "collectionId", "Collection ID"), s("revision", "revisionId", "Revision ID"), b("use-latest-revision", "useLatestRevision", "Resolve and restore the latest revision")), Required: []string{"id"}, Transform: transformDocumentsRestore},
	{Group: "documents", Action: "search-titles", Method: "documents.search_titles", Short: "Search document titles", Flags: searchFields(), Required: []string{"query"}},
	{Group: "documents", Action: "answer-question", Method: "documents.answerQuestion", Short: "Query documents with natural language", Flags: fields(s("query", "query", "Question"), s("user", "userId", "User ID"), s("collection", "collectionId", "Collection ID"), s("document", "documentId", "Document ID"), s("status-filter", "statusFilter", "Status filter"), s("date-filter", "dateFilter", "Date filter")), Required: []string{"query"}},
	{Group: "documents", Action: "templatize", Method: "documents.templatize", Short: "Create a template from a document", Flags: fields(s("id", "id", "Document ID"), s("collection", "collectionId", "Collection ID"), b("publish", "publish", "Publish")), Required: []string{"id"}},
	{Group: "documents", Action: "duplicate", Method: "documents.duplicate", Short: "Duplicate a document", Flags: fields(s("id", "id", "Document ID"), s("title", "title", "Title"), b("recursive", "recursive", "Recursive"), b("publish", "publish", "Publish"), s("collection", "collectionId", "Collection ID"), s("parent-document", "parentDocumentId", "Parent document ID")), Required: []string{"id"}},
	{Group: "documents", Action: "move", Method: "documents.move", Short: "Move a document", Flags: fields(s("id", "id", "Document ID"), s("collection", "collectionId", "Collection ID"), s("parent-document", "parentDocumentId", "Parent document ID"), i("index", "index", "Index")), Required: []string{"id"}},
	{Group: "documents", Action: "archive", Method: "documents.archive", Short: "Archive a document", Flags: fields(s("id", "id", "Document ID")), Required: []string{"id"}},
	{Group: "documents", Action: "delete", Method: "documents.delete", Short: "Delete a document", Flags: fields(s("id", "id", "Document ID"), b("permanent", "permanent", "Permanently delete")), Required: []string{"id"}, Destructive: true},
	{Group: "documents", Action: "unpublish", Method: "documents.unpublish", Short: "Unpublish a document", Flags: fields(s("id", "id", "Document ID"), b("detach", "detach", "Detach")), Required: []string{"id"}},
	{Group: "documents", Action: "import", Method: "documents.import", Short: "Import a file as a document", Flags: fields(s("file", "file", "File to import"), s("collection", "collectionId", "Collection ID"), s("parent-document", "parentDocumentId", "Parent document ID"), b("publish", "publish", "Publish"), s("content-type", "contentType", "File content type")), Required: []string{"file"}, Multipart: multipartSpec{Enabled: true, FileField: "file", FileFlag: "file", ContentFlag: "content-type"}},
	{Group: "documents", Action: "add-user", Method: "documents.add_user", Short: "Add a document user", Flags: fields(s("id", "id", "Document ID"), s("user", "userId", "User ID"), s("permission", "permission", "Permission")), Required: []string{"id", "user"}, Transform: transformDocumentsAddUser},
	{Group: "documents", Action: "remove-user", Method: "documents.remove_user", Short: "Remove a document user", Flags: fields(s("id", "id", "Document ID"), s("user", "userId", "User ID")), Required: []string{"id", "user"}, Destructive: true},
	{Group: "documents", Action: "memberships", Method: "documents.memberships", Short: "List document memberships", Flags: fields(s("id", "id", "Document ID"), s("query", "query", "Query"), s("permission", "permission", "Permission"), limitFlag(), offsetFlag()), Required: []string{"id"}},
	{Group: "documents", Action: "add-group", Method: "documents.add_group", Short: "Add a group to a document", Flags: fields(s("id", "id", "Document ID"), s("group", "groupId", "Group ID"), s("permission", "permission", "Permission")), Required: []string{"id", "group"}},
	{Group: "documents", Action: "remove-group", Method: "documents.remove_group", Short: "Remove a group from a document", Flags: fields(s("id", "id", "Document ID"), s("group", "groupId", "Group ID")), Required: []string{"id", "group"}, Destructive: true},
	{Group: "documents", Action: "group-memberships", Method: "documents.group_memberships", Short: "List document group memberships", Flags: fields(s("id", "id", "Document ID"), s("query", "query", "Query"), s("permission", "permission", "Permission"), limitFlag(), offsetFlag())},
	{Group: "documents", Action: "empty-trash", Method: "documents.empty_trash", Short: "Empty trash", Destructive: true},

	{Group: "events", Action: "list", Method: "events.list", Short: "List events", Flags: fields(s("name", "name", "Event name"), s("actor", "actorId", "Actor ID"), s("document", "documentId", "Document ID"), s("collection", "collectionId", "Collection ID"), b("audit-log", "auditLog", "Audit log"), limitFlag(), offsetFlag())},

	{Group: "file-operations", Action: "info", Method: "fileOperations.info", Short: "Retrieve a file operation", Flags: fields(s("id", "id", "File operation ID")), Required: []string{"id"}},
	{Group: "file-operations", Action: "list", Method: "fileOperations.list", Short: "List file operations", Flags: fields(s("type", "type", "Operation type"), sortFlag(), directionFlag(), limitFlag(), offsetFlag())},
	{Group: "file-operations", Action: "redirect", Method: "fileOperations.redirect", Short: "Retrieve file operation file", Flags: fields(s("id", "id", "File operation ID")), Required: []string{"id"}, Binary: binarySpec{Enabled: true, Accept: "application/octet-stream"}},
	{Group: "file-operations", Action: "download", Method: "fileOperations.redirect", Short: "Download a file operation file", Flags: fields(s("id", "id", "File operation ID"), s("out", "out", "Output file")), Required: []string{"id", "out"}, Binary: binarySpec{Enabled: true, Accept: "application/octet-stream"}},
	{Group: "file-operations", Action: "delete", Method: "fileOperations.delete", Short: "Delete a file operation", Flags: fields(s("id", "id", "File operation ID")), Required: []string{"id"}, Destructive: true},

	{Group: "groups", Action: "info", Method: "groups.info", Short: "Retrieve a group", Flags: fields(s("id", "id", "Group ID")), Required: []string{"id"}},
	{Group: "groups", Action: "list", Method: "groups.list", Short: "List groups", Flags: fields(s("query", "query", "Query"), sortFlag(), directionFlag(), limitFlag(), offsetFlag())},
	{Group: "groups", Action: "create", Method: "groups.create", Short: "Create a group", Flags: fields(s("name", "name", "Group name")), Required: []string{"name"}},
	{Group: "groups", Action: "update", Method: "groups.update", Short: "Update a group", Flags: fields(s("id", "id", "Group ID"), s("name", "name", "Group name")), Required: []string{"id", "name"}},
	{Group: "groups", Action: "delete", Method: "groups.delete", Short: "Delete a group", Flags: fields(s("id", "id", "Group ID")), Required: []string{"id"}, Destructive: true},
	{Group: "groups", Action: "memberships", Method: "groups.memberships", Short: "List group members", Flags: fields(s("id", "id", "Group ID"), s("query", "query", "Query"), limitFlag(), offsetFlag()), Required: []string{"id"}},
	{Group: "groups", Action: "add-user", Method: "groups.add_user", Short: "Add a group member", Flags: fields(s("id", "id", "Group ID"), s("user", "userId", "User ID")), Required: []string{"id", "user"}},
	{Group: "groups", Action: "remove-user", Method: "groups.remove_user", Short: "Remove a group member", Flags: fields(s("id", "id", "Group ID"), s("user", "userId", "User ID")), Required: []string{"id", "user"}, Destructive: true},

	{Group: "oauth-authentications", Action: "list", Method: "oauthAuthentications.list", Short: "List OAuth authentications", Flags: fields(limitFlag(), offsetFlag())},
	{Group: "oauth-authentications", Action: "delete", Method: "oauthAuthentications.delete", Short: "Delete an OAuth authentication", Flags: fields(s("oauth-client", "oauthClientId", "OAuth client ID"), s("scope", "scope", "Scope")), Required: []string{"oauth-client"}, Destructive: true},

	{Group: "oauth-clients", Action: "info", Method: "oauthClients.info", Short: "Retrieve an OAuth client", Flags: fields(s("id", "id", "OAuth client ID"), s("client", "clientId", "OAuth client public ID"))},
	{Group: "oauth-clients", Action: "list", Method: "oauthClients.list", Short: "List OAuth clients", Flags: fields(limitFlag(), offsetFlag())},
	{Group: "oauth-clients", Action: "create", Method: "oauthClients.create", Short: "Create an OAuth client", Flags: fields(s("name", "name", "Name"), s("description", "description", "Description"), s("developer-name", "developerName", "Developer name"), s("developer-url", "developerUrl", "Developer URL"), s("avatar-url", "avatarUrl", "Avatar URL"), sl("redirect-uri", "redirectUris", "Redirect URI"), b("published", "published", "Published")), Required: []string{"name", "redirect-uri"}},
	{Group: "oauth-clients", Action: "update", Method: "oauthClients.update", Short: "Update an OAuth client", Flags: fields(s("id", "id", "OAuth client ID"), s("name", "name", "Name"), s("description", "description", "Description"), s("developer-name", "developerName", "Developer name"), s("developer-url", "developerUrl", "Developer URL"), s("avatar-url", "avatarUrl", "Avatar URL"), sl("redirect-uri", "redirectUris", "Redirect URI"), b("published", "published", "Published")), Required: []string{"id"}},
	{Group: "oauth-clients", Action: "rotate-secret", Method: "oauthClients.rotate_secret", Short: "Rotate an OAuth client secret", Flags: fields(s("id", "id", "OAuth client ID")), Required: []string{"id"}, Destructive: true},
	{Group: "oauth-clients", Action: "delete", Method: "oauthClients.delete", Short: "Delete an OAuth client", Flags: fields(s("id", "id", "OAuth client ID")), Required: []string{"id"}, Destructive: true},

	{Group: "revisions", Action: "info", Method: "revisions.info", Short: "Retrieve a revision", Flags: fields(s("id", "id", "Revision ID")), Required: []string{"id"}},
	{Group: "revisions", Action: "list", Method: "revisions.list", Short: "List revisions", Flags: fields(s("document", "documentId", "Document ID"), s("sort", "sort", "Sort"), s("direction", "direction", "Direction"), limitFlag(), offsetFlag())},

	{Group: "shares", Action: "info", Method: "shares.info", Short: "Retrieve a share", Flags: fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")), Transform: transformSharesInfo},
	{Group: "shares", Action: "list", Method: "shares.list", Short: "List shares", Flags: fields(s("document", "documentId", "Document ID"), s("collection", "collectionId", "Collection ID"), s("query", "query", "Query"), s("sort", "sort", "Sort"), s("direction", "direction", "Direction"), limitFlag(), offsetFlag())},
	{Group: "shares", Action: "create", Method: "shares.create", Short: "Create a share", Flags: fields(s("document", "documentId", "Document ID"), s("collection", "collectionId", "Collection ID"))},
	{Group: "shares", Action: "update", Method: "shares.update", Short: "Update a share", Flags: fields(s("id", "id", "Share ID"), b("published", "published", "Published"), s("title", "title", "Title"), s("icon-url", "iconUrl", "Icon URL")), Required: []string{"id", "published"}},
	{Group: "shares", Action: "revoke", Method: "shares.revoke", Short: "Revoke a share", Flags: fields(s("id", "id", "Share ID")), Required: []string{"id"}, Destructive: true},

	{Group: "stars", Action: "create", Method: "stars.create", Short: "Create a star", Flags: fields(s("document", "documentId", "Document ID"), s("collection", "collectionId", "Collection ID"), i("index", "index", "Index"))},
	{Group: "stars", Action: "list", Method: "stars.list", Short: "List stars", Flags: fields(limitFlag(), offsetFlag())},
	{Group: "stars", Action: "update", Method: "stars.update", Short: "Update a star", Flags: fields(s("id", "id", "Star ID"), s("index", "index", "Index")), Required: []string{"id", "index"}},
	{Group: "stars", Action: "delete", Method: "stars.delete", Short: "Delete a star", Flags: fields(s("id", "id", "Star ID")), Required: []string{"id"}, Destructive: true},

	{Group: "templates", Action: "create", Method: "templates.create", Short: "Create a template", Flags: fields(s("id", "id", "Template ID"), s("title", "title", "Title"), j("data-json", "data", "Template data JSON"), s("icon", "icon", "Icon"), s("color", "color", "Color"), s("collection", "collectionId", "Collection ID")), Required: []string{"title", "data-json"}},
	{Group: "templates", Action: "list", Method: "templates.list", Short: "List templates", Flags: commonListFields()},
	{Group: "templates", Action: "info", Method: "templates.info", Short: "Retrieve a template", Flags: fields(s("id", "id", "Template ID")), Required: []string{"id"}},
	{Group: "templates", Action: "update", Method: "templates.update", Short: "Update a template", Flags: fields(s("id", "id", "Template ID"), s("title", "title", "Title"), j("data-json", "data", "Template data JSON"), s("icon", "icon", "Icon"), s("color", "color", "Color"), b("full-width", "fullWidth", "Full width"), s("collection", "collectionId", "Collection ID")), Required: []string{"id"}},
	{Group: "templates", Action: "delete", Method: "templates.delete", Short: "Delete a template", Flags: fields(s("id", "id", "Template ID")), Required: []string{"id"}, Destructive: true},
	{Group: "templates", Action: "restore", Method: "templates.restore", Short: "Restore a template", Flags: fields(s("id", "id", "Template ID")), Required: []string{"id"}},
	{Group: "templates", Action: "duplicate", Method: "templates.duplicate", Short: "Duplicate a template", Flags: fields(s("id", "id", "Template ID"), s("title", "title", "Title"), s("collection", "collectionId", "Collection ID")), Required: []string{"id"}},

	{Group: "users", Action: "invite", Method: "users.invite", Short: "Invite users", Flags: fields(j("invites-json", "invites", "Invites JSON array"), b("suppress-email", "suppressEmail", "Suppress email")), Required: []string{"invites-json"}},
	{Group: "users", Action: "info", Method: "users.info", Short: "Retrieve a user", Flags: fields(s("id", "id", "User ID")), Required: []string{"id"}},
	{Group: "users", Action: "list", Method: "users.list", Short: "List users", Flags: fields(s("query", "query", "Query"), s("filter", "filter", "Filter"), s("sort", "sort", "Sort"), s("direction", "direction", "Direction"), limitFlag(), offsetFlag())},
	{Group: "users", Action: "update", Method: "users.update", Short: "Update current user", Flags: fields(s("name", "name", "Name"), s("language", "language", "Language"), s("avatar-url", "avatarUrl", "Avatar URL"))},
	{Group: "users", Action: "update-role", Method: "users.update_role", Short: "Change a user's role", Flags: fields(s("id", "id", "User ID"), s("role", "role", "Role")), Required: []string{"id", "role"}, Destructive: true},
	{Group: "users", Action: "suspend", Method: "users.suspend", Short: "Suspend a user", Flags: fields(s("id", "id", "User ID")), Required: []string{"id"}, Destructive: true},
	{Group: "users", Action: "activate", Method: "users.activate", Short: "Activate a user", Flags: fields(s("id", "id", "User ID")), Required: []string{"id"}},
	{Group: "users", Action: "delete", Method: "users.delete", Short: "Delete a user", Flags: fields(s("id", "id", "User ID")), Required: []string{"id"}, Destructive: true},

	{Group: "views", Action: "list", Method: "views.list", Short: "List views", Flags: fields(s("document-id", "documentId", "Document ID"), b("include-suspended", "includeSuspended", "Include suspended users"), limitFlag(), offsetFlag()), Required: []string{"document-id"}},
	{Group: "views", Action: "create", Method: "views.create", Short: "Create a view", Flags: fields(s("document-id", "documentId", "Document ID")), Required: []string{"document-id"}},
}

func fields(values ...fieldSpec) []fieldSpec { return values }
func s(name, payloadName, usage string) fieldSpec {
	return fieldSpec{Name: name, PayloadName: payloadName, Type: fieldString, Usage: usage, Aliases: aliasesFor(name)}
}
func b(name, payloadName, usage string) fieldSpec {
	return fieldSpec{Name: name, PayloadName: payloadName, Type: fieldBool, Usage: usage, Aliases: aliasesFor(name)}
}
func i(name, payloadName, usage string) fieldSpec {
	return fieldSpec{Name: name, PayloadName: payloadName, Type: fieldInt, Usage: usage, Aliases: aliasesFor(name)}
}
func sl(name, payloadName, usage string) fieldSpec {
	return fieldSpec{Name: name, PayloadName: payloadName, Type: fieldStringList, Usage: usage, Aliases: aliasesFor(name)}
}
func j(name, payloadName, usage string) fieldSpec {
	return fieldSpec{Name: name, PayloadName: payloadName, Type: fieldJSON, Usage: usage, Aliases: aliasesFor(name)}
}

func aliasesFor(name string) []string {
	switch name {
	case "collection":
		return []string{"collection-id"}
	case "collection-id":
		return []string{"collection"}
	case "document":
		return []string{"document-id"}
	case "document-id":
		return []string{"document"}
	case "group":
		return []string{"group-id"}
	case "group-id":
		return []string{"group"}
	case "user":
		return []string{"user-id"}
	case "user-id":
		return []string{"user"}
	case "revision":
		return []string{"revision-id"}
	case "revision-id":
		return []string{"revision"}
	default:
		return nil
	}
}
func limitFlag() fieldSpec     { return i("limit", "limit", "Pagination limit") }
func offsetFlag() fieldSpec    { return i("offset", "offset", "Pagination offset") }
func sortFlag() fieldSpec      { return s("sort", "sort", "Sort field") }
func directionFlag() fieldSpec { return s("direction", "direction", "Sort direction") }
func commonListFields() []fieldSpec {
	return fields(s("collection", "collectionId", "Collection ID"), s("user", "userId", "User ID"), s("query", "query", "Query"), sortFlag(), directionFlag(), limitFlag(), offsetFlag())
}
func searchFields() []fieldSpec {
	return fields(s("query", "query", "Search query"), s("user", "userId", "User ID"), s("collection", "collectionId", "Collection ID"), s("document", "documentId", "Document ID"), sl("status-filter", "statusFilter", "Status filter"), s("date-filter", "dateFilter", "Date filter"), s("share", "shareId", "Share ID"), i("snippet-min-words", "snippetMinWords", "Snippet minimum words"), i("snippet-max-words", "snippetMaxWords", "Snippet maximum words"), sortFlag(), directionFlag(), limitFlag(), offsetFlag())
}

func newAPIRootCommands() []*cobra.Command {
	commands := []*cobra.Command{newAuthCmd(), newCacheCmd(), newCollectionsCmd(), newCommentsCmd(), newDocumentsCmd()}
	groups := map[string]*cobra.Command{}
	for _, command := range commands {
		groups[command.Name()] = command
	}
	for _, spec := range outlineMethods {
		group := groups[spec.Group]
		if group == nil {
			group = &cobra.Command{Use: spec.Group, Short: "Manage Outline " + strings.ReplaceAll(spec.Group, "-", " ")}
			groups[spec.Group] = group
			commands = append(commands, group)
		}
		if hasCommand(group, spec.Action) {
			continue
		}
		group.AddCommand(newMethodCommand(spec))
	}
	if attachments := groups["attachments"]; attachments != nil {
		attachments.AddCommand(newAttachmentUploadCmd())
	}
	return commands
}

func newAttachmentUploadCmd() *cobra.Command {
	return newMethodCommand(methodSpec{Group: "attachments", Action: "upload", Method: "attachments.create", Short: "Create attachment instructions and upload a file", Flags: fields(s("file", "file", "File to upload"), s("name", "name", "Attachment filename"), s("document", "documentId", "Document ID"), s("content-type", "contentType", "MIME content type"), s("preset", "preset", "Attachment preset")), Required: []string{"file"}})
}

func hasCommand(group *cobra.Command, name string) bool {
	for _, command := range group.Commands() {
		if command.Name() == name {
			return true
		}
	}
	return false
}

func newMethodCommand(spec methodSpec) *cobra.Command {
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	confirm := confirmationValues{}
	cmd := &cobra.Command{
		Use:   methodUse(spec),
		Short: spec.Short,
		Args:  cobra.ExactArgs(len(spec.Args)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMethodCommand(cmd, spec, values, confirm, args)
		},
	}
	for _, field := range spec.Flags {
		registerFieldFlag(cmd, values, field)
	}
	if spec.Destructive {
		cmd.Flags().BoolVar(&confirm.yes, "yes", false, "Confirm this destructive or security-sensitive action")
	}
	return cmd
}

func methodUse(spec methodSpec) string {
	if spec.Use != "" {
		return spec.Use
	}
	if len(spec.Args) == 0 {
		return spec.Action
	}
	parts := []string{spec.Action}
	for _, arg := range spec.Args {
		parts = append(parts, "<"+arg.Name+">")
	}
	return strings.Join(parts, " ")
}

func registerFieldFlag(cmd *cobra.Command, values methodValues, field fieldSpec) {
	switch field.Type {
	case fieldString, fieldJSON:
		value := ""
		values.strings[field.Name] = &value
		cmd.Flags().StringVar(&value, field.Name, "", field.Usage)
		for _, alias := range field.Aliases {
			aliasValue := ""
			values.strings[alias] = &aliasValue
			cmd.Flags().StringVar(&aliasValue, alias, "", field.Usage+" (alias)")
		}
	case fieldBool:
		value := false
		values.bools[field.Name] = &value
		cmd.Flags().BoolVar(&value, field.Name, false, field.Usage)
		for _, alias := range field.Aliases {
			aliasValue := false
			values.bools[alias] = &aliasValue
			cmd.Flags().BoolVar(&aliasValue, alias, false, field.Usage+" (alias)")
		}
	case fieldInt:
		value := 0
		values.ints[field.Name] = &value
		cmd.Flags().IntVar(&value, field.Name, 0, field.Usage)
		for _, alias := range field.Aliases {
			aliasValue := 0
			values.ints[alias] = &aliasValue
			cmd.Flags().IntVar(&aliasValue, alias, 0, field.Usage+" (alias)")
		}
	case fieldStringList:
		value := []string{}
		values.stringLists[field.Name] = &value
		cmd.Flags().StringArrayVar(&value, field.Name, nil, field.Usage)
		for _, alias := range field.Aliases {
			aliasValue := []string{}
			values.stringLists[alias] = &aliasValue
			cmd.Flags().StringArrayVar(&aliasValue, alias, nil, field.Usage+" (alias)")
		}
	default:
		panic(fmt.Sprintf("unsupported field type %q", field.Type))
	}
}

func runMethodCommand(cmd *cobra.Command, spec methodSpec, values methodValues, confirm confirmationValues, args []string) error {
	payload, err := buildPayload(cmd, spec, values, args)
	if err != nil {
		return err
	}
	if spec.Transform != nil {
		payload, err = spec.Transform(cmd, spec, payload)
		if err != nil {
			return err
		}
	}
	if spec.Destructive {
		if err := confirmAction(cmd, confirm.yes, spec.Method); err != nil {
			return err
		}
	}
	if spec.Group == "attachments" && spec.Action == "upload" {
		return runAttachmentUpload(cmd, payload, values)
	}
	if spec.Multipart.Enabled {
		return runMultipartMethod(cmd, spec, payload, values)
	}
	if spec.Binary.Enabled {
		return runBinaryMethod(cmd, spec, payload, values)
	}
	if spec.Method == "shares.info" {
		return runSharesInfo(cmd, payload)
	}
	if spec.Method == "shares.create" {
		return runSharesCreate(cmd, payload)
	}
	return runRPC(cmd, spec.Method, payload)
}

func runSharesCreate(cmd *cobra.Command, payload map[string]any) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	response, err := runContext.Client.Post(cmd.Context(), "shares.create", payload)
	if err != nil {
		return err
	}
	if err := cacheSharesFromResponseWithDocumentID(runContext.BaseURL, response, fmt.Sprint(payload["documentId"])); err != nil {
		return err
	}
	return printResponse(cmd, outline.ResponseData(response))
}

func runSharesInfo(cmd *cobra.Command, payload map[string]any) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	response, err := runContext.Client.Post(cmd.Context(), "shares.info", payload)
	if err == nil {
		if payloadHasValue(payload, "documentId") {
			if err := cacheSharesFromResponseWithDocumentID(runContext.BaseURL, response, fmt.Sprint(payload["documentId"])); err != nil {
				return err
			}
		}
		return printResponse(cmd, outline.ResponseData(response))
	}
	shareID := strings.TrimSpace(fmt.Sprint(payload["id"]))
	documentID := strings.TrimSpace(fmt.Sprint(payload["documentId"]))
	if !outline.IsNotFound(err) || shareID == "" || shareID == "<nil>" {
		return err
	}
	if documentID == "" || documentID == "<nil>" {
		var inferred bool
		documentID, inferred, err = documentIDForShareCache(cmd, shareID)
		if err != nil {
			return err
		}
		if !inferred {
			documentID, inferred, err = documentIDForShare(cmd, shareID)
			if err != nil {
				return err
			}
		}
		if !inferred {
			documentID, inferred, err = documentIDForSharePage(cmd, shareID)
			if err != nil {
				return err
			}
		}
		if !inferred {
			return fmt.Errorf("shares.info by id returned not found and share %s could not be resolved through cache, shares.list, or share page", shareID)
		}
	}
	share, ok, err := shareForDocumentID(cmd, shareID, documentID)
	if err != nil {
		return err
	}
	if ok {
		return printResponse(cmd, share)
	}

	if payloadHasValue(payload, "documentId") {
		return fmt.Errorf("share %s not found in document share response", shareID)
	}
	documentID, inferred, err := documentIDForShare(cmd, shareID)
	if err != nil {
		return err
	}
	if inferred {
		share, ok, err = shareForDocumentID(cmd, shareID, documentID)
		if err != nil {
			return err
		}
		if ok {
			return printResponse(cmd, share)
		}
	}
	documentID, inferred, err = documentIDForSharePage(cmd, shareID)
	if err != nil {
		return err
	}
	if inferred {
		share, ok, err = shareForDocumentID(cmd, shareID, documentID)
		if err != nil {
			return err
		}
		if ok {
			return printResponse(cmd, share)
		}
	}
	return fmt.Errorf("shares.info by id returned not found and share %s could not be resolved through cache, shares.list, or share page", shareID)
}

func shareForDocumentID(cmd *cobra.Command, shareID string, documentID string) (any, bool, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return nil, false, err
	}
	response, err := runContext.Client.Post(cmd.Context(), "shares.info", map[string]any{"documentId": documentID})
	if err != nil {
		if outline.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := cacheSharesFromResponseWithDocumentID(runContext.BaseURL, response, documentID); err != nil {
		return nil, false, err
	}
	share, err := shareFromDocumentShareResponse(response, shareID)
	if err != nil {
		return nil, false, nil
	}
	return share, true, nil
}

func documentIDForShareCache(cmd *cobra.Command, shareID string) (string, bool, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return "", false, err
	}
	return lookupCachedShareDocument(runContext.BaseURL, shareID)
}

func documentIDForShare(cmd *cobra.Command, shareID string) (string, bool, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return "", false, err
	}
	response, err := runContext.Client.Post(cmd.Context(), "shares.list", map[string]any{})
	if err != nil {
		return "", false, err
	}
	share, ok := findShareInResponse(response, shareID)
	if !ok {
		return "", false, nil
	}
	documentID := strings.TrimSpace(fmt.Sprint(share["documentId"]))
	if documentID == "" || documentID == "<nil>" {
		return "", false, fmt.Errorf("share %s found in shares.list but documentId is missing", shareID)
	}
	return documentID, true, nil
}

func documentIDForSharePage(cmd *cobra.Command, shareID string) (string, bool, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return "", false, err
	}
	sharePage, ok, err := fetchSharePage(cmd.Context(), runContext.BaseURL, shareID)
	if err != nil || !ok {
		return "", ok, err
	}
	if documentID, ok := documentIDFromShareHTML(sharePage); ok {
		return documentID, true, nil
	}
	urlID, ok := documentURLIDFromShareHTML(sharePage)
	if !ok {
		return "", false, nil
	}
	response, err := runContext.Client.Post(cmd.Context(), "documents.info", map[string]any{"id": urlID})
	if err != nil {
		return "", false, err
	}
	documentID, ok := documentIDFromDocumentsInfo(response)
	if !ok {
		return "", false, fmt.Errorf("documents.info response for urlId %s did not include document id", urlID)
	}
	return documentID, true, nil
}

func fetchSharePage(ctx context.Context, apiBaseURL string, shareID string) (string, bool, error) {
	publicBase := publicBaseURL(apiBaseURL)
	if publicBase == "" {
		return "", false, nil
	}
	shareURL, err := url.JoinPath(publicBase, "s", shareID)
	if err != nil {
		return "", false, fmt.Errorf("build share page URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shareURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("create share page request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("fetch share page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("fetch share page failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("read share page: %w", err)
	}
	return string(body), true, nil
}

func publicBaseURL(apiBaseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	return strings.TrimSuffix(trimmed, "/api")
}

var (
	shareHTMLDocumentIDPattern = regexp.MustCompile(`(?i)"documentId"\s*:\s*"([^"]+)"`)
	shareHTMLURLIDPattern      = regexp.MustCompile(`(?i)"urlId"\s*:\s*"([^"]+)"`)
	shareHTMLDocURLPattern     = regexp.MustCompile(`(?i)(?:https?://[^"'<>\s]+)?/(?:s/[^"'<>\s/]+/)?doc/[^"'<>\s]+`)
)

func documentIDFromShareHTML(pageHTML string) (string, bool) {
	decoded := html.UnescapeString(pageHTML)
	match := shareHTMLDocumentIDPattern.FindStringSubmatch(decoded)
	if len(match) != 2 {
		return "", false
	}
	documentID := strings.TrimSpace(match[1])
	return documentID, documentID != ""
}

func documentURLIDFromShareHTML(pageHTML string) (string, bool) {
	decoded := html.UnescapeString(pageHTML)
	match := shareHTMLURLIDPattern.FindStringSubmatch(decoded)
	if len(match) == 2 {
		urlID := strings.TrimSpace(match[1])
		if urlID != "" {
			return urlID, true
		}
	}
	for _, documentURL := range shareHTMLDocURLPattern.FindAllString(decoded, -1) {
		if urlID, ok := urlIDFromDocumentURL(documentURL); ok {
			return urlID, true
		}
	}
	return "", false
}

func urlIDFromDocumentURL(documentURL string) (string, bool) {
	parsedURL, err := url.Parse(documentURL)
	if err != nil {
		return "", false
	}
	documentPath := parsedURL.Path
	if documentPath == "" {
		documentPath = documentURL
	}
	parts := strings.Split(strings.Trim(documentPath, "/"), "/")
	docIndex := -1
	for index, part := range parts {
		if part == "doc" {
			docIndex = index
			break
		}
	}
	if docIndex < 0 || docIndex == len(parts)-1 {
		return "", false
	}
	slug := strings.TrimSpace(parts[docIndex+1])
	separator := strings.LastIndex(slug, "-")
	if separator < 0 || separator == len(slug)-1 {
		return "", false
	}
	urlID := slug[separator+1:]
	return urlID, urlID != ""
}

func documentIDFromDocumentsInfo(response map[string]any) (string, bool) {
	data, ok := outline.ResponseData(response).(map[string]any)
	if !ok {
		return "", false
	}
	documentID := strings.TrimSpace(fmt.Sprint(data["id"]))
	return documentID, documentID != "" && documentID != "<nil>"
}

func cacheSharesFromResponse(baseURL string, response map[string]any) error {
	return cacheSharesFromResponseWithDocumentID(baseURL, response, "")
}

func cacheSharesFromResponseWithDocumentID(baseURL string, response map[string]any, fallbackDocumentID string) error {
	fallbackDocumentID = strings.TrimSpace(fallbackDocumentID)
	for _, share := range sharesInResponse(response) {
		shareID := strings.TrimSpace(fmt.Sprint(share["id"]))
		documentID := strings.TrimSpace(fmt.Sprint(share["documentId"]))
		if (documentID == "" || documentID == "<nil>") && fallbackDocumentID != "<nil>" {
			documentID = fallbackDocumentID
		}
		if err := cacheShareDocument(baseURL, shareID, documentID); err != nil {
			return err
		}
	}
	return nil
}

func sharesInResponse(response map[string]any) []map[string]any {
	data := outline.ResponseData(response)
	shares := []map[string]any{}
	if share, ok := data.(map[string]any); ok {
		if strings.TrimSpace(fmt.Sprint(share["id"])) != "" {
			shares = append(shares, share)
		}
		if nested, ok := share["shares"].([]any); ok {
			shares = append(shares, shareMaps(nested)...)
		}
		return shares
	}
	if list, ok := data.([]any); ok {
		return shareMaps(list)
	}
	return shares
}

func shareMaps(values []any) []map[string]any {
	shares := []map[string]any{}
	for _, value := range values {
		share, ok := value.(map[string]any)
		if ok {
			shares = append(shares, share)
		}
	}
	return shares
}

func shareFromDocumentShareResponse(response map[string]any, shareID string) (any, error) {
	share, ok := findShareInResponse(response, shareID)
	if !ok {
		return nil, fmt.Errorf("share %s not found in document share response", shareID)
	}
	return share, nil
}

func findShareInResponse(response map[string]any, shareID string) (map[string]any, bool) {
	data := outline.ResponseData(response)
	if share, ok := data.(map[string]any); ok {
		if strings.TrimSpace(fmt.Sprint(share["id"])) == shareID {
			return share, true
		}
		if shares, ok := share["shares"].([]any); ok {
			return matchingShare(shares, shareID)
		}
		return nil, false
	}
	if shares, ok := data.([]any); ok {
		return matchingShare(shares, shareID)
	}
	return nil, false
}

func matchingShare(shares []any, shareID string) (map[string]any, bool) {
	for _, candidate := range shares {
		share, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(share["id"])) == shareID {
			return share, true
		}
	}
	return nil, false
}

func transformSharesInfo(cmd *cobra.Command, spec methodSpec, payload map[string]any) (map[string]any, error) {
	if !payloadHasValue(payload, "id") && !payloadHasValue(payload, "documentId") {
		return nil, fmt.Errorf("one of --id or --document-id is required")
	}
	return payload, nil
}

func transformDocumentsRestore(cmd *cobra.Command, spec methodSpec, payload map[string]any) (map[string]any, error) {
	if useLatest, ok := payload["useLatestRevision"].(bool); ok && useLatest {
		if _, ok := payload["revisionId"]; ok {
			return nil, fmt.Errorf("use either --revision-id or --use-latest-revision, not both")
		}
		revisionID, err := latestRevisionID(cmd, fmt.Sprint(payload["id"]))
		if err != nil {
			return nil, err
		}
		payload["revisionId"] = revisionID
	}
	delete(payload, "useLatestRevision")
	return payload, nil
}

func transformDocumentsAddUser(cmd *cobra.Command, spec methodSpec, payload map[string]any) (map[string]any, error) {
	selfID, err := currentUserID(cmd)
	if err != nil {
		return nil, err
	}
	if selfID != "" && strings.TrimSpace(fmt.Sprint(payload["userId"])) == selfID {
		return nil, fmt.Errorf("cannot add yourself as a document user")
	}
	return payload, nil
}

func latestRevisionID(cmd *cobra.Command, documentID string) (string, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return "", err
	}
	response, err := runContext.Client.Post(cmd.Context(), "revisions.list", map[string]any{"documentId": documentID, "limit": 1})
	if err != nil {
		return "", err
	}
	data, ok := outline.ResponseData(response).([]any)
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("latest revision not found for document %s", documentID)
	}
	revision, ok := data[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("latest revision response has unexpected shape")
	}
	revisionID := strings.TrimSpace(fmt.Sprint(revision["id"]))
	if revisionID == "" || revisionID == "<nil>" {
		return "", fmt.Errorf("latest revision id missing from response")
	}
	return revisionID, nil
}

func currentUserID(cmd *cobra.Command) (string, error) {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return "", err
	}
	response, err := runContext.Client.Post(cmd.Context(), "auth.info", nil)
	if err != nil {
		return "", err
	}
	data, ok := outline.ResponseData(response).(map[string]any)
	if !ok {
		return "", fmt.Errorf("auth.info response data missing")
	}
	user, ok := data["user"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("auth.info user missing")
	}
	userID := strings.TrimSpace(fmt.Sprint(user["id"]))
	if userID == "" || userID == "<nil>" {
		return "", fmt.Errorf("auth.info user id missing")
	}
	return userID, nil
}

func buildPayload(cmd *cobra.Command, spec methodSpec, values methodValues, args []string) (map[string]any, error) {
	payload := map[string]any{}
	for index, arg := range spec.Args {
		payload[arg.PayloadName] = args[index]
	}
	for _, field := range spec.Flags {
		changedName, err := changedFieldName(cmd, field, values)
		if err != nil {
			return nil, err
		}
		if changedName == "" {
			continue
		}
		switch field.Type {
		case fieldString:
			payload[field.PayloadName] = strings.TrimSpace(*values.strings[changedName])
		case fieldBool:
			payload[field.PayloadName] = *values.bools[changedName]
		case fieldInt:
			payload[field.PayloadName] = *values.ints[changedName]
		case fieldStringList:
			payload[field.PayloadName] = *values.stringLists[changedName]
		case fieldJSON:
			parsed, err := parseJSONFlag(*values.strings[changedName], changedName)
			if err != nil {
				return nil, err
			}
			payload[field.PayloadName] = parsed
		default:
			panic(fmt.Sprintf("unsupported field type %q", field.Type))
		}
	}
	for _, required := range spec.Required {
		if !payloadHasValue(payload, flagPayloadName(spec, required)) {
			return nil, fmt.Errorf("--%s is required", required)
		}
	}
	delete(payload, "out")
	delete(payload, "accept")
	delete(payload, "file")
	if spec.Multipart.Enabled {
		delete(payload, "contentType")
	}
	return payload, nil
}

func changedFieldName(cmd *cobra.Command, field fieldSpec, values methodValues) (string, error) {
	changed := []string{}
	if cmd.Flags().Changed(field.Name) {
		changed = append(changed, field.Name)
	}
	for _, alias := range field.Aliases {
		if cmd.Flags().Changed(alias) {
			changed = append(changed, alias)
		}
	}
	if len(changed) == 0 {
		return "", nil
	}
	if len(changed) == 1 {
		return changed[0], nil
	}

	firstValue := fieldValueString(field.Type, values, changed[0])
	for _, name := range changed[1:] {
		if fieldValueString(field.Type, values, name) != firstValue {
			return "", fmt.Errorf("conflicting values for --%s and --%s", changed[0], name)
		}
	}
	return changed[0], nil
}

func fieldValueString(fieldType fieldType, values methodValues, name string) string {
	switch fieldType {
	case fieldString, fieldJSON:
		return strings.TrimSpace(*values.strings[name])
	case fieldBool:
		return strconv.FormatBool(*values.bools[name])
	case fieldInt:
		return strconv.Itoa(*values.ints[name])
	case fieldStringList:
		return strings.Join(*values.stringLists[name], "\x00")
	default:
		panic(fmt.Sprintf("unsupported field type %q", fieldType))
	}
}

func flagPayloadName(spec methodSpec, flagName string) string {
	for _, field := range spec.Flags {
		if field.Name == flagName {
			return field.PayloadName
		}
	}
	return flagName
}

func payloadHasValue(payload map[string]any, name string) bool {
	value, ok := payload[name]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case nil:
		return false
	default:
		return true
	}
}

func parseJSONFlag(value string, name string) (any, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("--%s is required", name)
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("parse --%s: %w", name, err)
	}
	return parsed, nil
}

func confirmAction(cmd *cobra.Command, yes bool, method string) error {
	if yes {
		return nil
	}
	if !isTerminal(os.Stdin) {
		return fmt.Errorf("%s is destructive or security-sensitive; rerun with --yes to confirm", method)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s is destructive or security-sensitive. Type 'yes' to continue: ", method)
	var answer string
	_, err := fmt.Fscanln(os.Stdin, &answer)
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if answer != "yes" {
		return fmt.Errorf("confirmation rejected")
	}
	return nil
}

func isTerminal(file *os.File) bool {
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func runMultipartMethod(cmd *cobra.Command, spec methodSpec, payload map[string]any, values methodValues) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	for key, value := range payload {
		fields[key] = fmt.Sprint(value)
	}
	contentType := ""
	if spec.Multipart.ContentFlag != "" && values.strings[spec.Multipart.ContentFlag] != nil {
		contentType = *values.strings[spec.Multipart.ContentFlag]
	}
	response, err := runContext.Client.PostMultipart(cmd.Context(), spec.Method, fields, outline.FilePart{
		FieldName:   spec.Multipart.FileField,
		Path:        *values.strings[spec.Multipart.FileFlag],
		ContentType: contentType,
	})
	if err != nil {
		return err
	}
	return printResponse(cmd, outline.ResponseData(response))
}

func runBinaryMethod(cmd *cobra.Command, spec methodSpec, payload map[string]any, values methodValues) error {
	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	accept := spec.Binary.Accept
	if value := values.strings["accept"]; value != nil && strings.TrimSpace(*value) != "" {
		accept = strings.TrimSpace(*value)
	}
	response, err := runContext.Client.PostBinary(cmd.Context(), spec.Method, payload, accept)
	if err != nil {
		return err
	}
	outPath := ""
	if value := values.strings["out"]; value != nil {
		outPath = strings.TrimSpace(*value)
	}
	if outPath != "" {
		return os.WriteFile(outPath, response.Body, 0644)
	}
	_, err = cmd.OutOrStdout().Write(response.Body)
	return err
}

func runAttachmentUpload(cmd *cobra.Command, payload map[string]any, values methodValues) error {
	filePath := strings.TrimSpace(*values.strings["file"])
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat attachment file: %w", err)
	}
	if _, ok := payload["name"]; !ok {
		payload["name"] = filepath.Base(filePath)
	}
	if _, ok := payload["size"]; !ok {
		payload["size"] = info.Size()
	}
	if _, ok := payload["contentType"]; !ok {
		payload["contentType"] = attachmentContentType(filePath)
	}

	runContext, err := RunContextFrom(cmd)
	if err != nil {
		return err
	}
	response, err := runContext.Client.Post(cmd.Context(), "attachments.create", payload)
	if err != nil {
		return err
	}
	if err := uploadAttachmentFile(response, filePath); err != nil {
		return err
	}
	return printResponse(cmd, outline.ResponseData(response))
}

func attachmentContentType(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "application/octet-stream"
	}
	return http.DetectContentType(buffer[:n])
}

func uploadAttachmentFile(response map[string]any, filePath string) error {
	data, ok := outline.ResponseData(response).(map[string]any)
	if !ok {
		return fmt.Errorf("attachment upload instructions missing from response")
	}
	uploadURL, ok := data["uploadUrl"].(string)
	if !ok || strings.TrimSpace(uploadURL) == "" {
		return fmt.Errorf("attachment upload URL missing from response")
	}
	form, ok := data["form"].(map[string]any)
	if !ok {
		return fmt.Errorf("attachment upload form missing from response")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range form {
		if err := writer.WriteField(key, fmt.Sprint(value)); err != nil {
			return fmt.Errorf("write upload form field: %w", err)
		}
	}
	if err := writeUploadFile(writer, filePath); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close upload form: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, uploadURL, &body)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload attachment: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload attachment failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func writeUploadFile(writer *multipart.Writer, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("create upload file field: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy upload file: %w", err)
	}
	return nil
}

func intString(value int) string {
	return strconv.Itoa(value)
}
