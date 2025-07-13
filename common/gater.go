package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// SolanaRegistryGater implements connection gating based on Solana wallet registry
type SolanaRegistryGater struct {
	getAuthorizedWallets GetAuthorizedWalletsFunc // Function provided by node software
	logger               Logger                   // Logger instance
	cache                *AuthorizationCache      // Wallet authorization cache for performance
}

// AuthorizationCache provides simple caching for wallet authorization
type AuthorizationCache struct {
	walletAuthorizations sync.Map // solana.PublicKey -> bool
	mutex                sync.RWMutex
}

// NewSolanaRegistryGater creates a new connection gater with the provided registry function
func NewSolanaRegistryGater(getAuthorizedWallets GetAuthorizedWalletsFunc, logger Logger) *SolanaRegistryGater {
	return &SolanaRegistryGater{
		getAuthorizedWallets: getAuthorizedWallets,
		logger:               logger,
		cache:                &AuthorizationCache{},
	}
}

// InterceptPeerDial validates outgoing connections against registry
func (g *SolanaRegistryGater) InterceptPeerDial(p peer.ID) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	authorized, err := g.isAuthorizedPeer(ctx, p)
	if err != nil {
		g.logger.Warn("Failed to check peer authorization for outgoing connection",
			"peer_id", p.String(),
			"error", err.Error())
		return false
	}

	if !authorized {
		g.logger.Debug("Blocking outgoing connection to unauthorized peer",
			"peer_id", p.String())
		return false
	}

	g.logger.Debug("Allowing outgoing connection to authorized peer",
		"peer_id", p.String())
	return true
}

// InterceptAddrDial validates specific address connections
func (g *SolanaRegistryGater) InterceptAddrDial(p peer.ID, m multiaddr.Multiaddr) bool {
	// Use the same logic as InterceptPeerDial
	return g.InterceptPeerDial(p)
}

// InterceptAccept validates incoming connections (initial filter)
func (g *SolanaRegistryGater) InterceptAccept(network.ConnMultiaddrs) bool {
	// Allow all incoming connections at address level
	// Authorization happens in InterceptSecured after handshake
	return true
}

// InterceptSecured performs post-handshake authorization via wallet registry lookup
func (g *SolanaRegistryGater) InterceptSecured(direction network.Direction, p peer.ID, connMultiaddr network.ConnMultiaddrs) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	authorized, err := g.isAuthorizedPeer(ctx, p)
	if err != nil {
		g.logger.Warn("Failed to check peer authorization after handshake",
			"peer_id", p.String(),
			"direction", direction.String(),
			"error", err.Error())
		return false
	}

	if !authorized {
		g.logger.Info("Blocking connection from unauthorized peer",
			"peer_id", p.String(),
			"direction", direction.String())
		return false
	}

	g.logger.Info("Accepting connection from authorized peer",
		"peer_id", p.String(),
		"direction", direction.String())
	return true
}

// InterceptUpgraded validates connections after protocol upgrade (no additional checks needed)
func (g *SolanaRegistryGater) InterceptUpgraded(network.Conn) (allow bool, reason control.DisconnectReason) {
	// All authorization already done in InterceptSecured
	return true, 0
}

// isAuthorizedPeer checks if a peer is authorized by looking up its wallet in the registry
func (g *SolanaRegistryGater) isAuthorizedPeer(ctx context.Context, peerID peer.ID) (bool, error) {
	// Extract Solana public key from peer ID
	solanaPublicKey, err := PeerIDToSolanaPublicKey(peerID)
	if err != nil {
		g.logger.Debug("Failed to extract Solana public key from peer ID",
			"peer_id", peerID.String(),
			"error", err.Error())
		return false, err
	}

	// Check wallet authorization (this will use the wallet cache)
	authorized, err := g.checkWalletInRegistry(ctx, solanaPublicKey)
	if err != nil {
		return false, fmt.Errorf("failed to check wallet in registry: %w", err)
	}

	g.logger.Debug("Checked peer authorization against registry",
		"peer_id", peerID.String(),
		"wallet", solanaPublicKey.String(),
		"authorized", authorized)

	return authorized, nil
}

// checkWalletInRegistry validates a wallet against the registry using the provided function
func (g *SolanaRegistryGater) checkWalletInRegistry(ctx context.Context, wallet solana.PublicKey) (bool, error) {
	// Check cache first
	if cached, found := g.cache.walletAuthorizations.Load(wallet); found {
		g.logger.Debug("Found cached wallet authorization",
			"wallet", wallet.String(),
			"authorized", cached.(bool))
		return cached.(bool), nil
	}

	// Get authorized wallets from registry
	authorizedWallets, err := g.getAuthorizedWallets(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get authorized wallets: %w", err)
	}

	// Check if wallet is in the authorized list
	authorized := false
	for _, authorizedWallet := range authorizedWallets {
		if wallet.Equals(authorizedWallet) {
			authorized = true
			break
		}
	}

	// Cache the result
	g.cache.walletAuthorizations.Store(wallet, authorized)

	g.logger.Debug("Checked wallet authorization against registry",
		"wallet", wallet.String(),
		"authorized", authorized,
		"total_authorized_wallets", len(authorizedWallets))

	return authorized, nil
}

// ClearCache clears the authorization cache (useful for testing or manual refresh)
func (g *SolanaRegistryGater) ClearCache() {
	g.cache.mutex.Lock()
	defer g.cache.mutex.Unlock()

	g.cache.walletAuthorizations = sync.Map{}
	g.logger.Debug("Cleared authorization cache")
}
