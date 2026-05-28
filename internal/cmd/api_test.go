package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

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
	want := map[string]bool{}
	for _, spec := range outlineMethods {
		want[spec.Method] = true
	}
	want["auth.config"] = true
	want["auth.info"] = true
	for _, method := range []string{
		"collections.documents", "collections.info", "collections.list",
		"comments.create", "comments.list",
		"documents.create", "documents.info", "documents.list", "documents.search", "documents.update",
	} {
		want[method] = true
	}
	for _, method := range []string{
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
	} {
		if !want[method] {
			t.Fatalf("missing command for %s", method)
		}
	}
}
