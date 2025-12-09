package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raycarroll/vk-flightctl-provider/pkg/provider"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	log.Println("Starting VK-Flightctl Provider...")

	// Load configuration from environment
	cfg := provider.Config{
		NodeName:              getEnvOrDefault("NODE_NAME", "vk-flightctl-node"),
		FlightctlAPIURL:       getEnvOrDefault("FLIGHTCTL_API_URL", "https://api.flightctl.apps.ocp-rh-aio1.waltoninstitute.ie/api/v1/"),
		FlightctlClientID:     os.Getenv("FLIGHTCTL_CLIENT_ID"),
		FlightctlClientSecret: os.Getenv("FLIGHTCTL_CLIENT_SECRET"),
		FlightctlTokenURL:     getEnvOrDefault("FLIGHTCTL_TOKEN_URL", "https://auth.flightctl.apps.ocp-rh-aio1.waltoninstitute.ie/realms/flightctl/protocol/openid-connect/token"),
		FlightctlInsecureTLS:  getEnvOrDefault("FLIGHTCTL_INSECURE_TLS", "false") == "true",
	}

	// Validate required config
	if cfg.FlightctlClientID == "" {
		log.Fatal("FLIGHTCTL_CLIENT_ID environment variable is required")
	}
	if cfg.FlightctlClientSecret == "" {
		log.Fatal("FLIGHTCTL_CLIENT_SECRET environment variable is required")
	}
	if cfg.FlightctlTokenURL == "" {
		log.Fatal("FLIGHTCTL_TOKEN_URL environment variable is required")
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

	// Create Kubernetes client (in-cluster config)
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Get initial node definition for logging
	nodeSpec, err := p.GetNode(ctx)
	if err != nil {
		log.Fatalf("Failed to get node spec: %v", err)
	}

	// Serialize node to JSON for logging
	nodeJSON, err := json.MarshalIndent(nodeSpec, "", "  ")
	if err != nil {
		log.Printf("Warning: Failed to serialize node to JSON: %v", err)
	} else {
		log.Printf("Node definition:\n%s", string(nodeJSON))
	}

	// Create node using Virtual Kubelet's nodeutil
	nodeRunner, err := nodeutil.NewNode(
		cfg.NodeName,
		func(providerCfg nodeutil.ProviderConfig) (nodeutil.Provider, node.NodeProvider, error) {
			// The provider can be updated with the node from providerCfg if needed
			// For now, just return the provider which implements both interfaces
			return p, p, nil
		},
		nodeutil.WithClient(k8sClient),
		func(nodeCfg *nodeutil.NodeConfig) error {
			// Configure the node with our custom node spec
			nodeCfg.NodeSpec = *nodeSpec
			nodeCfg.NumWorkers = 10
			nodeCfg.InformerResyncPeriod = 30 * time.Second
			return nil
		},
	)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	log.Printf("Virtual node '%s' controller created with capacity: CPU=%s, Memory=%s",
		nodeSpec.Name,
		nodeSpec.Status.Capacity.Cpu().String(),
		nodeSpec.Status.Capacity.Memory().String())

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Run the node controller in a goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Println("Starting Virtual Kubelet node controller...")
		if err := nodeRunner.Run(ctx); err != nil {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigCh:
		log.Println("Received shutdown signal, shutting down gracefully...")
	case err := <-errCh:
		log.Printf("Node controller error: %v", err)
	}

	// Cancel context to stop the node controller
	cancel()

	// Give it a moment to cleanup
	time.Sleep(2 * time.Second)

	log.Println("Shutdown complete")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
