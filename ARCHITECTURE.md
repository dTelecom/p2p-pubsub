# P2P Distributed Pub/Sub System Architecture for DePIN

## Overview

This system is a **distributed peer-to-peer publish/subscribe network** built on libp2p, designed for secure, decentralized messaging in **DePIN (Decentralized Physical Infrastructure Network) applications**. The system provides **permissionless node participation** through **Solana smart contract registry integration**, where nodes are authorized via on-chain wallet verification.

## Key Characteristics

- **Distributed Pub/Sub**: No central server; all nodes are equal peers
- **DePIN-Optimized**: Designed for decentralized physical infrastructure networks
- **Permissionless Access**: Nodes automatically authorized via Solana smart contract registry
- **Security-First**: Solana wallet-based identity with smart contract gating
- **Bootstrap Discovery**: New nodes join using DHT and registry-based peer discovery  
- **Namespace Isolation**: Single database name acts as namespace for topic keys
- **Real-time Messaging**: Publish and subscribe to arbitrary topics

---

## System Architecture

### Core Components Stack

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

---

## Detailed Component Analysis

### 1. Node Identity & Security

#### Solana-Based Identity
- **Identity Source**: Solana wallet keypairs (Ed25519)
- **Key Conversion**: Solana private key → Ed25519 libp2p key
- **Peer ID Generation**: Deterministic from public key
- **Format**: Base58-encoded private keys for external storage

```go
type Config struct {
    WalletPrivateKey        string                      // Base58-encoded Solana private key
    DatabaseName            string                      // Namespace for topics
    GetAuthorizedWallets    GetAuthorizedWalletsFunc    // Function to get authorized wallets
    GetBootstrapNodes       GetBootstrapNodesFunc       // Function to get bootstrap nodes
    Logger                  Logger                      // Logger for all internal operations
    ListenPorts             ListenPorts                 // Ports for different transports
}

// Function provided by node software to get authorized wallets
type GetAuthorizedWalletsFunc func(ctx context.Context) ([]solana.PublicKey, error)

// Function provided by node software to get bootstrap nodes
type GetBootstrapNodesFunc func(ctx context.Context) ([]BootstrapNode, error)

// Bootstrap node information
type BootstrapNode struct {
    PublicKey   solana.PublicKey    // Node's Solana public key
    IP          string              // IP address
    QUICPort    int                 // QUIC port
    TCPPort     int                 // TCP port
}

// Listen ports for different transports
type ListenPorts struct {
    QUIC    int     // QUIC listen port (default: 4001)
    TCP     int     // TCP listen port (default: 4002)
}
```

#### Registry-Based Connection Gater
```go
type SolanaRegistryGater struct {
    getAuthorizedWallets    GetAuthorizedWalletsFunc  // Function provided by node software
    logger                  Logger                    // Logger instance
    cache                   *AuthorizationCache       // Peer authorization cache for performance
}

// Authorization cache to avoid repeated registry lookups
type AuthorizationCache struct {
    walletAuthorizations    sync.Map                  // solana.PublicKey -> CacheEntry
    peerAuthorizations      sync.Map                  // peer.ID -> bool
    cacheTTL               time.Duration             // How long to cache entries
    lastCleanup            time.Time                 // Last cache cleanup time
    cleanupInterval        time.Duration             // How often to cleanup expired entries
}

type CacheEntry struct {
    authorized   bool
    timestamp    time.Time
}
```

**Network-Level Security Functions**:
- `InterceptPeerDial()`: Validates outgoing connections against registry
- `InterceptAddrDial()`: Validates specific address connections  
- `InterceptSecured()`: Post-handshake authorization via wallet registry lookup
- `checkWalletInRegistry()`: Core validation using provided function
- `isAuthorizedWallet()`: Calls node software's registry function

