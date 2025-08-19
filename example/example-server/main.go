package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	fmt.Print("Running...")

	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-time.After(60 * time.Second):
		fmt.Println("Timeout reached")
		break
	case sig := <-sigChan:
		fmt.Printf("Received signal: %v, shutting down gracefully...\n", sig)
		break
	}
}
