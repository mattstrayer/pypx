package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pypx/mcp/internal/tools"
)

const cannedBody = "package: httpx\nversion: 0.28.1\n"

// newTestServer serves the canned .txt body for httpx and 404 for everything
// else, modeling api/internal/handler/extras_test.go's httptest pattern.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/packages/httpx.txt" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(cannedBody))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchPackageTextHappyPath(t *testing.T) {
	srv := newTestServer(t)
	c := tools.NewClient(srv.URL)

	got, err := c.FetchPackageText(context.Background(), "httpx")
	if err != nil {
		t.Fatalf("FetchPackageText: %v", err)
	}
	if got != cannedBody {
		t.Errorf("body = %q, want %q", got, cannedBody)
	}
}

func TestFetchPackageTextNotFound(t *testing.T) {
	srv := newTestServer(t)
	c := tools.NewClient(srv.URL)

	_, err := c.FetchPackageText(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing package, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error %q should mention the package name", err.Error())
	}
}

// TestGetPackageInMemory drives tools/list + tools/call end-to-end through the
// SDK's in-memory transport pair, proving registration and the call path.
func TestGetPackageInMemory(t *testing.T) {
	srv := newTestServer(t)

	server := mcp.NewServer(&mcp.Implementation{Name: "pypx", Version: "test"}, nil)
	tools.Register(server, tools.NewClient(srv.URL))

	clientT, serverT := mcp.NewInMemoryTransports()

	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	defer serverSession.Close() //nolint:errcheck

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0"}, nil)
	clientSession, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer clientSession.Close() //nolint:errcheck

	// tools/list must return exactly get_package.
	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listed.Tools) != 1 {
		t.Fatalf("tool count = %d, want 1", len(listed.Tools))
	}
	if listed.Tools[0].Name != "get_package" {
		t.Fatalf("tool name = %q, want get_package", listed.Tools[0].Name)
	}

	// tools/call get_package round-trips the canned body.
	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_package",
		Arguments: map[string]any{"name": "httpx"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error result: %+v", res.Content)
	}
	if len(res.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] type = %T, want *mcp.TextContent", res.Content[0])
	}
	if text.Text != cannedBody {
		t.Errorf("text = %q, want %q", text.Text, cannedBody)
	}
}
