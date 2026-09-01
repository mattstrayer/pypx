// Command pypx-mcp is an MCP server exposing the pypx plain-text agent surface
// as MCP tools. The spike registers a single tool (get_package) and serves
// over stdio; the transport is chosen here so a hosted streamable-HTTP variant
// can be swapped in later without touching the tool code.
package main

import (
	"context"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pypx/mcp/internal/tools"
)

const version = "0.1.0"

func main() {
	baseURL := os.Getenv("PYPX_API_BASE")
	if baseURL == "" {
		baseURL = "https://pypx.app"
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "pypx", Version: version}, nil)
	tools.Register(server, tools.NewClient(baseURL))

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("pypx-mcp: %v", err)
	}
}
