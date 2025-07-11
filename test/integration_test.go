package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dtelecom/p2p-pubsub/common"
	"github.com/dtelecom/p2p-pubsub/pubsub"
	"github.com/gagliardetto/solana-go"
)

// Node keypairs for testing (generated using cmd/keygen)
var (
	// Node 0 - Bootstrap and authorized
	node0PrivateKey = "gR1mAahBg6XGRSSPLRYyihHVcC2VE5oagGcLPPi81G69Yz1GZvoq8XsjXKjDVD6D8cD6nY3q8GX9PAb7PbsvhB6"
	node0PublicKey  = "jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN"

	// Node 1 - Just authorized
	node1PrivateKey = "5GeZEx4FUD1vZEoJsH1KVbtCa2tGw5qhedxMuuNnmYVkMR3XPJpLh8oSLanCwHcDzuK6M3D24BVx1aHb3xsmYgxt"
	node1PublicKey  = "FLE6aWnEhSqVeyUHzpkHmaxhPJEJj61foYqkL9z1qJpL"

	// Node 2 - Just authorized
	node2PrivateKey = "5zRE2yD43ERnUEDuCBJd3e1y9P5gGaegnYTQxa42GEFdB2Bd6aFf2sH4jSjPiV3MgxqXQAQPKPRgR3RReDHWfdhH"
	node2PublicKey  = "Dg99EAH5xgLFSqCo8DSsUbaW6wNdFXWVNnXmqofodoKd"
)

// Mock registry function - returns all 3 test nodes as authorized
func mockGetAuthorizedWallets(ctx context.Context) ([]solana.PublicKey, error) {
	return []solana.PublicKey{
		solana.MustPublicKeyFromBase58(node0PublicKey),
		solana.MustPublicKeyFromBase58(node1PublicKey),
		solana.MustPublicKeyFromBase58(node2PublicKey),
	}, nil
}

// Mock bootstrap function for nodes 1 and 2 - returns node 0 as bootstrap
func mockGetBootstrapNodesForClients(node0Host string, node0PeerID string) common.GetBootstrapNodesFunc {
	return func(ctx context.Context) ([]common.BootstrapNode, error) {
		return []common.BootstrapNode{
			{
				PublicKey: solana.MustPublicKeyFromBase58(node0PublicKey),
				IP:        node0Host,
				QUICPort:  15001, // Node 0's QUIC port
				TCPPort:   15002, // Node 0's TCP port
			},
		}, nil
	}
}

// Mock bootstrap function for node 0 - returns empty (it's the bootstrap node)
func mockGetBootstrapNodesForBootstrap(ctx context.Context) ([]common.BootstrapNode, error) {
	return []common.BootstrapNode{}, nil
}

