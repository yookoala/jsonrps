package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yookoala/jsonrps"
)

func main() {
	const socketPath = "example.sock"

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to receive shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Connect to the server socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("Error connecting to server: %v", err)
	}

	log.Printf("Connected to server socket: %s", socketPath)

	// Channel to signal connection is closed
	connClosed := make(chan struct{})

	// Goroutine to handle the connection
	go func() {
		defer close(connClosed)
		defer conn.Close()

		// Initialize the JSON-RPC session
		sess, err := jsonrps.InitializeClientConn(conn, jsonrps.DefaultClientHeader())
		if err != nil {
			log.Printf("Error initializing client: %v", err)
			return
		}

		log.Println("JSON-RPC session initialized successfully")
		_ = sess

		// Keep the connection alive until context is cancelled
		<-ctx.Done()
		log.Println("Closing connection...")
	}()

	// Goroutine to handle shutdown signals
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, shutting down gracefully...", sig)

		// Cancel context to stop the connection handler
		cancel()
	}()

	// Wait for either connection to close or shutdown signal
	select {
	case <-connClosed:
		log.Println("Connection closed")
	case <-ctx.Done():
		log.Println("Shutdown initiated")

		// Give some time for graceful cleanup
		select {
		case <-connClosed:
			log.Println("Connection closed gracefully")
		case <-time.After(5 * time.Second):
			log.Println("Timeout waiting for connection to close, forcing shutdown")
		}
	}

	log.Println("Client shutdown complete")
}
