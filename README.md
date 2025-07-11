# P2P Distributed Pub/Sub System for DePIN

A distributed peer-to-peer publish/subscribe network built on libp2p, designed for secure, decentralized messaging in **DePIN (Decentralized Physical Infrastructure Network)** applications. The system provides **permissionless node participation** through **Solana smart contract registry integration**.

## Features

- **Distributed P2P Architecture**: No central servers, all nodes are equal peers
- **Solana Integration**: Identity and authorization via Solana wallet keypairs
- **Smart Contract Registry**: Permissionless node participation via on-chain authorization
- **Multi-Transport Support**: QUIC (primary), TCP (fallback), Circuit Relay (NAT assistance)
- **Real-time Messaging**: Publish/subscribe to arbitrary topics with JSON payloads
- **Namespace Isolation**: Database-based topic namespacing for network segmentation
- **Production Ready**: Structured logging, connection management, and error handling

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                        │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │            Go Library Interface                          │ │
│  │           (Subscribe/Publish)                            │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    P2P Pub/Sub Layer                        │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │  Topic Manager  │    │      Event Routing              │ │
│  │   (Pub/Sub)     │    │   (Message Distribution)        │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Security Layer                           │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │Registry Gater   │    │    Solana Identity              │ │
│  │(Smart Contract) │    │  (Ed25519 Keys)                 │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    libp2p Networking Layer                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │   GossipSub     │  │   Kademlia DHT  │  │ Connection  │ │
│  │   (Pub/Sub)     │  │  (Discovery)    │  │  Manager    │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                    Transport Layer                          │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────┐ │
│  │      QUIC       │    │     TCP     │    │   Circuit   │ │
│  │  (Primary)      │    │ (Fallback)  │    │   (Relay)   │ │
│  └─────────────────┘    └─────────────┘    └─────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Go 1.23 or later
- Solana wallet private key (Base58 encoded)
- Access to Solana RPC endpoint (for registry integration)

### Installation

```bash
git clone https://github.com/dtelecom/p2p-pubsub.git
cd p2p-pubsub
go mod download
```

### Using as a Library

```go
package main

import (
    "context"
    "log/slog"
    "os"
    
    "github.com/dtelecom/p2p-pubsub/common"
    "github.com/dtelecom/p2p-pubsub/pubsub"
    "github.com/gagliardetto/solana-go"
)

func main() {
    // Create logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    pubsubLogger := common.NewSlogLogger(logger)
    
    // Configuration
    config := common.Config{
        WalletPrivateKey:     "your_base58_encoded_private_key",
        DatabaseName:         "depin-network",
        GetAuthorizedWallets: getAuthorizedWallets, // Your registry function
        GetBootstrapNodes:    getBootstrapNodes,    // Your bootstrap function
        Logger:               pubsubLogger,
        ListenPorts: common.ListenPorts{
            QUIC: 4001,
            TCP:  4002,
        },
    }
    
    // Connect to network
    ctx := context.Background()
    db, err := pubsub.Connect(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Disconnect(ctx)
    
    // Subscribe to messages
    err = db.Subscribe(ctx, "my-topic", func(event common.Event) {
        log.Printf("Received: %v", event.Message)
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Publish a message
    event, err := db.Publish(ctx, "my-topic", map[string]interface{}{
        "content": "Hello, DePIN!",
        "timestamp": time.Now().Unix(),
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Published message with ID: %s", event.ID)
}
```

## Integration Functions

You need to implement two functions for registry integration:

### GetAuthorizedWallets Function

```go
func getAuthorizedWallets(ctx context.Context) ([]solana.PublicKey, error) {
    // Query your smart contract registry
    // Return list of authorized Solana public keys
    return authorizedWallets, nil
}
```

### GetBootstrapNodes Function

```go
func getBootstrapNodes(ctx context.Context) ([]common.BootstrapNode, error) {
    // Return list of bootstrap nodes
    return []common.BootstrapNode{
        {
            PublicKey: solana.PublicKeyFromBase58("..."),
            IP:        "192.168.1.100",
            QUICPort:  4001,
            TCPPort:   4002,
        },
    }, nil
}
```

## Testing

Run the integration tests:

```bash
go test ./test/...
```

Note: Tests require a valid Solana wallet private key for full functionality.

## Configuration

### Environment Variables

- `WALLET_PRIVATE_KEY`: Base58-encoded Solana private key (required)

### Config Structure

```go
type Config struct {
    WalletPrivateKey     string                   // Base58-encoded Solana private key
    DatabaseName         string                   // Namespace for topics
    GetAuthorizedWallets GetAuthorizedWalletsFunc // Function to get authorized wallets
    GetBootstrapNodes    GetBootstrapNodesFunc    // Function to get bootstrap nodes
    Logger               Logger                   // Logger for all internal operations
    ListenPorts          ListenPorts              // Ports for different transports
}
```

## Network Ports

- **QUIC**: 4001 (default, primary transport)
- **TCP**: 4002 (default, fallback transport) 