**Connection Authorization Flow**:
1. **Node Attempts Connection** → Gater checks wallet against registry
2. **If Authorized** → Connection allowed, node joins network
3. **If Unauthorized** → Connection blocked at transport level
4. **Once Connected** → Authorized nodes can access any topics

**Registry Integration (Handled by Node Software)**:
```go
// Example: Node software provides this function to pubsub library
func (node *DePINNode) GetAuthorizedWallets(ctx context.Context) ([]solana.PublicKey, error) {
    // Node software handles:
    // - Smart contract RPC calls
    // - Caching and refresh logic
    // - Error handling and retries
    // - Multiple RPC endpoint failover
    
    node.registryMutex.RLock()
    defer node.registryMutex.RUnlock()
    
    // Return cached authorized wallets from node's internal registry
    return node.cachedAuthorizedWallets, nil
}

// Pubsub gater simply calls the provided function
func (g *SolanaRegistryGater) checkWalletInRegistry(ctx context.Context, wallet solana.PublicKey) (bool, error) {
    authorizedWallets, err := g.getAuthorizedWallets(ctx)
    if err != nil {
        return false, err
    }
    
    for _, authorized := range authorizedWallets {
        if wallet.Equals(authorized) {
            return true, nil
        }
    }
    return false, nil
}
```

### 2. Network Layer

#### libp2p Host Configuration
- **Multiple Transports**: QUIC (primary), TCP (fallback), Circuit Relay (NAT assistance)
- **Connection Manager**: Automatic with connection limits and peer tagging
- **Datacenter Optimization**: Connection limits optimized for datacenter infrastructure
- **NAT Traversal**: QUIC hole punching and Circuit Relay

#### Transport Strategy

**Three-Transport Architecture for Universal Connectivity**:

```go
// Transport priority for DePIN nodes (enabled by default)
transports := []string{
    "QUIC",     // Primary: UDP-based, built-in encryption, connection migration
    "TCP",      // Fallback: Reliable for stable connections
    "Circuit",  // NAT assistance: Relay through other nodes when direct connection fails
}
```

**1. QUIC Transport (Primary)**
- **Multiaddr Format**: `/ip4/<ip>/udp/<port>/quic-v1`
- **Benefits for Datacenter DePIN**:
  - Built-in TLS 1.3 encryption
  - Reduced connection setup time
  - No head-of-line blocking
  - NAT hole punching capabilities

**2. TCP Transport (Fallback)**
- **Multiaddr Format**: `/ip4/<ip>/tcp/<port>`
- **Benefits for Datacenter DePIN**:
  - Universal compatibility
  - Reliable for stable connections
  - Excellent for datacenter infrastructure
  - Works through most firewalls

**3. Circuit Relay (NAT Assistance)**
- **Multiaddr Format**: `/ip4/<relay-ip>/tcp/<port>/p2p/<relay-peer-id>/p2p-circuit`
- **Benefits for Datacenter DePIN**:
  - Connects nodes that can't establish direct connections
  - Helps with network bootstrapping
  - Provides connectivity fallback for complex network topologies



#### DePIN-Optimized Peer Discovery
1. **Registry-Based Discovery**: Query smart contract for active nodes
2. **DHT Discovery**: Kademlia-based peer finding with rendezvous
3. **Gossip Discovery**: Peer discovery through topic participation
4. **Bootstrap Redundancy**: Multiple fallback discovery mechanisms

```go
// DePIN discovery process
rendezvous := DiscoveryTag + "_" + db.Name  // "p2p-database-discovery_<dbname>"
routingDiscovery := routing.NewRoutingDiscovery(globalDHT)

// Advertise this node to the network
util.Advertise(ctx, routingDiscovery, rendezvous)

// Find peers through DHT discovery
util.FindPeers(ctx, routingDiscovery, rendezvous)
// Note: Authorization happens at connection time via gater
```

### 3. Pub/Sub Messaging Layer

