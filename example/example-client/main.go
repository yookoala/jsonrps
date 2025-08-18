package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yookoala/jsonrps"
)

func main() {
	const socketPath = "example.sock"
	lh := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(lh)

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to receive shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Connect to the server socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		logger.Error("Error connecting to server", "error", err)
		os.Exit(1)
	}

	logger.Info("Connected to server socket", "socket", socketPath)

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
		sess, err := jsonrps.InitializeClientSession(conn, jsonrps.DefaultClientHeader(), logger)
		if err != nil {
			logger.Error("Error initializing client", "error", err)
			return
		}

		// Clear deadline after successful initialization
		conn.SetDeadline(time.Time{})

		sess.Logger.Info("JSON-RPC session initialized successfully")

		// Monitor connection status and context cancellation
		go func() {
			// Keep reading from connection to detect when it's closed
			buffer := make([]byte, 1024)
			for {
				// Set a short read timeout to periodically check context
				conn.SetReadDeadline(time.Now().Add(1 * time.Second))

				_, err := conn.Read(buffer)
				if err != nil {
					// Check if it's a timeout (expected) or actual error
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						// Timeout is expected, check if context is done
						select {
						case <-ctx.Done():
							return
						default:
							continue
						}
					}
					// Real error (connection closed, etc.)
					logger.Error("Connection error detected", "error", err)
					cancel() // Signal shutdown
					return
				}
				// If we actually read data, we could process it here
				// For now, we just continue monitoring
			}
		}()

		// Wait for context to be cancelled (either by signal or connection error)
		<-ctx.Done()
		logger.Info("Closing connection...")
	}()

	// Wait for either connection to close or shutdown signal
	select {
	case <-connClosed:
		logger.Info("Connection closed")
	case sig := <-sigChan:
		logger.Info("Received signal, shutting down gracefully...", "signal", sig)

		// Cancel context to stop the connection handler
		cancel()

		// Force close the connection to unblock any reads
		closeConn()

		// Give a brief moment for cleanup
		select {
		case <-connClosed:
			logger.Info("Connection closed gracefully")
		case <-time.After(1 * time.Second):
			logger.Info("Forcing immediate shutdown")
		}
	}

	logger.Info("Client shutdown complete")
}
