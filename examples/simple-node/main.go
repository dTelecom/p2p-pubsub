package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dtelecom/p2p-pubsub/common"
	"github.com/dtelecom/p2p-pubsub/pubsub"
	"github.com/gagliardetto/solana-go"
)

// SimpleNodeConfig represents the configuration for our example node
type SimpleNodeConfig struct {
	NodeID         string
	PrivateKey     string
	QUICPort       int
	TCPPort        int
	BootstrapIP    string
	BootstrapQUIC  int
	BootstrapTCP   int
	BootstrapKey   string
	IsBootstrap    bool
	AuthorizedKeys []string
}

// SimpleNode represents our example node software
type SimpleNode struct {
	config        SimpleNodeConfig
	db            *pubsub.DB
	logger        common.Logger
	subscriptions map[string]bool
}

// Mock registry - in real node software, this would query smart contracts
func (node *SimpleNode) getAuthorizedWallets(ctx context.Context) ([]solana.PublicKey, error) {
	var authorizedKeys []string

	if len(node.config.AuthorizedKeys) > 0 {
		// Use provided authorized keys
		authorizedKeys = node.config.AuthorizedKeys
	} else {
		// Fallback to default keys for testing
		authorizedKeys = []string{
			"jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN",  // Node 0
			"FLE6aWnEhSqVeyUHzpkHmaxhPJEJj61foYqkL9z1qJpL", // Node 1
			"Dg99EAH5xgLFSqCo8DSsUbaW6wNdFXWVNnXmqofodoKd", // Node 2
			"8vQm2x5MHqYgRnE3DpWvF9BzGkTnJL2wHxCr7ZkNfK4m", // Node 3 (for testing)
		}
	}

	var wallets []solana.PublicKey
	for _, keyStr := range authorizedKeys {
		key, err := solana.PublicKeyFromBase58(keyStr)
		if err != nil {
			node.logger.Error("Invalid authorized public key", "key", keyStr, "error", err)
			continue
		}
		wallets = append(wallets, key)
	}

	node.logger.Info("Registry query", "authorized_wallets", len(wallets), "keys", authorizedKeys)
	return wallets, nil
}

// Mock bootstrap nodes - in real node software, this would query registry
func (node *SimpleNode) getBootstrapNodes(ctx context.Context) ([]common.BootstrapNode, error) {
	if node.config.IsBootstrap {
		// Bootstrap node doesn't need to connect to other bootstrap nodes
		return []common.BootstrapNode{}, nil
	}

	// Return the configured bootstrap node
	if node.config.BootstrapIP != "" && node.config.BootstrapKey != "" {
		return []common.BootstrapNode{
			{
				PublicKey: solana.MustPublicKeyFromBase58(node.config.BootstrapKey),
				IP:        node.config.BootstrapIP,
				QUICPort:  node.config.BootstrapQUIC,
				TCPPort:   node.config.BootstrapTCP,
			},
		}, nil
	}

	return []common.BootstrapNode{}, nil
}

// Connect initializes the P2P connection
func (node *SimpleNode) Connect(ctx context.Context) error {
	config := common.Config{
		WalletPrivateKey:     node.config.PrivateKey,
		DatabaseName:         "depin-example-network",
		GetAuthorizedWallets: node.getAuthorizedWallets,
		GetBootstrapNodes:    node.getBootstrapNodes,
		Logger:               node.logger,
		ListenPorts: common.ListenPorts{
			QUIC: node.config.QUICPort,
			TCP:  node.config.TCPPort,
		},
	}

	var err error
	node.db, err = pubsub.Connect(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to connect to P2P network: %w", err)
	}

	node.logger.Info("Connected to P2P network",
		"peer_id", node.db.GetHost().ID().String(),
		"node_id", node.config.NodeID)

	return nil
}

