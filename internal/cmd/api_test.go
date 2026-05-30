package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		BaseURL:      server.URL,
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

func TestSharesInfoFallsBackFromIDToDocumentID(t *testing.T) {
	const shareID = "share-1"
	const documentID = "doc-1"
	requests := []map[string]any{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shares.info" {
			t.Fatalf("request path = %q, want /shares.info", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, payload)
		w.Header().Set("Content-Type", "application/json")
		if _, ok := payload["id"]; ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"other","documentId":"doc-1"},{"id":"share-1","documentId":"doc-1","url":"https://wiki.example.com/share-1"}]}`))
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--id", shareID, "--document-id", documentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	if requests[0]["id"] != shareID || requests[0]["documentId"] != documentID {
		t.Fatalf("first request = %#v, want id and documentId", requests[0])
	}
	if _, ok := requests[1]["id"]; ok {
		t.Fatalf("fallback request should not include id: %#v", requests[1])
	}
	if requests[1]["documentId"] != documentID {
		t.Fatalf("fallback documentId = %v, want %s", requests[1]["documentId"], documentID)
	}
	if !strings.Contains(stdout.String(), `"id": "share-1"`) {
		t.Fatalf("stdout = %s, want matching share", stdout.String())
	}
	if strings.Contains(stdout.String(), `"id": "other"`) {
		t.Fatalf("stdout = %s, should filter non-matching share", stdout.String())
	}
}

func TestSharesInfoInfersDocumentIDFromList(t *testing.T) {
	const shareID = "share-1"
	const documentID = "doc-1"
	requests := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, strings.TrimPrefix(r.URL.Path, "/")+":"+fmt.Sprint(payload))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/shares.info", "/api/shares.info":
			if _, ok := payload["id"]; ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":"share-1","documentId":"doc-1"}}`))
		case "/shares.list", "/api/shares.list":
			_, _ = w.Write([]byte(`{"data":[{"id":"share-1","documentId":"doc-1"}]}`))
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--id", shareID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %v, want id lookup, shares.list, document lookup", requests)
	}
	if !strings.Contains(requests[2], "documentId:"+documentID) {
		t.Fatalf("document fallback request = %q, want documentId %s", requests[2], documentID)
	}
	if !strings.Contains(stdout.String(), `"id": "share-1"`) {
		t.Fatalf("stdout = %s, want inferred share", stdout.String())
	}
}

func TestSharesInfoResolvesDocumentIDFromSharePage(t *testing.T) {
	const shareID = "share-1"
	const documentID = "doc-uuid"
	const urlID = "url123"
	requests := []string{}
	sharePageAuth := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/shares.info", "/api/shares.info":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests = append(requests, "shares.info:"+fmt.Sprint(payload))
			if _, ok := payload["id"]; ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
				return
			}
			if payload["documentId"] != documentID {
				t.Fatalf("document fallback payload = %#v, want documentId %s", payload, documentID)
			}
			_, _ = w.Write([]byte(`{"data":{"shares":[{"id":"other","documentId":"doc-uuid"},{"id":"share-1","documentId":"doc-uuid"}]}}`))
		case "/shares.list", "/api/shares.list":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests = append(requests, "shares.list:"+fmt.Sprint(payload))
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/s/share-1":
			sharePageAuth = r.Header.Get("Authorization")
			requests = append(requests, "GET /s/share-1")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><a href="/s/share-1/doc/example-title-url123">doc</a></body></html>`))
		case "/documents.info", "/api/documents.info":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests = append(requests, "documents.info:"+fmt.Sprint(payload))
			if payload["id"] != urlID {
				t.Fatalf("documents.info payload = %#v, want urlId %s", payload, urlID)
			}
			_, _ = w.Write([]byte(`{"data":{"id":"doc-uuid","urlId":"url123"}}`))
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL+"/api", "token"),
		BaseURL:      server.URL + "/api",
		OutputFormat: output.FormatJSON,
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--id", shareID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if sharePageAuth != "" {
		t.Fatalf("share page Authorization header = %q, want empty", sharePageAuth)
	}
	if len(requests) != 5 {
		t.Fatalf("requests = %v, want id lookup, list, page, document info, document share", requests)
	}
	if !strings.Contains(stdout.String(), `"id": "share-1"`) {
		t.Fatalf("stdout = %s, want resolved share", stdout.String())
	}
	if strings.Contains(stdout.String(), `"id": "other"`) {
		t.Fatalf("stdout = %s, should filter non-matching share", stdout.String())
	}
}

func TestSharesInfoSharePageResolutionErrorsClearly(t *testing.T) {
	const shareID = "share-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/shares.info", "/api/shares.info":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
		case "/shares.list", "/api/shares.list":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/s/share-1":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>No document link</body></html>`))
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--id", shareID})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected resolution error")
	}
	if !strings.Contains(err.Error(), "could not be resolved through cache, shares.list, or share page") {
		t.Fatalf("error = %q, want share page resolution message", err.Error())
	}
}

