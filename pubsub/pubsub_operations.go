package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dtelecom/p2p-pubsub/common"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Subscribe subscribes to a topic and starts listening for messages
func (db *DB) Subscribe(ctx context.Context, topic string, handler common.PubSubHandler) error {
	db.instance.mutex.Lock()
	defer db.instance.mutex.Unlock()

	// Create topic name with database namespace
	topicName := fmt.Sprintf("%s_%s", db.instance.name, topic)

	// Check if already subscribed
	if _, exists := db.instance.subscriptions[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	// Join the topic
	pubsubTopic, err := db.infrastructure.gossipSub.Join(topicName)
	if err != nil {
		return fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}

	// Subscribe to the topic
	subscription, err := pubsubTopic.Subscribe()
	if err != nil {
		pubsubTopic.Close()
		return fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}

	// Create cancellation context for the listener
	listenerCtx, cancel := context.WithCancel(ctx)

	// Create topic subscription
	topicSub := &TopicSubscription{
		subscription: subscription,
		topic:        pubsubTopic,
		handler:      handler,
		cancel:       cancel,
	}

	// Store the subscription
	db.instance.topics[topic] = pubsubTopic
	db.instance.subscriptions[topic] = topicSub

	// Start discovery for this database if it's the first topic
	if len(db.instance.subscriptions) == 1 {
		if err := db.infrastructure.discovery.StartDiscovery(ctx, db.instance.name); err != nil {
			db.infrastructure.logger.Warn("Failed to start discovery", "error", err.Error())
		}
	}

	// Start message listener goroutine
	go db.messageListener(listenerCtx, topicSub, topicName)

	db.infrastructure.logger.Info("Subscribed to topic",
		"topic", topic,
		"full_topic_name", topicName,
		"database", db.instance.name)

	return nil
}

// messageListener listens for messages on a topic subscription
func (db *DB) messageListener(ctx context.Context, topicSub *TopicSubscription, topicName string) {
	defer func() {
		if r := recover(); r != nil {
			db.infrastructure.logger.Error("Message listener panic recovered",
				"topic", topicName,
				"error", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			db.infrastructure.logger.Debug("Message listener stopping",
				"topic", topicName,
				"reason", ctx.Err())
			return
		default:
			// Get next message
			msg, err := topicSub.subscription.Next(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Context cancelled, normal shutdown
					return
				}
				db.infrastructure.logger.Warn("Failed to get next message",
					"topic", topicName,
					"error", err.Error())
				continue
			}

			// Skip messages from ourselves
			if msg.ReceivedFrom == db.infrastructure.host.ID() {
				continue
			}

			// Parse the message
			var event common.Event
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				db.infrastructure.logger.Warn("Failed to unmarshal message",
					"topic", topicName,
					"from", msg.ReceivedFrom.String(),
					"error", err.Error())
				continue
			}

			// Validate the event
			if event.ID == "" {
				db.infrastructure.logger.Warn("Received event with empty ID",
					"topic", topicName,
					"from", msg.ReceivedFrom.String())
				continue
			}

			// Set the peer ID if not already set
			if event.FromPeerId == "" {
				event.FromPeerId = msg.ReceivedFrom.String()
			}

			db.infrastructure.logger.Debug("Received message",
				"topic", topicName,
				"from", msg.ReceivedFrom.String(),
				"event_id", event.ID,
				"timestamp", event.Timestamp)

			// Call the handler
			topicSub.handler(event)
		}
	}
}