#### GossipSub Protocol
- **Protocol**: libp2p GossipSub for reliable message propagation  
- **Topic Format**: `<database_name>_<user_topic>`
- **Message Structure**: JSON-encoded Event objects
- **Deduplication**: Built-in message ID tracking
- **DePIN Optimizations**: Bandwidth-aware routing, adaptive mesh overlay

#### Event Structure
```go
type Event struct {
    ID         string      // UUID for deduplication
    FromPeerId string      // Sender's peer ID
    Message    interface{} // Arbitrary message payload
    Timestamp  int64       // Unix timestamp
}
```

#### Topic Management
```go
type TopicSubscription struct {
    subscription *pubsub.Subscription  // libp2p subscription
    topic        *pubsub.Topic         // Topic handle
    handler      PubSubHandler         // User callback function
}
```

**Topic Lifecycle**:
1. **Join**: `pubSub.Join(db.Name + "_" + topic)`
2. **Subscribe**: `topic.Subscribe()` + event listener goroutine
3. **Publish**: JSON marshal + `topic.Publish()`
4. **Leave**: `subscription.Cancel()` + `topic.Close()`

### 4. Shared Infrastructure Management

#### Process-Level Resource Sharing
```go
// Shared libp2p infrastructure managed by process-level manager
type P2PInfrastructure struct {
    host         host.Host                                      // Single libp2p host for all databases
    dht          *dual.DHT                                     // Kademlia DHT
    gossipSub    *pubsub.PubSub                               // GossipSub instance
    connManager  connmgr.ConnManager                          // Connection manager
    
    // Database-specific resources
    databases    map[string]*DatabaseInstance                 // Active database instances
    mutex        sync.RWMutex                                 // Protects databases map
}

// Per-database instance with isolated topics
type DatabaseInstance struct {
    name                string                                 // Database name
    gater              *SolanaRegistryGater                   // Registry-based connection gater
    topics             map[string]*pubsub.Topic               // Joined topics
    subscriptions      map[string]*TopicSubscription          // Active subscriptions
    mutex              sync.RWMutex                           // Protects topics/subscriptions
}
```

#### Initialization Strategy

**Process-Level Initialization (Once)**:
```go
func initializeP2PInfrastructure(config Config) (*P2PInfrastructure, error) {
    // 1. Create libp2p host with all transports
    host, err := libp2p.New(
        libp2p.ListenAddrStrings(
            fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", config.ListenPorts.QUIC),
            fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.ListenPorts.TCP),
        ),
        libp2p.Transport(quic.NewTransport),
        libp2p.Transport(tcp.NewTCPTransport),
        libp2p.EnableCircuitRelay(),
        libp2p.ConnectionManager(connManager),
        libp2p.NATPortMap(),
    )
    
    // 2. Initialize DHT for peer discovery
    dht, err := dual.New(ctx, host)
    
    // 3. Initialize GossipSub for pub/sub messaging
    gossipSub, err := pubsub.NewGossipSub(ctx, host)
    
    // 4. Bootstrap connection to network
    return connectToNetwork(host, dht, config.GetBootstrapNodes)
}
```

**Database-Level Initialization (Per Database)**:
```go
func createDatabaseInstance(infra *P2PInfrastructure, config Config) (*DB, error) {
    // 1. Create registry-based connection gater
    gater := &SolanaRegistryGater{
        getAuthorizedWallets: config.GetAuthorizedWallets,
        logger:              config.Logger,
        cache:               newAuthorizationCache(),
    }
    
    // 2. Apply connection gating to host
    infra.host.Network().Notify(gater)
    
    // 3. Create database instance
    dbInstance := &DatabaseInstance{
        name:          config.DatabaseName,
        gater:         gater,
        topics:        make(map[string]*pubsub.Topic),
        subscriptions: make(map[string]*TopicSubscription),
    }
    
    return &DB{infrastructure: infra, instance: dbInstance}, nil
}
```

---

## Message Flow Architecture