func TestSharesInfoFallbackErrorsWhenIDMissingFromDocumentResponse(t *testing.T) {
	const shareID = "missing-share"
	const documentID = "doc-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		if _, ok := payload["id"]; ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"other","documentId":"doc-1"}]}`))
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--id", shareID, "--document-id", documentID})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() expected missing share error")
	}
	if !strings.Contains(err.Error(), "not found in document share response") {
		t.Fatalf("error = %q, want missing share message", err.Error())
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
		BaseURL:      server.URL,
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

func TestCommentsUpdateRequiresProseMirrorDataJSON(t *testing.T) {
	proseMirror := `{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"updated"}]}]}`
	payload := buildMethodPayloadForTest(t, methodSpec{
		Flags:    fields(s("id", "id", "Comment ID"), j("data-json", "data", "Valid ProseMirror comment document JSON")),
		Required: []string{"id", "data-json"},
	}, []string{"--id", "comment-1", "--data-json", proseMirror})

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want map", payload["data"])
	}
	if data["type"] != "doc" {
		t.Fatalf("data.type = %v, want doc", data["type"])
	}
	content, ok := data["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("data.content = %#v, want one ProseMirror node", data["content"])
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

func TestShareCacheReadWriteIsPerBaseURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := cacheShareDocument("https://one.example.com/api", "share-1", "doc-1"); err != nil {
		t.Fatalf("cacheShareDocument() error = %v", err)
	}
	if err := cacheShareDocument("https://two.example.com/api", "share-1", "doc-2"); err != nil {
		t.Fatalf("cacheShareDocument() second base error = %v", err)
	}

	documentID, ok, err := lookupCachedShareDocument("https://one.example.com/api", "share-1")
	if err != nil {
		t.Fatalf("lookupCachedShareDocument() error = %v", err)
	}
	if !ok || documentID != "doc-1" {
		t.Fatalf("one.example lookup = %q, %v; want doc-1, true", documentID, ok)
	}
	documentID, ok, err = lookupCachedShareDocument("https://two.example.com/api", "share-1")
	if err != nil {
		t.Fatalf("lookupCachedShareDocument() second base error = %v", err)
	}
	if !ok || documentID != "doc-2" {
		t.Fatalf("two.example lookup = %q, %v; want doc-2, true", documentID, ok)
	}

	cachePath := filepath.Join(home, ".outline-cli", "cache.json")
	info, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Stat() cache error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("cache mode = %v, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("ReadFile() cache error = %v", err)
	}
	if strings.Contains(string(data), "token") || strings.Contains(string(data), "api-key") {
		t.Fatalf("cache should not contain secret-looking fields: %s", string(data))
	}
}

func TestSharesInfoFallsBackToCachedDocumentID(t *testing.T) {
	const shareID = "share-1"
	const documentID = "doc-1"
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/shares.info" {
			t.Fatalf("request path = %q, want /shares.info", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := payload["id"]; ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
			return
		}
		if payload["documentId"] != documentID {
			t.Fatalf("document fallback payload = %#v, want documentId %s", payload, documentID)
		}
		_, _ = w.Write([]byte(`{"data":{"id":"share-1","documentId":"doc-1"}}`))
	}))
	defer server.Close()

	if err := cacheShareDocument(server.URL, shareID, documentID); err != nil {
		t.Fatalf("cacheShareDocument() error = %v", err)
	}

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--id", shareID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"id": "share-1"`) {
		t.Fatalf("stdout = %s, want cached share", stdout.String())
	}
}

func TestExpiredShareCacheEntryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cache := newShareCache()
	cache.Bases["https://wiki.example.com/api"] = shareCacheBase{Shares: map[string]shareCacheEntry{
		"share-1": {DocumentID: "doc-1", CreatedAt: time.Now().UTC().Add(-shareCacheTTL - time.Hour).Format(time.RFC3339)},
	}}
	if err := saveShareCache(cache); err != nil {
		t.Fatalf("saveShareCache() error = %v", err)
	}

	documentID, ok, err := lookupCachedShareDocument("https://wiki.example.com/api", "share-1")
	if err != nil {
		t.Fatalf("lookupCachedShareDocument() error = %v", err)
	}
	if ok || documentID != "" {
		t.Fatalf("expired lookup = %q, %v; want empty, false", documentID, ok)
	}
	cache, err = loadShareCache()
	if err != nil {
		t.Fatalf("loadShareCache() error = %v", err)
	}
	if _, ok := cache.Bases["https://wiki.example.com/api"].Shares["share-1"]; ok {
		t.Fatal("expired entry should be removed from cache")
	}
}

func TestSharesInfoContinuesAfterStaleCacheMiss(t *testing.T) {
	const shareID = "share-1"
	const cachedDocumentID = "old-doc"
	const listedDocumentID = "doc-1"
	home := t.TempDir()
	t.Setenv("HOME", home)
	requests := []string{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, strings.TrimPrefix(r.URL.Path, "/")+":"+fmt.Sprint(payload))
		switch r.URL.Path {
		case "/shares.info":
			if _, ok := payload["id"]; ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"Resource not found"}`))
				return
			}
			switch payload["documentId"] {
			case cachedDocumentID:
				_, _ = w.Write([]byte(`{"data":{"id":"other","documentId":"old-doc"}}`))
			case listedDocumentID:
				_, _ = w.Write([]byte(`{"data":{"id":"share-1","documentId":"doc-1"}}`))
			default:
				t.Fatalf("unexpected documentId payload %#v", payload)
			}
		case "/shares.list":
			_, _ = w.Write([]byte(`{"data":[{"id":"share-1","documentId":"doc-1"}]}`))
		default:
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	if err := cacheShareDocument(server.URL, shareID, cachedDocumentID); err != nil {
		t.Fatalf("cacheShareDocument() error = %v", err)
	}

	cmd := newMethodCommand(methodSpec{
		Group:     "shares",
		Action:    "info",
		Method:    "shares.info",
		Flags:     fields(s("id", "id", "Share ID"), s("document", "documentId", "Document ID")),
		Transform: transformSharesInfo,
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"--id", shareID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"id": "share-1"`) {
		t.Fatalf("stdout = %s, want listed share", stdout.String())
	}
	if len(requests) != 4 {
		t.Fatalf("requests = %v, want id lookup, cached doc lookup, list, listed doc lookup", requests)
	}
	if !strings.Contains(requests[1], "documentId:old-doc") || !strings.Contains(requests[3], "documentId:doc-1") {
		t.Fatalf("fallback requests = %v, want stale cache then shares.list doc", requests)
	}
}

func TestSharesCreateUpdatesShareCache(t *testing.T) {
	const shareID = "share-1"
	const documentID = "doc-1"
	home := t.TempDir()
	t.Setenv("HOME", home)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shares.create" {
			t.Fatalf("request path = %q, want /shares.create", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"share-1","documentId":"doc-1"}}`))
	}))
	defer server.Close()

	cmd := newMethodCommand(methodSpec{
		Group:    "shares",
		Action:   "create",
		Method:   "shares.create",
		Flags:    fields(s("document", "documentId", "Document ID")),
		Required: []string{"document"},
	})
	cmd.SetContext(withRunContext(context.Background(), &RunContext{
		Client:       outline.NewClient(server.URL, "token"),
		BaseURL:      server.URL,
		OutputFormat: output.FormatJSON,
	}))
	cmd.SetArgs([]string{"--document-id", documentID})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	cachedDocumentID, ok, err := lookupCachedShareDocument(server.URL, shareID)
	if err != nil {
		t.Fatalf("lookupCachedShareDocument() error = %v", err)
	}
	if !ok || cachedDocumentID != documentID {
		t.Fatalf("cached document = %q, %v; want %s, true", cachedDocumentID, ok, documentID)
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
