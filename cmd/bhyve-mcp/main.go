package main

import (
	"log"

	"github.com/mlapointe/bhyve-mcp/internal/mcp"
)

func main() {
	log.Println("bhyve-mcp starting...")

	server := mcp.NewServer()
	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
