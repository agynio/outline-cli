package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agynio/outline-cli/internal/outline"
	"github.com/agynio/outline-cli/internal/output"
	"github.com/spf13/cobra"
)

func TestBuildPayloadParsesRepresentativeEndpoint(t *testing.T) {
	spec := methodSpec{
		Group:    "documents",
		Action:   "search",
		Method:   "documents.search",
		Flags:    searchFields(),
		Required: []string{"query"},
	}
	cmd := newMethodCommand(spec)
	cmd.SetArgs([]string{"--query", "handbook", "--status-filter", "published", "--limit", "25"})
	if err := cmd.ParseFlags(cmd.Flags().Args()); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
}

func TestViewsCreateUsesDocumentIDFlag(t *testing.T) {
	cmd := findCommand(t, newAPIRootCommands(), "views", "create")
	if cmd.Flags().Lookup("document-id") == nil {
		t.Fatal("views create missing --document-id flag")
	}
	if cmd.Flags().Lookup("document") == nil {
		t.Fatal("views create missing backward-compatible --document alias")
	}
}

func TestCommonIDFlagAliases(t *testing.T) {
	tests := []struct {
		group   string
		command string
		flags   []string
	}{
		{group: "documents", command: "create", flags: []string{"collection", "collection-id"}},
		{group: "comments", command: "list", flags: []string{"document", "document-id"}},
		{group: "collections", command: "add-user", flags: []string{"user", "user-id"}},
		{group: "collections", command: "add-group", flags: []string{"group", "group-id"}},
		{group: "views", command: "create", flags: []string{"document-id", "document"}},
	}

	commands := newAPIRootCommands()
	for _, test := range tests {
		t.Run(test.group+" "+test.command, func(t *testing.T) {
			cmd := findCommand(t, commands, test.group, test.command)
			for _, flag := range test.flags {
				if cmd.Flags().Lookup(flag) == nil {
					t.Fatalf("missing --%s", flag)
				}
			}
		})
	}
}

func TestAliasPayloadUsesCanonicalName(t *testing.T) {
	spec := methodSpec{Flags: fields(s("collection", "collectionId", "Collection ID")), Required: []string{"collection"}}
	cmd := &cobra.Command{}
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	registerFieldFlag(cmd, values, spec.Flags[0])
	_ = cmd.Flags().Set("collection-id", "collection-123")

	payload, err := buildPayload(cmd, spec, values, nil)
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}
	if payload["collectionId"] != "collection-123" {
		t.Fatalf("collectionId = %v, want collection-123", payload["collectionId"])
	}
}

func TestDocumentsCreateCollectionIDPayloadIsVerbatim(t *testing.T) {
	const collectionID = "29532493-abcb-4d35-9477-8537f7335fc7"
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/documents.create" {
			t.Fatalf("request path = %q, want /documents.create", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"doc-1"}}`))
	}))
	defer server.Close()

	cmd := newDocumentsCreateCmd()
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--collection-id", collectionID, "--title", "Smoke", "--text", "# Smoke"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if payload["collectionId"] != collectionID {
		t.Fatalf("collectionId = %v, want %s", payload["collectionId"], collectionID)
	}
}

func TestRootDocumentsCreateCollectionIDPayloadIsVerbatim(t *testing.T) {
	const collectionID = "29532493-abcb-4d35-9477-8537f7335fc7"
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/documents.create" {
			t.Fatalf("request path = %q, want /api/documents.create", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"collectionId: Invalid uuid"}`))
	}))
	defer server.Close()

	home := t.TempDir()
	configDir := filepath.Join(home, ".outline-cli")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "token"), []byte("token\n"), 0600); err != nil {
		t.Fatalf("WriteFile() token error = %v", err)
	}
	t.Setenv("HOME", home)

	root := newRootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stdout)
	root.SetArgs([]string{
		"documents", "create",
		"--collection-id", collectionID,
		"--title", "Smoke",
		"--text", "# Smoke",
		"--publish",
		"--base-url", server.URL,
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() expected server validation error")
	}
	if payload["collectionId"] != collectionID {
		t.Fatalf("collectionId = %v, want %s", payload["collectionId"], collectionID)
	}
	if strings.Contains(err.Error(), "collectionId=d"+collectionID) {
		t.Fatalf("error context includes corrupted collectionId: %v", err)
	}
	if !strings.Contains(err.Error(), "collectionId="+collectionID) {
		t.Fatalf("error context = %q, want verbatim collectionId", err.Error())
	}
}