### Publishing Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │  Topic      │    │  GossipSub  │    │   Network   │
│             │    │  Manager    │    │             │    │             │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │                   │
       │ Publish(topic,    │                   │                   │
       │ message)          │                   │                   │
       │──────────────────▶│                   │                   │
       │                   │                   │                   │
       │                   │ Join topic if     │                   │
       │                   │ not exists        │                   │
       │                   │──────────────────▶│                   │
       │                   │                   │                   │
       │                   │ Create Event{     │                   │
       │                   │   ID: uuid,       │                   │
       │                   │   FromPeerId,     │                   │
       │                   │   Message,        │                   │
       │                   │   Timestamp }     │                   │
       │                   │                   │                   │
       │                   │ JSON.Marshal      │                   │
       │                   │ (Event)           │                   │
       │                   │                   │                   │
       │                   │ topic.Publish()   │                   │
       │                   │──────────────────▶│                   │
       │                   │                   │                   │
       │                   │                   │ Gossip to         │
       │                   │                   │ authorized peers  │
       │                   │                   │──────────────────▶│
       │                   │                   │                   │
       │ Return Event{ID}  │                   │                   │
       │◀──────────────────│                   │                   │
```

### Subscription Flow
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Client    │    │  Topic      │    │  Event      │    │  Message    │
│             │    │  Manager    │    │  Listener   │    │  Handler    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │                   │
       │ Subscribe(topic,  │                   │                   │
       │ handler)          │                   │                   │
       │──────────────────▶│                   │                   │
       │                   │                   │                   │
       │                   │ Join topic        │                   │
       │                   │ topic.Subscribe() │                   │
       │                   │                   │                   │
       │                   │ Start event       │                   │
       │                   │ listener goroutine│                   │
       │                   │──────────────────▶│                   │
       │                   │                   │                   │
       │                   │                   │ Listen loop:      │
       │                   │                   │ subscription.     │
       │                   │                   │ Next()            │
       │                   │                   │                   │
       │                   │                   │ JSON.Unmarshal   │
       │                   │                   │ → Event          │
       │                   │                   │                   │
       │                   │                   │ handler(Event)    │
       │                   │                   │──────────────────▶│
       │                   │                   │                   │
       │ Registration      │                   │                   │
       │ successful        │                   │                   │
       │◀──────────────────│                   │                   │
```

**Note**: Authorization happens at connection time via the connection gater. Once nodes are connected to the network, they can freely subscribe to and publish on any topics.

---

## Security Architecture for DePIN

### Authorization Model

#### Two-Layer Security for DePIN
1. **Smart Contract Level**: On-chain wallet registry provides Sybil resistance
2. **Network Level**: Connection gater blocks unauthorized peers at connection time
3. **Namespace Level**: Database name provides network isolation (different DePIN networks)
4. **Application Level**: Custom message validation (if needed)

**Important**: Once a node passes connection gating (wallet in registry), it can access all topics within that database namespace. Topic-level access control is not implemented at the pub/sub layer.

#### DePIN Threat Model Protection
- **Economic Attacks**: Smart contract registry prevents low-cost identity creation
- **Malicious Hardware**: Wallet-based identity tied to physical device ownership
- **Network Partitioning**: DHT and GossipSub provide resilience
- **Resource Exhaustion**: Connection limits protect against DoS
- **Identity Spoofing**: Cryptographic peer IDs prevent impersonation
- **Infrastructure Attacks**: Network topology monitoring for datacenter infrastructure

#### Smart Contract Registry vs Manual Node Lists
- **Permissionless Operation**: New nodes automatically authorized via smart contract
- **Dynamic Authorization**: Real-time registry updates without node restarts
- **Decentralized Control**: No single point of failure for node authorization
- **Audit Trail**: On-chain record of all authorization changes

### Connection Management for DePIN