// Subscribe to a topic
func (node *SimpleNode) Subscribe(topic string) error {
	if node.subscriptions[topic] {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	handler := func(event common.Event) {
		fmt.Printf("\n🔔 [%s] Received message on topic '%s':\n",
			time.Now().Format("15:04:05"), topic)
		fmt.Printf("   From: %s\n", event.FromPeerId)
		fmt.Printf("   ID: %s\n", event.ID)

		// Pretty print the message
		if msgBytes, err := json.MarshalIndent(event.Message, "   ", "  "); err == nil {
			fmt.Printf("   Message: %s\n", string(msgBytes))
		} else {
			fmt.Printf("   Message: %v\n", event.Message)
		}
		fmt.Print("\n> ") // Prompt for next command
	}

	err := node.db.Subscribe(context.Background(), topic, handler)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	node.subscriptions[topic] = true
	fmt.Printf("✅ Subscribed to topic: %s\n", topic)
	return nil
}

// Publish a message to a topic
func (node *SimpleNode) Publish(topic, message string) error {
	// Try to parse as JSON, fallback to string
	var messageData interface{}
	if err := json.Unmarshal([]byte(message), &messageData); err != nil {
		// Not valid JSON, treat as string
		messageData = map[string]interface{}{
			"content":   message,
			"from_node": node.config.NodeID,
			"timestamp": time.Now().Unix(),
		}
	}

	event, err := node.db.Publish(context.Background(), topic, messageData)
	if err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", topic, err)
	}

	fmt.Printf("📤 Published message to topic '%s' (ID: %s)\n", topic, event.ID)
	return nil
}

// Unsubscribe from a topic
func (node *SimpleNode) Unsubscribe(topic string) error {
	if !node.subscriptions[topic] {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	err := node.db.Unsubscribe(context.Background(), topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from topic %s: %w", topic, err)
	}

	delete(node.subscriptions, topic)
	fmt.Printf("✅ Unsubscribed from topic: %s\n", topic)
	return nil
}

// ShowPeers displays connected peers
func (node *SimpleNode) ShowPeers() {
	peers := node.db.ConnectedPeers()
	fmt.Printf("\n👥 Connected Peers (%d):\n", len(peers))
	for i, peer := range peers {
		fmt.Printf("  %d. %s\n", i+1, peer.ID.String())
		for j, addr := range peer.Addrs {
			fmt.Printf("     Address %d: %s\n", j+1, addr.String())
		}
	}
	if len(peers) == 0 {
		fmt.Println("  No peers connected")
	}
	fmt.Println()
}

// ShowStatus displays node status
func (node *SimpleNode) ShowStatus() {
	host := node.db.GetHost()
	fmt.Printf("\n📊 Node Status:\n")
	fmt.Printf("  Node ID: %s\n", node.config.NodeID)
	fmt.Printf("  Peer ID: %s\n", host.ID().String())
	fmt.Printf("  Listen Ports: QUIC:%d, TCP:%d\n", node.config.QUICPort, node.config.TCPPort)
	fmt.Printf("  Is Bootstrap: %v\n", node.config.IsBootstrap)
	fmt.Printf("  Connected Peers: %d\n", len(node.db.ConnectedPeers()))
	fmt.Printf("  Subscribed Topics: %v\n", node.getSubscribedTopics())

	fmt.Printf("  Listen Addresses:\n")
	for i, addr := range host.Addrs() {
		fmt.Printf("    %d. %s\n", i+1, addr.String())
	}
	fmt.Println()
}

func (node *SimpleNode) getSubscribedTopics() []string {
	var topics []string
	for topic := range node.subscriptions {
		topics = append(topics, topic)
	}
	return topics
}

// ShowHelp displays available commands
func (node *SimpleNode) ShowHelp() {
	fmt.Println(`
📋 Available Commands:
  subscribe <topic>          - Subscribe to a topic
  publish <topic> <message>  - Publish a message to a topic
  unsubscribe <topic>        - Unsubscribe from a topic
  peers                      - Show connected peers
  status                     - Show node status
  help                       - Show this help message
  quit                       - Exit the node

Examples:
  subscribe sensors
  publish sensors {"temperature": 25.5, "humidity": 60}
  publish alerts "High temperature detected!"
  unsubscribe sensors`)
}

// RunCLI runs the interactive command line interface
func (node *SimpleNode) RunCLI() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Printf("\n🚀 DePIN Node '%s' Ready!\n", node.config.NodeID)
	fmt.Printf("   Peer ID: %s\n", node.db.GetHost().ID().String())
	node.ShowHelp()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		if len(parts) == 0 {
			continue
		}

		command := strings.ToLower(parts[0])

		switch command {
		case "subscribe", "sub":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: subscribe <topic>")
				continue
			}
			if err := node.Subscribe(parts[1]); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}

		case "publish", "pub":
			if len(parts) < 3 {
				fmt.Println("❌ Usage: publish <topic> <message>")
				continue
			}
			topic := parts[1]
			message := strings.Join(parts[2:], " ")
			if err := node.Publish(topic, message); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}

		case "unsubscribe", "unsub":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: unsubscribe <topic>")
				continue
			}
			if err := node.Unsubscribe(parts[1]); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}

		case "peers":
			node.ShowPeers()

		case "status":
			node.ShowStatus()

		case "help":
			node.ShowHelp()

		case "quit", "exit":
			fmt.Println("👋 Shutting down node...")
			return

		default:
			fmt.Printf("❌ Unknown command: %s (type 'help' for available commands)\n", command)
		}
	}
}

