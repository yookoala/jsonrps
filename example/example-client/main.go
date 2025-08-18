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
	var connClosedOnce bool

	// Function to safely close connection
	closeConn := func() {
		if !connClosedOnce {
			connClosedOnce = true
			conn.Close()
			close(connClosed)
		}
	}

	// Goroutine to handle the connection
	go func() {
		defer closeConn()

		// Set a reasonable timeout for initialization
		conn.SetDeadline(time.Now().Add(10 * time.Second))

		// Initialize the JSON-RPC session
		sess, err := jsonrps.InitializeClientConn(conn, jsonrps.DefaultClientHeader())
		if err != nil {
			log.Printf("Error initializing client: %v", err)
			return
		}

		// Clear deadline after successful initialization
		conn.SetDeadline(time.Time{})

		log.Println("JSON-RPC session initialized successfully")
		_ = sess

		// Keep the connection alive until context is cancelled
		<-ctx.Done()
		log.Println("Closing connection...")
	}()

	// Wait for either connection to close or shutdown signal
	select {
	case <-connClosed:
		log.Println("Connection closed")
	case sig := <-sigChan:
		log.Printf("Received signal: %v, shutting down gracefully...", sig)

		// Cancel context to stop the connection handler
		cancel()

		// Force close the connection to unblock any reads
		closeConn()

		// Give a brief moment for cleanup
		select {
		case <-connClosed:
			log.Println("Connection closed gracefully")
		case <-time.After(1 * time.Second):
			log.Println("Forcing immediate shutdown")
		}
	}

	log.Println("Client shutdown complete")
}
