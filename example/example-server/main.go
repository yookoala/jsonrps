package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/yookoala/jsonrps"
)

func handleConnection(logger *slog.Logger, conn net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	defer conn.Close()
	sess, err := jsonrps.InitializeServerSession(conn, logger)
	if err != nil {
		logger.Error("Error initializing session", "error", err)
		return
	}
	_ = sess
}

func main() {
	const socketPath = "example.sock"
	logger := slog.Default()

	// Remove socket file if it already exists
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		logger.Warn("failed to remove existing socket file", "error", err)
	}

	// Create a socket for communication
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}

	logger.Info("Server listening on socket", "socket", socketPath)

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait group to track active connections
	var wg sync.WaitGroup

	// Channel to receive shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Goroutine to handle shutdown signals
	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down gracefully...", "signal", sig)

		// Cancel context to stop accepting new connections
		cancel()

		// Close the listener to stop accepting new connections
		if err := listener.Close(); err != nil {
			logger.Info("Error closing listener", "error", err)
		}

		// Wait for active connections to finish (with timeout)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("All connections closed gracefully")
		case <-time.After(10 * time.Second):
			logger.Info("Timeout waiting for connections to close, forcing shutdown")
		}

		// Clean up socket file
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			logger.Info("Error removing socket file", "error", err)
		} else {
			logger.Info("Socket file removed", "socket", socketPath)
		}

		logger.Info("Server shutdown complete")
		os.Exit(0)
	}()

	// Accept connections loop
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop accepting connections
			return
		default:
			// Set a short read timeout on accept to allow checking context
			if tcpListener, ok := listener.(*net.UnixListener); ok {
				tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
			}

			conn, err := listener.Accept()
			if err != nil {
				// Check if this is a timeout error (expected during shutdown)
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				// Check if listener was closed (during shutdown)
				if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
					logger.Info("Listener closed", "error", err)
					return
				}
				logger.Info("Error accepting connection", "error", err)
				continue
			}

			wg.Add(1)
			go handleConnection(logger, conn, &wg)
		}
	}
}
