package main

import (
	"log"
	"os"

	"github.com/zafrem/vanish/mcp/server"
)

func main() {
	// Create Vanish MCP server
	srv, err := server.NewServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Run server (stdio transport)
	if err := srv.Run(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
