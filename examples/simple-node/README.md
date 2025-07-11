# Simple Node Example

This example demonstrates how to integrate the p2p-pubsub library into node software with a practical CLI interface.

## Overview

The example shows:
- Mock implementation of `GetAuthorizedWallets` and `GetBootstrapNodes` functions
- Interactive CLI for pub/sub operations
- Two-node communication setup
- Real-time message handling

## Features

- ✅ **Subscribe/Publish/Unsubscribe** to topics
- ✅ **Real-time message reception** with notifications
- ✅ **Peer management** and status monitoring
- ✅ **JSON and text message** support
- ✅ **Bootstrap node discovery**
- ✅ **Authorization** via mock registry

## Quick Start

### Option A: Automated Demo (Recommended)

Run the demo script to automatically generate keys and show you the exact commands:

```bash
cd examples/simple-node
./test-demo.sh
```

This will generate fresh keypairs and provide you with copy-paste commands for two terminals.

### Option B: Manual Setup

### Step 1: Generate Keypairs

First, generate keypairs for two nodes:

```bash
# Generate keypair for Node 1 (Bootstrap)
go run examples/simple-node/main.go -generate-key
# Example output:
# Private Key: gR1mAahBg6XGRSSPLRYyihHVcC2VE5oagGcLPPi81G69Yz1GZvoq8XsjXKjDVD6D8cD6nY3q8GX9PAb7PbsvhB6
# Public Key:  jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN

# Generate keypair for Node 2 (Client)  
go run examples/simple-node/main.go -generate-key
# Example output:
# Private Key: 5GeZEx4FUD1vZEoJsH1KVbtCa2tGw5qhedxMuuNnmYVkMR3XPJpLh8oSLanCwHcDzuK6M3D24BVx1aHb3xsmYgxt
# Public Key:  FLE6aWnEhSqVeyUHzpkHmaxhPJEJj61foYqkL9z1qJpL
```

### Step 2: Start Bootstrap Node (Terminal 1)

```bash
go run examples/simple-node/main.go \
  -node-id "bootstrap-node" \
  -private-key "gR1mAahBg6XGRSSPLRYyihHVcC2VE5oagGcLPPi81G69Yz1GZvoq8XsjXKjDVD6D8cD6nY3q8GX9PAb7PbsvhB6" \
  -quic-port 4001 \
  -tcp-port 4002 \
  -authorized-keys "jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN,FLE6aWnEhSqVeyUHzpkHmaxhPJEJj61foYqkL9z1qJpL"
```

The bootstrap node will start and show:
```
🔌 Connecting node 'bootstrap-node' to P2P network...
🚀 DePIN Node 'bootstrap-node' Ready!
   Peer ID: 12D3KooWAZ44ZvkXvnqX42d8ZWBfmGoGorvBstWdvt6zYCqYCuuJ

📋 Available Commands:
  subscribe <topic>          - Subscribe to a topic
  publish <topic> <message>  - Publish a message to a topic
  ...

> 
```

### Step 3: Start Client Node (Terminal 2)

```bash
go run examples/simple-node/main.go \
  -node-id "client-node" \
  -private-key "5GeZEx4FUD1vZEoJsH1KVbtCa2tGw5qhedxMuuNnmYVkMR3XPJpLh8oSLanCwHcDzuK6M3D24BVx1aHb3xsmYgxt" \
  -quic-port 4003 \
  -tcp-port 4004 \
  -bootstrap-ip "127.0.0.1" \
  -bootstrap-quic 4001 \
  -bootstrap-tcp 4002 \
  -bootstrap-key "jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN" \
  -authorized-keys "jgtpoApjta2e97hvuSKzUiSKXsYrCS9yHjyADgeJyBN,FLE6aWnEhSqVeyUHzpkHmaxhPJEJj61foYqkL9z1qJpL"
```

### Step 4: Test Communication

Now you can test pub/sub communication between the nodes:

#### In Bootstrap Node Terminal:
```bash
> subscribe sensors
✅ Subscribed to topic: sensors

> status
📊 Node Status:
  Node ID: bootstrap-node
  Peer ID: 12D3KooWAZ44ZvkXvnqX42d8ZWBfmGoGorvBstWdvt6zYCqYCuuJ
  Connected Peers: 1
  Subscribed Topics: [sensors]
```

#### In Client Node Terminal:
```bash
> publish sensors {"temperature": 25.5, "humidity": 60, "location": "warehouse"}
📤 Published message to topic 'sensors' (ID: abc123...)
```

#### Bootstrap Node Will Receive:
```bash
🔔 [14:30:25] Received message on topic 'sensors':
   From: 12D3KooWQ9bGKeMwtM6z4ryidRVdYP3XsdGwkn69m9Cmi98ujFYG
   ID: abc123...
   Message: {
     "humidity": 60,
     "location": "warehouse", 
     "temperature": 25.5
   }

> 
```

## CLI Commands

### Subscribe to Topic
```bash
> subscribe <topic>
> subscribe sensors          # Subscribe to sensor data
> subscribe alerts           # Subscribe to alerts
```

### Publish Messages

**JSON Messages:**
```bash
> publish sensors {"temperature": 23.5, "humidity": 55}
> publish alerts {"level": "warning", "message": "High temperature"}
```

**Text Messages:**
```bash  
> publish chat "Hello from node 1!"
> publish logs "System startup complete"
```

### Unsubscribe
```bash
> unsubscribe sensors        # Stop receiving sensor messages
> unsubscribe alerts         # Stop receiving alerts
```

### Show Connected Peers
```bash
> peers
👥 Connected Peers (1):
  1. 12D3KooWQ9bGKeMwtM6z4ryidRVdYP3XsdGwkn69m9Cmi98ujFYG
     Address 1: /ip4/127.0.0.1/tcp/4004
     Address 2: /ip4/127.0.0.1/udp/4003/quic-v1
```

### Show Node Status  
```bash
> status
📊 Node Status:
  Node ID: client-node
  Peer ID: 12D3KooWQ9bGKeMwtM6z4ryidRVdYP3XsdGwkn69m9Cmi98ujFYG
  Listen Ports: QUIC:4003, TCP:4004
  Is Bootstrap: false
  Connected Peers: 1
  Subscribed Topics: [sensors]
```

### Help and Exit
```bash
> help                       # Show available commands
> quit                       # Exit the node
```

## Configuration Options

```bash
Usage: go run examples/simple-node/main.go [options]

Options:
  -node-id string             Node identifier (default "node-1")
  -private-key string         Base58-encoded Solana private key (required)
  -quic-port int             QUIC listen port (default 4001)
  -tcp-port int              TCP listen port (default 4002)
  -bootstrap-ip string       Bootstrap node IP (empty for bootstrap mode)
  -bootstrap-quic int        Bootstrap node QUIC port (default 4001)
  -bootstrap-tcp int         Bootstrap node TCP port (default 4002)  
  -bootstrap-key string      Bootstrap node public key
  -authorized-keys string    Comma-separated list of authorized public keys
  -generate-key              Generate a new keypair and exit
```

## Example Scenarios

### Scenario 1: Sensor Network
```bash
# Node 1: Subscribe to sensor data
> subscribe sensors
> subscribe alerts

# Node 2: Publish sensor readings
> publish sensors {"device":"temp-01", "temperature":22.5, "timestamp":1640995200}
> publish alerts {"level":"info", "message":"All sensors operational"}
```

### Scenario 2: Chat System
```bash
# Both nodes subscribe to chat
> subscribe chat

# Send messages back and forth
> publish chat "Hello from warehouse!"
> publish chat "Received your message, thanks!"
```

### Scenario 3: Topic Isolation Testing
```bash
# Node 1: Subscribe to multiple topics
> subscribe topic-a
> subscribe topic-b

# Node 2: Publish to different topics  
> publish topic-a "Message A"
> publish topic-b "Message B"
> publish topic-c "Message C"    # Node 1 won't receive this
```

## Authorization System

### Understanding Authorization

The P2P network uses Solana public keys to authorize which nodes can participate. Each node must provide:

1. A **private key** (for identity and signing)
2. A list of **authorized public keys** (who can join the network)

### Finding Public Keys for Your Private Keys

If you have private keys but need their corresponding public keys for the `--authorized-keys` parameter:

**Method 1: Use the generate-key utility**
```bash
# Generate a keypair to see the format
go run examples/simple-node/main.go -generate-key
```

**Method 2: Calculate manually in Go**
```go
import "github.com/gagliardetto/solana-go"

privKey, _ := solana.PrivateKeyFromBase58("your-private-key-here")
wallet := solana.NewWallet()
wallet.PrivateKey = privKey
publicKey := wallet.PublicKey().String()
```

**Method 3: Use your existing keys**
For the private keys you provided:
- `7tPK5CN...` → Derive its public key and add to authorized list
- `4678rWx...` → Derive its public key and add to authorized list

### Authorization Flow

1. **Node starts** → Loads private key for identity
2. **Registry query** → Calls `getAuthorizedWallets()` with provided keys
3. **Peer connects** → Network checks if peer's public key is in authorized list
4. **Access granted/denied** → Only authorized peers can join

### Example with Custom Keys

```bash
# First, find your public keys (pseudo-command)
bootstrap_public=$(derive_public_key "7tPK5CN28m3CE8sGwdGgrHXgWoDYjdFKKoBZWY1uaqaEk7MSt4U1SUPGKwtbzivGz9imVAVvugfH71n8JRecKdk")
client_public=$(derive_public_key "4678rWx4mmWpoCbjWHSANnCAVpEXTev8wYdPX6jZvahHciCaFRCBuqoG1FtDFXGuWaaE1PNJaSvtJPQJTAEG9ncC")

# Then use both in your authorized keys list
-authorized-keys "$bootstrap_public,$client_public"
```

## Integration Points

This example demonstrates key integration points for real node software:

### 1. Registry Integration
```go
// Replace this mock with real smart contract queries
func (node *SimpleNode) getAuthorizedWallets(ctx context.Context) ([]solana.PublicKey, error) {
    // In real implementation:
    // - Query Solana smart contract
    // - Handle RPC failures and retries
    // - Cache results with TTL
    // - Return authorized wallet list
}
```

### 2. Bootstrap Discovery
```go
// Replace this mock with real bootstrap node discovery
func (node *SimpleNode) getBootstrapNodes(ctx context.Context) ([]common.BootstrapNode, error) {
    // In real implementation:
    // - Query registry for active bootstrap nodes
    // - Load balance across multiple nodes
    // - Handle node availability
    // - Return peer connection info
}
```

### 3. Configuration Management
```go
// Extend SimpleNodeConfig for production use:
type ProductionNodeConfig struct {
    // Network settings
    SolanaRPCEndpoint string
    RegistryContract  string
    
    // Security settings
    MaxPeers         int
    AuthCacheTTL     time.Duration
    
    // Performance settings  
    MessageBatchSize int
    ReconnectDelay   time.Duration
}
```

## Production Considerations

When integrating into production node software:

1. **Error Handling**: Add comprehensive error handling and retry logic
2. **Logging**: Integrate with your existing logging infrastructure  
3. **Metrics**: Add Prometheus/metrics collection
4. **Configuration**: Use proper config files instead of CLI flags
5. **Security**: Implement proper key management and storage
6. **Testing**: Add unit and integration tests
7. **Documentation**: Add API documentation and deployment guides

## Troubleshooting

### Connection Issues
```bash
# Check if nodes can reach each other
> peers    # Should show connected peers

# Check node addresses  
> status   # Shows listen addresses
```

### Authorization Issues
```bash
# Verify your private key is in the authorized list
# Check the getAuthorizedWallets() function in main.go
```

### Message Delivery Issues
```bash
# Ensure both nodes are subscribed to the same topic
> status   # Shows subscribed topics

# Check network connectivity
> peers    # Should show peer connections
```

This example provides a complete foundation for integrating the p2p-pubsub library into your DePIN node software! 🚀 