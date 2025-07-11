package pubsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dtelecom/p2p-pubsub/common"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht/dual"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

// P2PInfrastructure manages the shared libp2p resources for all databases
type P2PInfrastructure struct {
	host      host.Host             // Single libp2p host for all databases
	dht       *dual.DHT             // Kademlia DHT for peer discovery
	gossipSub *pubsub.PubSub        // GossipSub instance for pub/sub messaging
	connMgr   *connmgr.BasicConnMgr // Connection manager
	discovery *DiscoveryService     // Peer discovery service

	// Database-specific resources
	databases map[string]*DatabaseInstance // Active database instances
	mutex     sync.RWMutex                 // Protects databases map

	// Configuration
	logger common.Logger
}

// DatabaseInstance represents a per-database instance with isolated topics
type DatabaseInstance struct {
	name          string                        // Database name
	gater         *common.SolanaRegistryGater   // Registry-based connection gater
	topics        map[string]*pubsub.Topic      // Joined topics
	subscriptions map[string]*TopicSubscription // Active subscriptions
	mutex         sync.RWMutex                  // Protects topics/subscriptions
}

// TopicSubscription manages a topic subscription
type TopicSubscription struct {
	subscription *pubsub.Subscription // libp2p subscription
	topic        *pubsub.Topic        // Topic handle
	handler      common.PubSubHandler // User callback function
	cancel       context.CancelFunc   // Cancel function for the listener goroutine
}

// DB represents the main database connection
type DB struct {
	infrastructure *P2PInfrastructure
	instance       *DatabaseInstance
}

// globalInfrastructures stores infrastructure instances keyed by peer ID
var (
	globalInfrastructures = make(map[string]*P2PInfrastructure)
	globalInfraMutex      sync.RWMutex
)

