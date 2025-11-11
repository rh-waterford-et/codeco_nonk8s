package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/raycarroll/vk-flightctl-provider/pkg/provider"
)

func main() {
	log.Println("Starting VK-Flightctl Provider...")

	// Load configuration from environment
	cfg := provider.Config{
		NodeName:             getEnvOrDefault("NODE_NAME", "vk-flightctl-node"),
		FlightctlAPIURL:      getEnvOrDefault("FLIGHTCTL_API_URL", "https://api.flightctl.apps.ocp-rh-aio1.waltoninstitute.ie:3443"),
		FlightctlAuthToken:   os.Getenv("FLIGHTCTL_AUTH_TOKEN"),
		FlightctlInsecureTLS: getEnvOrDefault("FLIGHTCTL_INSECURE_TLS", "false") == "true",
	}

	// Validate required config
	if cfg.FlightctlAuthToken == "" {
		log.Fatal("FLIGHTCTL_AUTH_TOKEN environment variable is required")
	}

	// Create provider
	p, err := provider.NewProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	// Check connectivity
	ctx := context.Background()
	if err := p.Ping(ctx); err != nil {
		log.Printf("Warning: Failed to ping Flightctl API: %v", err)
	} else {
		log.Println("Successfully connected to Flightctl API")
	}

	// Get node info
	node, err := p.GetNode(ctx)
	if err != nil {
		log.Fatalf("Failed to get node: %v", err)
	}
	log.Printf("Virtual node '%s' initialized with capacity: CPU=%s, Memory=%s",
		node.Name,
		node.Status.Capacity.Cpu().String(),
		node.Status.Capacity.Memory().String())

	log.Println("Provider is running. Press Ctrl+C to exit.")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