#### Connection-Level Authorization
- **Authorization Point**: Connection gater validates wallet during connection handshake
- **One-Time Check**: Once connected, nodes have access to all topics in database namespace
- **No Per-Topic Gating**: Topic subscription/publishing is unrestricted for connected nodes
- **Network Isolation**: Different database names create separate networks with separate authorization

```go
// Connection flow
1. Node A attempts to connect to Node B
2. Connection gater checks Node A's wallet against registry
3. If authorized: Connection established, Node A can use any topics
4. If unauthorized: Connection rejected at transport level
```

#### Resource-Aware Scaling
- **Bandwidth Adaptation**: Message routing based on connection quality
- **Datacenter Distribution**: Latency-based peer prioritization across datacenters

---

## API Interfaces

### Core Library Interface
```go
// Primary connection function with registry function
func Connect(ctx context.Context, config Config) (*DB, error)

// Pub/Sub operations (no additional authorization - connection-level gating only)
func (db *DB) Subscribe(ctx context.Context, topic string, handler PubSubHandler) error
func (db *DB) Publish(ctx context.Context, topic string, value interface{}) (Event, error)

// Network information
func (db *DB) ConnectedPeers() []*peer.AddrInfo
func (db *DB) GetHost() host.Host

// Lifecycle management  
func (db *DB) Disconnect(ctx context.Context) error
```

### HTTP API (cmd/p2p-node)
```http
POST /subscribe          # Subscribe to topic
POST /publish            # Publish message  
GET  /status             # Node status
GET  /peers              # Connected peers
GET  /messages           # Retrieve received messages
GET  /health             # Health check
```

**Note**: Registry management endpoints (`/registry/*`) are handled by the node software, not the pubsub library.

### Event Handler Interface
```go
type PubSubHandler func(Event)

type Event struct {
    ID         string      // Unique message identifier
    FromPeerId string      // Sender's peer ID  
    Message    interface{} // Message payload (any JSON-serializable data)
    Timestamp  int64       // Unix timestamp
}
```

---

## Message Delivery Behavior

### Self-Message Filtering

**Critical Implementation Detail**: Nodes **ALWAYS** skip messages they send themselves to prevent echo loops in the distributed network.

```go
// In messageListener goroutine (pubsub_operations.go)
if msg.ReceivedFrom == db.infrastructure.host.ID() {
    continue  // Always skip own messages - no exceptions
}
```

**Why This Matters:**
- **Echo Prevention**: Without self-filtering, nodes would receive their own messages infinitely
- **Network Efficiency**: Prevents unnecessary message processing and storage
- **Consistent Behavior**: All P2P networks implement this fundamental rule

**Developer Implications:**
- **Single-Node Testing**: Cannot verify message delivery with only one node
- **Multi-Node Requirement**: Message delivery verification requires at least 2 nodes
- **Handler Behavior**: Message handlers will never receive messages published by the same node

### Testing Requirements

**Minimum Node Count for Testing**: All tests verifying message delivery must use **at least 2 nodes**.

#### ✅ Correct Test Pattern
```go
func TestMessageDelivery(t *testing.T) {
    // Create at least 2 nodes
    node1 := setupNode(t, config1)
    node2 := setupNode(t, config2)
    
    // Subscribe on receiving node
    node2.Subscribe(ctx, "test-topic", handler)
    
    // Publish from sending node  
    node1.Publish(ctx, "test-topic", message)
    
    // Verify node2 received the message from node1
    verifyMessageReceived(t, receivedMessages)
}
```

#### ❌ Incorrect Test Pattern  
```go
func TestMessageDelivery(t *testing.T) {
    // Single node - CANNOT verify message delivery
    node := setupNode(t, config)
    
    node.Subscribe(ctx, "test-topic", handler)
    node.Publish(ctx, "test-topic", message)
    
    // This will ALWAYS fail - nodes never receive own messages
    verifyMessageReceived(t, receivedMessages) // Will be empty
}
```

#### Test Architecture Recommendations