// Unsubscribe unsubscribes from a topic and stops listening for messages
func (db *DB) Unsubscribe(ctx context.Context, topic string) error {
	db.instance.mutex.Lock()
	defer db.instance.mutex.Unlock()

	// Check if subscribed to this topic
	topicSub, exists := db.instance.subscriptions[topic]
	if !exists {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	// Cancel the message listener
	if topicSub.cancel != nil {
		topicSub.cancel()
	}

	// Cancel the subscription
	if topicSub.subscription != nil {
		topicSub.subscription.Cancel()
	}

	// Close the topic if no other subscriptions depend on it
	if topicSub.topic != nil {
		topicSub.topic.Close()
	}

	// Remove from maps
	delete(db.instance.subscriptions, topic)
	delete(db.instance.topics, topic)

	db.infrastructure.logger.Info("Unsubscribed from topic",
		"topic", topic,
		"database", db.instance.name)

	return nil
}

// Publish publishes a message to a topic
func (db *DB) Publish(ctx context.Context, topic string, value interface{}) (common.Event, error) {
	db.instance.mutex.RLock()
	defer db.instance.mutex.RUnlock()

	// Create topic name with database namespace
	topicName := fmt.Sprintf("%s_%s", db.instance.name, topic)

	// Get or create the topic
	pubsubTopic, exists := db.instance.topics[topic]
	if !exists {
		// Join the topic if not already joined
		var err error
		pubsubTopic, err = db.infrastructure.gossipSub.Join(topicName)
		if err != nil {
			return common.Event{}, fmt.Errorf("failed to join topic %s: %w", topicName, err)
		}

		// Store the topic (need to unlock and relock for write)
		db.instance.mutex.RUnlock()
		db.instance.mutex.Lock()
		db.instance.topics[topic] = pubsubTopic
		db.instance.mutex.Unlock()
		db.instance.mutex.RLock()
	}

	// Create event
	event := common.Event{
		ID:         uuid.New().String(),
		FromPeerId: db.infrastructure.host.ID().String(),
		Message:    value,
		Timestamp:  time.Now().Unix(),
	}

	// Marshal the event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return common.Event{}, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Publish the message
	if err := pubsubTopic.Publish(ctx, eventData); err != nil {
		return common.Event{}, fmt.Errorf("failed to publish message to topic %s: %w", topicName, err)
	}

	db.infrastructure.logger.Debug("Published message",
		"topic", topic,
		"full_topic_name", topicName,
		"event_id", event.ID,
		"database", db.instance.name)

	return event, nil
}

// ConnectedPeers returns information about connected peers
func (db *DB) ConnectedPeers() []*peer.AddrInfo {
	peers := db.infrastructure.host.Network().Peers()
	var peerInfos []*peer.AddrInfo

	for _, peerID := range peers {
		conns := db.infrastructure.host.Network().ConnsToPeer(peerID)
		if len(conns) > 0 {
			peerInfo := &peer.AddrInfo{
				ID:    peerID,
				Addrs: make([]multiaddr.Multiaddr, 0, len(conns)),
			}
			for _, conn := range conns {
				peerInfo.Addrs = append(peerInfo.Addrs, conn.RemoteMultiaddr())
			}
			peerInfos = append(peerInfos, peerInfo)
		}
	}

	return peerInfos
}

// GetHost returns the underlying libp2p host
func (db *DB) GetHost() host.Host {
	return db.infrastructure.host
}

// Disconnect closes the database connection and cleans up resources
func (db *DB) Disconnect(ctx context.Context) error {
	db.instance.mutex.Lock()
	defer db.instance.mutex.Unlock()

	// Cancel all subscriptions
	for topic, subscription := range db.instance.subscriptions {
		if subscription.cancel != nil {
			subscription.cancel()
		}
		if subscription.subscription != nil {
			subscription.subscription.Cancel()
		}
		delete(db.instance.subscriptions, topic)
	}

	// Close all topics
	for topic, pubsubTopic := range db.instance.topics {
		if pubsubTopic != nil {
			pubsubTopic.Close()
		}
		delete(db.instance.topics, topic)
	}

	// Remove database instance from infrastructure
	db.infrastructure.mutex.Lock()
	delete(db.infrastructure.databases, db.instance.name)
	db.infrastructure.mutex.Unlock()

	db.infrastructure.logger.Info("Disconnected from database",
		"database", db.instance.name)

	return nil
}