// Disconnect cleans up resources
func (node *SimpleNode) Disconnect() {
	if node.db != nil {
		node.db.Disconnect(context.Background())
	}
}

func main() {
	var (
		nodeID         = flag.String("node-id", "node-1", "Node identifier")
		privateKey     = flag.String("private-key", "", "Base58-encoded Solana private key")
		quicPort       = flag.Int("quic-port", 4001, "QUIC listen port")
		tcpPort        = flag.Int("tcp-port", 4002, "TCP listen port")
		bootstrapIP    = flag.String("bootstrap-ip", "", "Bootstrap node IP (empty for bootstrap mode)")
		bootstrapQUIC  = flag.Int("bootstrap-quic", 4001, "Bootstrap node QUIC port")
		bootstrapTCP   = flag.Int("bootstrap-tcp", 4002, "Bootstrap node TCP port")
		bootstrapKey   = flag.String("bootstrap-key", "", "Bootstrap node public key")
		authorizedKeys = flag.String("authorized-keys", "", "Comma-separated list of authorized public keys")
		generateKey    = flag.Bool("generate-key", false, "Generate a new keypair and exit")
		debug          = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	if *generateKey {
		// Generate a new keypair for testing
		wallet := solana.NewWallet()
		fmt.Printf("Generated new keypair:\n")
		fmt.Printf("Private Key: %s\n", wallet.PrivateKey.String())
		fmt.Printf("Public Key:  %s\n", wallet.PublicKey().String())
		return
	}

	if *privateKey == "" {
		fmt.Println("❌ Error: -private-key is required (use -generate-key to create one)")
		flag.Usage()
		os.Exit(1)
	}

	// Create logger
	var logger *slog.Logger
	if *debug {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug, // Enable debug logs to see libp2p internals
		}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo, // Default to info level
		}))
	}
	pubsubLogger := common.NewSlogLogger(logger)

	// Parse authorized keys
	var authKeys []string
	if *authorizedKeys != "" {
		authKeys = strings.Split(strings.TrimSpace(*authorizedKeys), ",")
		// Trim whitespace from each key
		for i, key := range authKeys {
			authKeys[i] = strings.TrimSpace(key)
		}
	}

	// Create node configuration
	config := SimpleNodeConfig{
		NodeID:         *nodeID,
		PrivateKey:     *privateKey,
		QUICPort:       *quicPort,
		TCPPort:        *tcpPort,
		BootstrapIP:    *bootstrapIP,
		BootstrapQUIC:  *bootstrapQUIC,
		BootstrapTCP:   *bootstrapTCP,
		BootstrapKey:   *bootstrapKey,
		AuthorizedKeys: authKeys,
		IsBootstrap:    *bootstrapIP == "", // Bootstrap mode if no bootstrap IP provided
	}

	// Create and start node
	node := &SimpleNode{
		config:        config,
		logger:        pubsubLogger,
		subscriptions: make(map[string]bool),
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\n👋 Received shutdown signal...")
		cancel()
	}()

	// Connect to P2P network
	fmt.Printf("🔌 Connecting node '%s' to P2P network...\n", config.NodeID)
	if err := node.Connect(ctx); err != nil {
		fmt.Printf("❌ Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer node.Disconnect()

	// Wait a moment for connections to establish
	time.Sleep(2 * time.Second)

	// Start CLI
	node.RunCLI()
}