func TestDocumentsAddUserRejectsSelfInvite(t *testing.T) {
	const selfID = "user-self"
	requests := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, strings.TrimPrefix(r.URL.Path, "/api/"))
		if r.URL.Path == "/api/auth.info" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"user":{"id":"user-self"}}}`))
			return
		}
		t.Fatalf("unexpected request path %s", r.URL.Path)
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "documents",
		Action:    "add-user",
		Method:    "documents.add_user",
		Flags:     fields(s("id", "id", "Document ID"), s("user", "userId", "User ID"), s("permission", "permission", "Permission")),
		Required:  []string{"id", "user"},
		Transform: transformDocumentsAddUser,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL+"/api", "token"),
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--id", "doc-1", "--user-id", selfID})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected self-invite error")
	}
	if !strings.Contains(err.Error(), "cannot add yourself") {
		t.Fatalf("error = %q, want self-invite message", err.Error())
	}
	if len(requests) != 1 || requests[0] != "auth.info" {
		t.Fatalf("requests = %v, want only auth.info", requests)
	}
}

func TestFileOperationsDownloadWritesBinaryResponse(t *testing.T) {
	const fileOperationID = "file-operation-1"
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fileOperations.redirect" {
			t.Fatalf("request path = %q, want /fileOperations.redirect", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte("export-data"))
	}))
	defer server.Close()

	outPath := filepath.Join(t.TempDir(), "export.zip")
	cmd := newMethodCommand(methodSpec{
		Group:    "file-operations",
		Action:   "download",
		Method:   "fileOperations.redirect",
		Flags:    fields(s("id", "id", "File operation ID"), s("out", "out", "Output file")),
		Required: []string{"id", "out"},
		Binary:   binarySpec{Enabled: true, Accept: "application/octet-stream"},
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--id", fileOperationID, "--out", outPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if payload["id"] != fileOperationID {
		t.Fatalf("id = %v, want %s", payload["id"], fileOperationID)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "export-data" {
		t.Fatalf("downloaded data = %q, want export-data", string(data))
	}
}

func TestAliasPayloadRejectsConflictingValues(t *testing.T) {
	spec := methodSpec{Flags: fields(s("document", "documentId", "Document ID")), Required: []string{"document"}}
	cmd := &cobra.Command{}
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	registerFieldFlag(cmd, values, spec.Flags[0])
	_ = cmd.Flags().Set("document", "document-1")
	_ = cmd.Flags().Set("document-id", "document-2")

	_, err := buildPayload(cmd, spec, values, nil)
	if err == nil {
		t.Fatal("buildPayload() expected conflict error")
	}
	if !strings.Contains(err.Error(), "conflicting values") {
		t.Fatalf("buildPayload() error = %q, want conflict", err.Error())
	}
}

func TestAliasPayloadAllowsMatchingValues(t *testing.T) {
	spec := methodSpec{Flags: fields(s("document", "documentId", "Document ID")), Required: []string{"document"}}
	cmd := &cobra.Command{}
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	registerFieldFlag(cmd, values, spec.Flags[0])
	_ = cmd.Flags().Set("document", "document-1")
	_ = cmd.Flags().Set("document-id", "document-1")

	payload, err := buildPayload(cmd, spec, values, nil)
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}
	if payload["documentId"] != "document-1" {
		t.Fatalf("documentId = %v, want document-1", payload["documentId"])
	}
}

func TestCommentsUpdateTextBuildsDataPayload(t *testing.T) {
	payload := buildMethodPayloadForTest(t, methodSpec{
		Flags:     fields(s("id", "id", "Comment ID"), s("text", "text", "Markdown text"), j("data-json", "data", "Comment body JSON")),
		Required:  []string{"id"},
		Transform: transformCommentUpdate,
	}, []string{"--id", "comment-1", "--text", "updated"})

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map", payload["data"])
	}
	if data["text"] != "updated" {
		t.Fatalf("data.text = %v, want updated", data["text"])
	}
	if _, ok := payload["text"]; ok {
		t.Fatal("payload should not include top-level text")
	}
}

func TestStarsUpdateIndexPayloadIsString(t *testing.T) {
	payload := buildMethodPayloadForTest(t, methodSpec{
		Flags:    fields(s("id", "id", "Star ID"), s("index", "index", "Index")),
		Required: []string{"id", "index"},
	}, []string{"--id", "star-1", "--index", "2"})

	if payload["index"] != "2" {
		t.Fatalf("index = %v (%T), want string 2", payload["index"], payload["index"])
	}
}

func TestDocumentsRestoreRevisionIDAlias(t *testing.T) {
	payload := buildMethodPayloadForTest(t, methodSpec{
		Flags:    fields(s("revision", "revisionId", "Revision ID")),
		Required: []string{"revision"},
	}, []string{"--revision-id", "revision-1"})

	if payload["revisionId"] != "revision-1" {
		t.Fatalf("revisionId = %v, want revision-1", payload["revisionId"])
	}
}

func buildMethodPayloadForTest(t *testing.T, spec methodSpec, args []string) map[string]any {
	t.Helper()
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	cmd := &cobra.Command{}
	for _, field := range spec.Flags {
		registerFieldFlag(cmd, values, field)
	}
	cmd.SetArgs(args)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags() error = %v", err)
	}
	payload, err := buildPayload(cmd, spec, values, nil)
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}
	if spec.Transform != nil {
		payload, err = spec.Transform(cmd, spec, payload)
		if err != nil {
			t.Fatalf("Transform() error = %v", err)
		}
	}
	return payload
}

func TestHandwrittenAliasesRejectConflictingValues(t *testing.T) {
	tests := []struct {
		name  string
		cmd   *cobra.Command
		flags []string
	}{
		{name: "documents list", cmd: newDocumentsListCmd(), flags: []string{"collection", "collection-id"}},
		{name: "documents create", cmd: newDocumentsCreateCmd(), flags: []string{"collection", "collection-id"}},
		{name: "documents update", cmd: newDocumentsUpdateCmd(), flags: []string{"collection", "collection-id"}},
		{name: "comments list", cmd: newCommentsListCmd(), flags: []string{"document", "document-id"}},
		{name: "comments create", cmd: newCommentsCreateCmd(), flags: []string{"document", "document-id"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_ = test.cmd.Flags().Set(test.flags[0], "value-1")
			_ = test.cmd.Flags().Set(test.flags[1], "value-2")
			_, err := aliasedStringFlagValue(test.cmd, test.flags[0], test.flags[1])
			if err == nil {
				t.Fatal("aliasedStringFlagValue() expected conflict error")
			}
		})
	}
}

func TestSmokeRunnerCommandFlagsExist(t *testing.T) {
	tests := []struct {
		group   string
		command string
		flags   []string
	}{
		{group: "documents", command: "create", flags: []string{"collection-id"}},
		{group: "documents", command: "list", flags: []string{"collection-id"}},
		{group: "documents", command: "search", flags: []string{"collection-id"}},
		{group: "comments", command: "list", flags: []string{"document-id"}},
		{group: "views", command: "list", flags: []string{"document-id"}},
		{group: "views", command: "create", flags: []string{"document-id"}},
		{group: "shares", command: "list", flags: []string{"document-id"}},
		{group: "events", command: "list", flags: []string{"document-id"}},
	}

	commands := newAPIRootCommands()
	for _, test := range tests {
		t.Run(test.group+" "+test.command, func(t *testing.T) {
			cmd := findCommand(t, commands, test.group, test.command)
			for _, flag := range test.flags {
				if cmd.Flags().Lookup(flag) == nil {
					t.Fatalf("missing --%s", flag)
				}
			}
		})
	}
}

func TestIDCommandsAcceptFlagAndOptionalArg(t *testing.T) {
	tests := []struct {
		name    string
		command *cobra.Command
		use     string
	}{
		{name: "collections info", command: newCollectionsInfoCmd(), use: "info"},
		{name: "collections tree", command: newCollectionsTreeCmd(), use: "tree [collection-id]"},
		{name: "documents info", command: newDocumentsInfoCmd(), use: "info"},
		{name: "documents pull", command: newDocumentsPullCmd(), use: "pull [id-or-urlId]"},
		{name: "documents update", command: newDocumentsUpdateCmd(), use: "update [id]"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.command.Flags().Lookup("id") == nil {
				t.Fatal("missing --id flag")
			}
			if test.command.Use != test.use {
				t.Fatalf("Use = %q, want %q", test.command.Use, test.use)
			}
		})
	}
}

func TestIDFromFlagOrArg(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		args      []string
		want      string
		wantErr   bool
	}{
		{name: "flag", flagValue: "doc-1", want: "doc-1"},
		{name: "arg", args: []string{"doc-2"}, want: "doc-2"},
		{name: "matching both", flagValue: "doc-3", args: []string{"doc-3"}, want: "doc-3"},
		{name: "missing", wantErr: true},
		{name: "conflict", flagValue: "doc-4", args: []string{"doc-5"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := idFromFlagOrArg(test.flagValue, test.args, "document ID")
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("idFromFlagOrArg() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("idFromFlagOrArg() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildPayloadParsesJSONFlag(t *testing.T) {
	spec := methodSpec{Flags: fields(j("data-json", "data", "Data JSON")), Required: []string{"data-json"}}
	cmd := &cobra.Command{}
	values := methodValues{strings: map[string]*string{}, bools: map[string]*bool{}, ints: map[string]*int{}, stringLists: map[string]*[]string{}}
	registerFieldFlag(cmd, values, spec.Flags[0])
	_ = cmd.Flags().Set("data-json", `{"type":"doc"}`)

	payload, err := buildPayload(cmd, spec, values, nil)
	if err != nil {
		t.Fatalf("buildPayload() error = %v", err)
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data payload type = %T, want map", payload["data"])
	}
	if data["type"] != "doc" {
		t.Fatalf("data.type = %v, want doc", data["type"])
	}
}

func TestConfirmActionRequiresYesForNonTTY(t *testing.T) {
	originalStdin := os.Stdin
	readFile, writeFile, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	_ = writeFile.Close()
	os.Stdin = readFile
	t.Cleanup(func() {
		os.Stdin = originalStdin
		_ = readFile.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(withRunContext(context.Background(), &RunContext{OutputFormat: output.FormatYAML}))
	err = confirmAction(cmd, false, "documents.delete")
	if err == nil {
		t.Fatal("confirmAction() expected error")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("confirmAction() error = %q, want --yes hint", err.Error())
	}
	if err := confirmAction(cmd, true, "documents.delete"); err != nil {
		t.Fatalf("confirmAction() with yes error = %v", err)
	}
}

func TestAllOpenAPIMethodsHaveCommands(t *testing.T) {
	officialMethods := []string{
		"accessRequests.approve", "accessRequests.create", "accessRequests.dismiss", "accessRequests.info",
		"attachments.create", "attachments.delete", "attachments.redirect",
		"auth.config", "auth.info",
		"collections.add_group", "collections.add_user", "collections.create", "collections.delete", "collections.documents", "collections.export", "collections.export_all", "collections.group_memberships", "collections.info", "collections.list", "collections.memberships", "collections.remove_group", "collections.remove_user", "collections.update",
		"comments.create", "comments.delete", "comments.info", "comments.list", "comments.update",
		"dataAttributes.create", "dataAttributes.delete", "dataAttributes.info", "dataAttributes.list", "dataAttributes.update",
		"documents.add_group", "documents.add_user", "documents.answerQuestion", "documents.archive", "documents.archived", "documents.create", "documents.delete", "documents.deleted", "documents.documents", "documents.drafts", "documents.duplicate", "documents.empty_trash", "documents.export", "documents.group_memberships", "documents.import", "documents.info", "documents.insights", "documents.list", "documents.memberships", "documents.move", "documents.remove_group", "documents.remove_user", "documents.restore", "documents.search", "documents.search_titles", "documents.templatize", "documents.unpublish", "documents.update", "documents.users", "documents.viewed",
		"events.list",
		"fileOperations.delete", "fileOperations.info", "fileOperations.list", "fileOperations.redirect",
		"groups.add_user", "groups.create", "groups.delete", "groups.info", "groups.list", "groups.memberships", "groups.remove_user", "groups.update",
		"oauthAuthentications.delete", "oauthAuthentications.list",
		"oauthClients.create", "oauthClients.delete", "oauthClients.info", "oauthClients.list", "oauthClients.rotate_secret", "oauthClients.update",
		"revisions.info", "revisions.list",
		"shares.create", "shares.info", "shares.list", "shares.revoke", "shares.update",
		"stars.create", "stars.delete", "stars.list", "stars.update",
		"templates.create", "templates.delete", "templates.duplicate", "templates.info", "templates.list", "templates.restore", "templates.update",
		"users.activate", "users.delete", "users.info", "users.invite", "users.list", "users.suspend", "users.update", "users.update_role",
		"views.create", "views.list",
	}

	want := map[string]bool{}
	for _, method := range officialMethods {
		want[method] = true
	}
	implemented := map[string]bool{}
	for _, spec := range outlineMethods {
		if !want[spec.Method] {
			t.Fatalf("extra command outside official OpenAPI inventory: %s", spec.Method)
		}
		implemented[spec.Method] = true
	}
	for _, method := range officialMethods {
		if !implemented[method] && !existingCommandMethod(method) {
			t.Fatalf("missing command for %s", method)
		}
	}
}

func existingCommandMethod(method string) bool {
	switch method {
	case "auth.config", "auth.info",
		"collections.documents", "collections.info", "collections.list",
		"comments.create", "comments.list",
		"documents.create", "documents.info", "documents.list", "documents.search", "documents.update":
		return true
	default:
		return false
	}
}

func findCommand(t *testing.T, roots []*cobra.Command, groupName string, commandName string) *cobra.Command {
	t.Helper()
	for _, root := range roots {
		if root.Name() != groupName {
			continue
		}
		for _, command := range root.Commands() {
			if command.Name() == commandName {
				return command
			}
		}
	}
	t.Fatalf("command %s %s not found", groupName, commandName)
	return nil
}
