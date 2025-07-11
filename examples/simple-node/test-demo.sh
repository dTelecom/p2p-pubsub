#!/bin/bash

echo "🚀 P2P PubSub Simple Node Demo"
echo "=============================="
echo ""

# Generate keys for two nodes
echo "📋 Step 1: Generating keypairs for demo nodes..."
echo ""

echo "Generating Bootstrap Node keys..."
BOOTSTRAP_OUTPUT=$(go run ../../examples/simple-node/main.go -generate-key 2>/dev/null)
BOOTSTRAP_PRIVATE=$(echo "$BOOTSTRAP_OUTPUT" | grep "Private Key:" | awk '{print $3}')
BOOTSTRAP_PUBLIC=$(echo "$BOOTSTRAP_OUTPUT" | grep "Public Key:" | awk '{print $3}')

echo "Generating Client Node keys..."
CLIENT_OUTPUT=$(go run ../../examples/simple-node/main.go -generate-key 2>/dev/null)
CLIENT_PRIVATE=$(echo "$CLIENT_OUTPUT" | grep "Private Key:" | awk '{print $3}')
CLIENT_PUBLIC=$(echo "$CLIENT_OUTPUT" | grep "Public Key:" | awk '{print $3}')

echo "✅ Generated keys:"
echo "   Bootstrap Node Public: $BOOTSTRAP_PUBLIC"
echo "   Client Node Public: $CLIENT_PUBLIC"
echo ""

# Create authorized keys list
AUTHORIZED_KEYS="$BOOTSTRAP_PUBLIC,$CLIENT_PUBLIC"
echo "🔐 Authorized Keys List: $AUTHORIZED_KEYS"
echo ""

echo "📋 Step 2: Starting nodes..."
echo ""
echo "To start the demo, open TWO terminal windows and run these commands:"
echo ""

echo "🟦 TERMINAL 1 (Bootstrap Node):"
echo "cd $(pwd)"
echo "go run main.go \\"
echo "  -node-id \"bootstrap-node\" \\"
echo "  -private-key \"$BOOTSTRAP_PRIVATE\" \\"
echo "  -quic-port 4001 \\"
echo "  -tcp-port 4002 \\"
echo "  -authorized-keys \"$AUTHORIZED_KEYS\""
echo ""

echo "🟩 TERMINAL 2 (Client Node):"
echo "cd $(pwd)"
echo "go run main.go \\"
echo "  -node-id \"client-node\" \\"
echo "  -private-key \"$CLIENT_PRIVATE\" \\"
echo "  -quic-port 4003 \\"
echo "  -tcp-port 4004 \\"
echo "  -bootstrap-ip \"127.0.0.1\" \\"
echo "  -bootstrap-quic 4001 \\"
echo "  -bootstrap-tcp 4002 \\"
echo "  -bootstrap-key \"$BOOTSTRAP_PUBLIC\" \\"
echo "  -authorized-keys \"$AUTHORIZED_KEYS\""
echo ""

echo "📋 Step 3: Test the communication..."
echo ""
echo "Once both nodes are running, try these commands:"
echo ""
echo "In Bootstrap Node terminal:"
echo "  > subscribe sensors"
echo "  > status"
echo ""
echo "In Client Node terminal:"
echo "  > publish sensors {\"temperature\": 25.5, \"humidity\": 60}"
echo "  > peers"
echo ""
echo "The bootstrap node should receive the sensor message!"
echo ""

echo "💡 More test commands:"
echo "  > subscribe alerts"
echo "  > publish alerts \"High temperature warning!\""
echo "  > unsubscribe sensors"
echo "  > help"
echo "  > quit"
echo ""

echo "🎉 Demo setup complete! Copy the commands above to test the P2P communication." 