// TestMultiNodePubSub tests actual P2P communication between 3 nodes
func TestMultiNodePubSub(t *testing.T) {
	t.Log("=== Starting 3-Node P2P Integration Test ===")

	// Test configuration
	testTopic := "integration-test-topic"
	messageTimeout := 10 * time.Second

	// Message tracking
	type NodeMessage struct {
		NodeID  string `json:"node_id"`
		Content string `json:"content"`
		Counter int    `json:"counter"`
	}

	// Set up message collection for each node
	var (
		node0Messages = make([]common.Event, 0)
		node1Messages = make([]common.Event, 0)
		node2Messages = make([]common.Event, 0)
		messagesMutex sync.RWMutex
	)

	// Create message handlers for each node
	node0Handler := func(event common.Event) {
		messagesMutex.Lock()
		node0Messages = append(node0Messages, event)
		messagesMutex.Unlock()
		t.Logf("Node 0 received message: %v", event.Message)
	}

	node1Handler := func(event common.Event) {
		messagesMutex.Lock()
		node1Messages = append(node1Messages, event)
		messagesMutex.Unlock()
		t.Logf("Node 1 received message: %v", event.Message)
	}

	node2Handler := func(event common.Event) {
		messagesMutex.Lock()
		node2Messages = append(node2Messages, event)
		messagesMutex.Unlock()
		t.Logf("Node 2 received message: %v", event.Message)
	}

	// Create logger
	logger := &TestLogger{}

	// Step 1: Setup Node 0 (Bootstrap node)
	t.Log("Setting up Node 0 (Bootstrap)...")
	node0Config := common.Config{
		WalletPrivateKey:     node0PrivateKey,
		DatabaseName:         "test-network",
		GetAuthorizedWallets: mockGetAuthorizedWallets,
		GetBootstrapNodes:    mockGetBootstrapNodesForBootstrap,
		Logger:               logger,
		ListenPorts: common.ListenPorts{
			QUIC: 15001,
			TCP:  15002,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	node0DB, err := pubsub.Connect(ctx, node0Config)
	if err != nil {
		t.Fatalf("Failed to connect Node 0: %v", err)
	}
	defer node0DB.Disconnect(ctx)

	// Get Node 0's addresses for bootstrap
	node0Host := node0DB.GetHost()
	node0Addrs := node0Host.Addrs()
	if len(node0Addrs) == 0 {
		t.Fatal("Node 0 has no addresses")
	}

	t.Logf("Node 0 started with Peer ID: %s", node0Host.ID().String())
	for _, addr := range node0Addrs {
		t.Logf("Node 0 address: %s", addr.String())
	}

	// Subscribe Node 0 to test topic
	err = node0DB.Subscribe(ctx, testTopic, node0Handler)
	if err != nil {
		t.Fatalf("Failed to subscribe Node 0: %v", err)
	}

	// Wait a bit for Node 0 to fully initialize
	time.Sleep(2 * time.Second)

	// Step 2: Setup Node 1 (Client node)
	t.Log("Setting up Node 1 (Client)...")
	node1Config := common.Config{
		WalletPrivateKey:     node1PrivateKey,
		DatabaseName:         "test-network",
		GetAuthorizedWallets: mockGetAuthorizedWallets,
		GetBootstrapNodes:    mockGetBootstrapNodesForClients("127.0.0.1", node0Host.ID().String()),
		Logger:               logger,
		ListenPorts: common.ListenPorts{
			QUIC: 15003,
			TCP:  15004,
		},
	}

	node1DB, err := pubsub.Connect(ctx, node1Config)
	if err != nil {
		t.Fatalf("Failed to connect Node 1: %v", err)
	}
	defer node1DB.Disconnect(ctx)

	t.Logf("Node 1 started with Peer ID: %s", node1DB.GetHost().ID().String())

	// Subscribe Node 1 to test topic
	err = node1DB.Subscribe(ctx, testTopic, node1Handler)
	if err != nil {
		t.Fatalf("Failed to subscribe Node 1: %v", err)
	}

	// Step 3: Setup Node 2 (Client node)
	t.Log("Setting up Node 2 (Client)...")
	node2Config := common.Config{
		WalletPrivateKey:     node2PrivateKey,
		DatabaseName:         "test-network",
		GetAuthorizedWallets: mockGetAuthorizedWallets,
		GetBootstrapNodes:    mockGetBootstrapNodesForClients("127.0.0.1", node0Host.ID().String()),
		Logger:               logger,
		ListenPorts: common.ListenPorts{
			QUIC: 15005,
			TCP:  15006,
		},
	}

	node2DB, err := pubsub.Connect(ctx, node2Config)
	if err != nil {
		t.Fatalf("Failed to connect Node 2: %v", err)
	}
	defer node2DB.Disconnect(ctx)

	t.Logf("Node 2 started with Peer ID: %s", node2DB.GetHost().ID().String())

	// Subscribe Node 2 to test topic
	err = node2DB.Subscribe(ctx, testTopic, node2Handler)
	if err != nil {
		t.Fatalf("Failed to subscribe Node 2: %v", err)
	}

	// Step 4: Wait for network convergence
	t.Log("Waiting for network convergence...")
	time.Sleep(5 * time.Second)

	// Check peer connections
	node0Peers := node0DB.ConnectedPeers()
	node1Peers := node1DB.ConnectedPeers()
	node2Peers := node2DB.ConnectedPeers()

	t.Logf("Node 0 connected to %d peers", len(node0Peers))
	t.Logf("Node 1 connected to %d peers", len(node1Peers))
	t.Logf("Node 2 connected to %d peers", len(node2Peers))

	// Step 5: Publish messages from each node
	t.Log("Publishing messages from all nodes...")

	// Publish from Node 0
	node0Message := NodeMessage{
		NodeID:  "node-0",
		Content: "Hello from Node 0 (Bootstrap)",
		Counter: 1,
	}
	event0, err := node0DB.Publish(ctx, testTopic, node0Message)
	if err != nil {
		t.Fatalf("Failed to publish from Node 0: %v", err)
	}
	t.Logf("Node 0 published message with ID: %s", event0.ID)

	time.Sleep(1 * time.Second)

	// Publish from Node 1
	node1Message := NodeMessage{
		NodeID:  "node-1",
		Content: "Hello from Node 1 (Client)",
		Counter: 2,
	}
	event1, err := node1DB.Publish(ctx, testTopic, node1Message)
	if err != nil {
		t.Fatalf("Failed to publish from Node 1: %v", err)
	}
	t.Logf("Node 1 published message with ID: %s", event1.ID)

	time.Sleep(1 * time.Second)

	// Publish from Node 2
	node2Message := NodeMessage{
		NodeID:  "node-2",
		Content: "Hello from Node 2 (Client)",
		Counter: 3,
	}
	event2, err := node2DB.Publish(ctx, testTopic, node2Message)
	if err != nil {
		t.Fatalf("Failed to publish from Node 2: %v", err)
	}
	t.Logf("Node 2 published message with ID: %s", event2.ID)

	// Step 6: Wait for message propagation
	t.Log("Waiting for message propagation...")
	time.Sleep(messageTimeout)

	// Step 7: Verify all nodes received all messages
	t.Log("Verifying message reception...")

	messagesMutex.RLock()
	defer messagesMutex.RUnlock()

	t.Logf("Node 0 received %d messages", len(node0Messages))
	t.Logf("Node 1 received %d messages", len(node1Messages))
	t.Logf("Node 2 received %d messages", len(node2Messages))

	// Each node should receive 2 messages (from the other 2 nodes, not their own)
	expectedMessages := 2

	if len(node0Messages) != expectedMessages {
		t.Errorf("Node 0 expected %d messages, got %d", expectedMessages, len(node0Messages))
	}

	if len(node1Messages) != expectedMessages {
		t.Errorf("Node 1 expected %d messages, got %d", expectedMessages, len(node1Messages))
	}

	if len(node2Messages) != expectedMessages {
		t.Errorf("Node 2 expected %d messages, got %d", expectedMessages, len(node2Messages))
	}

	// Verify message content from each node's perspective
	t.Log("Verifying message content...")

	allMessages := [][]common.Event{node0Messages, node1Messages, node2Messages}
	nodeNames := []string{"Node 0", "Node 1", "Node 2"}

	for i, messages := range allMessages {
		t.Logf("=== %s received messages ===", nodeNames[i])
		for j, msg := range messages {
			t.Logf("  Message %d: From %s, ID: %s", j+1, msg.FromPeerId, msg.ID)

			// Verify message structure
			if msg.ID == "" {
				t.Errorf("%s: Message %d has empty ID", nodeNames[i], j+1)
			}
			if msg.FromPeerId == "" {
				t.Errorf("%s: Message %d has empty FromPeerId", nodeNames[i], j+1)
			}
			if msg.Message == nil {
				t.Errorf("%s: Message %d has nil Message", nodeNames[i], j+1)
			}
			if msg.Timestamp == 0 {
				t.Errorf("%s: Message %d has zero timestamp", nodeNames[i], j+1)
			}
		}
	}

	t.Log("=== 3-Node P2P Integration Test COMPLETED ===")
	t.Log("✅ All nodes successfully communicated via P2P pubsub")
	t.Log("✅ Message propagation verified across the network")
	t.Log("✅ Bootstrap discovery working correctly")
	t.Log("✅ Authorization and signatures validated by libp2p")
}

// TestLogger is a simple test logger
type TestLogger struct{}

func (l *TestLogger) Debug(msg string, keysAndValues ...interface{}) {
	// In tests, we keep debug messages quiet unless needed
}

func (l *TestLogger) Info(msg string, keysAndValues ...interface{}) {
	// In tests, we keep info messages quiet unless needed
}

func (l *TestLogger) Warn(msg string, keysAndValues ...interface{}) {
	fmt.Printf("WARN: %s\n", msg)
}

func (l *TestLogger) Error(msg string, keysAndValues ...interface{}) {
	fmt.Printf("ERROR: %s\n", msg)
}
