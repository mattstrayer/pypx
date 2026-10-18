// Package tools exposes the pypx plain-text ("agent") HTTP surface as MCP
// tools. Each tool is a thin adapter that GETs the corresponding .txt
// endpoint and returns its body verbatim as MCP text content — the .txt
// formats are the semi-public, golden-file-locked contract, so this layer
// never re-renders them.
package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client fetches plain-text bodies from a pypx API base URL.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient returns a Client targeting baseURL (trailing slash trimmed) with a
// 10s per-request timeout.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// FetchPackageText GETs {base}/api/packages/{name}.txt and returns the body.
// 404 maps to a "package not found" error; any other non-200 includes the
// status code.
func (c *Client) FetchPackageText(ctx context.Context, name string) (string, error) {
	endpoint := c.baseURL + "/api/packages/" + url.PathEscape(name) + ".txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return string(body), nil
	case http.StatusNotFound:
		return "", fmt.Errorf("package not found: %s", name)
	default:
		return "", fmt.Errorf("pypx API returned status %d for package %q", resp.StatusCode, name)
	}
}

// getPackageArgs is the input schema for the get_package tool. The jsonschema
// tag becomes the property description in the generated MCP tool schema.
type getPackageArgs struct {
	Name string `json:"name" jsonschema:"the PyPI package name (PEP 503 normalized, case-insensitive)"`
}

// Register wires the pypx tools onto server. Only get_package is implemented in
// this spike; the remaining ten tools are deferred (see the design doc).
func Register(server *mcp.Server, c *Client) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_package",
		Description: "Package metadata (key:value plus dependency list) for a PyPI package.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getPackageArgs) (*mcp.CallToolResult, any, error) {
		body, err := c.FetchPackageText(ctx, args.Name)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: body}},
		}, nil, nil
	})
}