**Integration Test Pattern**:
```go
// Use multiple nodes to simulate real network conditions
nodes := []struct{
    id   string
    role string
}{
    {"bootstrap", "message receiver"},
    {"client1", "message sender"},  
    {"client2", "message sender"},
}

// Each test should verify:
// 1. Messages sent by node A are received by nodes B, C
// 2. Messages sent by node B are received by nodes A, C  
// 3. Messages sent by node C are received by nodes A, B
// 4. No node receives its own messages
```

**Topic Isolation Testing**:
```go
// Multi-node, multi-topic verification
func TestTopicIsolation(t *testing.T) {
    node1.Subscribe(ctx, "topic-a", handlerA)
    node2.Subscribe(ctx, "topic-b", handlerB)
    node3.Subscribe(ctx, "topic-a", handlerA) // Same topic as node1
    
    // Verify topic isolation across nodes
    node1.Publish(ctx, "topic-a", messageA)  // Should reach node3, not node2
    node2.Publish(ctx, "topic-b", messageB)  // Should not reach node1 or node3
}
```

---

## Technology Stack

### Core Dependencies (Updated for DePIN)
```go
// go.mod - Go 1.23.11
module github.com/your-org/p2p-pubsub

go 1.23.11

require (
    github.com/libp2p/go-libp2p v0.42.0                    // Latest libp2p networking
    github.com/libp2p/go-libp2p-pubsub v0.12.0             // Latest GossipSub
    github.com/libp2p/go-libp2p-kad-dht v0.27.0            // Latest Kademlia DHT
    github.com/libp2p/go-libp2p-quic-transport v0.10.0     // QUIC transport (primary)
    github.com/libp2p/go-libp2p-circuit v0.20.0            // Circuit relay transport
    github.com/gagliardetto/solana-go v1.12.0              // Solana integration
    github.com/multiformats/go-multiaddr v0.12.0           // Multiaddress support
    github.com/ipfs/go-datastore v0.6.0                    // Local storage
    github.com/google/uuid v1.5.0                          // Event ID generation
)
```

### DePIN-Specific Optimizations
- **Startup Performance**: Fast initialization optimized for DePIN deployment
- **Network Efficiency**: Efficient pub/sub routing with multi-transport optimization

### Logging Interface

The pubsub library uses a simple logging interface to remain logging-library agnostic:

```go
package common

import "log/slog"

// Logger interface for structured logging
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}

// NewSlogLogger creates a logger using Go's standard slog (recommended for Go 1.23.11+)
func NewSlogLogger(logger *slog.Logger) Logger {
    return &slogLogger{logger: logger}
}

type slogLogger struct {
    logger *slog.Logger
}

func (l *slogLogger) Debug(msg string, keysAndValues ...interface{}) {
    l.logger.Debug(msg, keysAndValues...)
}

func (l *slogLogger) Info(msg string, keysAndValues ...interface{}) {
    l.logger.Info(msg, keysAndValues...)
}

func (l *slogLogger) Warn(msg string, keysAndValues ...interface{}) {
    l.logger.Warn(msg, keysAndValues...)
}

func (l *slogLogger) Error(msg string, keysAndValues ...interface{}) {
    l.logger.Error(msg, keysAndValues...)
}
```

#### Usage Example with slog
```go
import (
    "log/slog"
    "os"
)

// Create structured JSON logger
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    AddSource: true,  // Add source file information
}))

// Create pubsub logger wrapper
pubsubLogger := common.NewSlogLogger(logger)

// Connect with logger in config
config := Config{
    WalletPrivateKey:     "your-wallet-key",
    DatabaseName:         "depin-network",
    GetAuthorizedWallets: node.GetAuthorizedWallets,
    GetBootstrapNodes:    node.GetBootstrapNodes,
    Logger:               pubsubLogger,
    ListenPorts: ListenPorts{
        QUIC: 4001,
        TCP:  4002,
    },
}

db, err := Connect(context.Background(), config)
```