// initializeP2PInfrastructure creates the process-level infrastructure (once per process)
func initializeP2PInfrastructure(config common.Config) (*P2PInfrastructure, error) {
	// Create identity from Solana private key
	privateKey, peerID, err := common.CreateIdentityFromSolanaKey(config.WalletPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	config.Logger.Info("Created libp2p identity from Solana wallet",
		"peer_id", peerID.String())

	// Create connection manager
	connManager, err := connmgr.NewConnManager(
		100, // Low water mark
		400, // High water mark
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	// Create registry-based connection gater
	gater := common.NewSolanaRegistryGater(config.GetAuthorizedWallets, config.Logger)

	// Create libp2p host with all transports
	host, err := libp2p.New(
		libp2p.Identity(privateKey),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", config.ListenPorts.QUIC),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", config.ListenPorts.TCP),
		),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.EnableRelay(),
		libp2p.ConnectionManager(connManager),
		libp2p.ConnectionGater(gater),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	config.Logger.Info("Created libp2p host",
		"addresses", host.Addrs(),
		"peer_id", host.ID().String())

	// Initialize DHT for peer discovery
	dht, err := dual.New(context.Background(), host)
	if err != nil {
		host.Close()
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	config.Logger.Info("Initialized Kademlia DHT")

	// Initialize GossipSub for pub/sub messaging
	gossipSub, err := pubsub.NewGossipSub(context.Background(), host)
	if err != nil {
		host.Close()
		return nil, fmt.Errorf("failed to create GossipSub: %w", err)
	}

	config.Logger.Info("Initialized GossipSub")

	// Create discovery service
	discoveryService := NewDiscoveryService(host, dht, config.Logger)

	// Bootstrap connection to network
	if err := bootstrapNetwork(context.Background(), host, dht, config.GetBootstrapNodes, config.Logger); err != nil {
		config.Logger.Warn("Failed to bootstrap network", "error", err.Error())
		// Don't fail completely, continue without bootstrap
	}

	return &P2PInfrastructure{
		host:      host,
		dht:       dht,
		gossipSub: gossipSub,
		connMgr:   connManager,
		discovery: discoveryService,
		databases: make(map[string]*DatabaseInstance),
		logger:    config.Logger,
	}, nil
}

// bootstrapNetwork connects to bootstrap nodes and starts DHT
func bootstrapNetwork(ctx context.Context, host host.Host, dht *dual.DHT, getBootstrapNodes common.GetBootstrapNodesFunc, logger common.Logger) error {
	// Bootstrap DHT
	if err := dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	// Get bootstrap nodes
	bootstrapNodes, err := getBootstrapNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get bootstrap nodes: %w", err)
	}

	if len(bootstrapNodes) == 0 {
		logger.Info("No bootstrap nodes provided, skipping network bootstrap")
		return nil
	}

	// Connect to bootstrap nodes
	var connectedCount int
	for _, node := range bootstrapNodes {
		// Create multiaddrs for the bootstrap node
		quicAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/udp/%d/quic-v1", node.IP, node.QUICPort))
		if err != nil {
			logger.Warn("Failed to create QUIC multiaddr for bootstrap node",
				"ip", node.IP,
				"port", node.QUICPort,
				"error", err.Error())
			continue
		}

		tcpAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", node.IP, node.TCPPort))
		if err != nil {
			logger.Warn("Failed to create TCP multiaddr for bootstrap node",
				"ip", node.IP,
				"port", node.TCPPort,
				"error", err.Error())
			continue
		}

		// Create peer ID from Solana public key
		peerID, err := common.CreatePeerIDFromSolanaPublicKey(node.PublicKey)
		if err != nil {
			logger.Warn("Failed to create peer ID from Solana public key",
				"public_key", node.PublicKey.String(),
				"error", err.Error())
			continue
		}

		// Create peer info
		peerInfo := peer.AddrInfo{
			ID:    peerID,
			Addrs: []multiaddr.Multiaddr{quicAddr, tcpAddr},
		}

		// Try to connect
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := host.Connect(connectCtx, peerInfo); err != nil {
			logger.Warn("Failed to connect to bootstrap node",
				"peer_id", peerID.String(),
				"addrs", peerInfo.Addrs,
				"error", err.Error())
			cancel()
			continue
		}
		cancel()

		connectedCount++
		logger.Info("Connected to bootstrap node",
			"peer_id", peerID.String(),
			"addrs", peerInfo.Addrs)
	}

	if connectedCount == 0 {
		return fmt.Errorf("failed to connect to any bootstrap nodes")
	}

	logger.Info("Successfully bootstrapped network",
		"connected_nodes", connectedCount,
		"total_bootstrap_nodes", len(bootstrapNodes))

	return nil
}

// createDatabaseInstance creates a new database instance with connection gating
func createDatabaseInstance(infra *P2PInfrastructure, config common.Config) (*DB, error) {
	// Apply connection event notifications
	infra.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			// Connection was allowed by gater
			config.Logger.Debug("Connection established",
				"peer_id", c.RemotePeer().String(),
				"local_addr", c.LocalMultiaddr().String(),
				"remote_addr", c.RemoteMultiaddr().String())
		},
		DisconnectedF: func(n network.Network, c network.Conn) {
			config.Logger.Debug("Connection closed",
				"peer_id", c.RemotePeer().String())
		},
	})

	// Create database instance
	dbInstance := &DatabaseInstance{
		name:          config.DatabaseName,
		gater:         nil, // Gater is set at host level, not per database
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*TopicSubscription),
	}

	// Register database instance
	infra.mutex.Lock()
	infra.databases[config.DatabaseName] = dbInstance
	infra.mutex.Unlock()

	config.Logger.Info("Created database instance",
		"database_name", config.DatabaseName,
		"peer_id", infra.host.ID().String())

	return &DB{
		infrastructure: infra,
		instance:       dbInstance,
	}, nil
}

// Connect is the main entry point for connecting to the P2P pubsub network
func Connect(ctx context.Context, config common.Config) (*DB, error) {
	// Validate configuration
	if config.WalletPrivateKey == "" {
		return nil, fmt.Errorf("wallet private key is required")
	}
	if config.DatabaseName == "" {
		return nil, fmt.Errorf("database name is required")
	}
	if config.GetAuthorizedWallets == nil {
		return nil, fmt.Errorf("GetAuthorizedWallets function is required")
	}
	if config.GetBootstrapNodes == nil {
		return nil, fmt.Errorf("GetBootstrapNodes function is required")
	}
	if config.Logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Set default ports if not provided
	if config.ListenPorts.QUIC == 0 {
		config.ListenPorts.QUIC = 4001
	}
	if config.ListenPorts.TCP == 0 {
		config.ListenPorts.TCP = 4002
	}

	// Create identity to get the peer ID for infrastructure lookup
	_, peerID, err := common.CreateIdentityFromSolanaKey(config.WalletPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity for infrastructure lookup: %w", err)
	}

	peerIDString := peerID.String()

	// Initialize infrastructure for this peer ID if not exists
	globalInfraMutex.Lock()
	infrastructure, exists := globalInfrastructures[peerIDString]
	if !exists {
		infrastructure, err = initializeP2PInfrastructure(config)
		if err != nil {
			globalInfraMutex.Unlock()
			return nil, fmt.Errorf("failed to initialize P2P infrastructure: %w", err)
		}
		globalInfrastructures[peerIDString] = infrastructure
	}
	globalInfraMutex.Unlock()

	// Create database instance
	return createDatabaseInstance(infrastructure, config)
}