#### Alternative Logger Implementations

The interface can be implemented with other popular logging libraries:

```go
// Using zap
import "go.uber.org/zap"

type ZapLogger struct {
    logger *zap.SugaredLogger
}

func (l *ZapLogger) Info(msg string, keysAndValues ...interface{}) {
    l.logger.Infow(msg, keysAndValues...)
}

// Using logrus  
import "github.com/sirupsen/logrus"

type LogrusLogger struct {
    logger *logrus.Logger
}

func (l *LogrusLogger) Info(msg string, keysAndValues ...interface{}) {
    l.logger.WithFields(logrus.Fields{
        // Convert keysAndValues to fields
    }).Info(msg)
}
```

**Recommendation**: Use Go's standard `slog` package for new projects as it provides excellent performance, structured logging, and is part of the standard library since Go 1.21.

## Error Handling & Resilience for DePIN

**Note**: Smart contract resilience (RPC failover, caching, retry logic) is handled by the node software, not the pubsub library.


### Smart Contract Test Integration
```go
// Test with mock smart contract registry
func TestDePINNetworkWithRegistry(t *testing.T) {
    // Deploy mock registry smart contract
    mockRegistry := NewMockRegistryContract()
    
    // Add test wallets to registry
    testWallets := generateTestWallets(10)
    for _, wallet := range testWallets {
        mockRegistry.AddAuthorizedWallet(wallet)
    }
    
    // Start test nodes with registry integration
    nodes := startTestNodes(testWallets, mockRegistry.RPCEndpoint())
    
    // Test pub/sub functionality
    testPubSubAcrossNodes(nodes)
    
    // Test unauthorized wallet rejection
    testUnauthorizedWalletBlocking(nodes, generateTestWallets(1))
}
```

## Conclusion

This updated P2P distributed pub/sub system is specifically optimized for **DePIN (Decentralized Physical Infrastructure Network)** applications. Key improvements include:

### Major Architectural Changes
1. **Clean Separation of Concerns**: Pubsub library focuses on messaging; node software handles registry and bootstrap management
2. **Function-Based Integration**: Simple function interfaces for authorization and bootstrap discovery without RPC complexity
3. **Three-Transport Architecture**: QUIC (primary), TCP (fallback), and Circuit Relay (NAT assistance) for universal connectivity
4. **Process-Level Resource Sharing**: Efficient infrastructure sharing across multiple database instances
5. **Connection-Level Security**: Registry-based authorization at connection time, not per-topic
6. **Latest Technology Stack**: Go 1.23.11, libp2p v0.42.0, solana-go v1.12.0

### DePIN-Specific Benefits
- **Permissionless Participation**: Nodes automatically authorized via smart contract (handled by node software)
- **Simple Integration**: Pubsub library receives authorization via function call, no RPC complexity
- **Universal Connectivity**: Multi-transport strategy (QUIC + TCP + Circuit Relay) ensures nodes can connect regardless of NAT/firewall restrictions
- **Economic Integration**: Registry tied to DePIN tokenomics and staking mechanisms  
- **Infrastructure Resilience**: Multi-region datacenter support with adaptive transport selection
- **Scalability**: Support for thousands of datacenter nodes with optimized connectivity

### Security & Reliability
- **Sybil Resistance**: Smart contract registry prevents cheap identity creation
- **Datacenter Distribution**: Built-in monitoring and optimization across datacenter infrastructure
- **Economic Security**: Integration with DePIN reward/penalty mechanisms
- **Infrastructure Protection**: Resource limits and reputation-based peer management

This architecture provides a robust foundation for DePIN applications requiring secure, scalable, and economically-integrated peer-to-peer communication infrastructure. The clean separation of concerns ensures the pubsub library remains focused on messaging performance while the node software handles all smart contract complexity, enabling truly permissionless and decentralized operation. 