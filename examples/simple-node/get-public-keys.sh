#!/bin/bash

if [ $# -eq 0 ]; then
    echo "Usage: ./get-public-keys.sh <private-key1> [private-key2] [...]"
    echo "Extracts public keys from Solana private keys for use with --authorized-keys"
    exit 1
fi

echo "🔑 Converting private keys to public keys:"
echo ""

PUBLIC_KEYS=()

for i in "$@"; do
    echo "Processing private key: ${i:0:20}..."
    
    # Create a temporary Go program to extract the public key
    cat > temp_key_extractor.go << INNER_EOF
package main

import (
    "fmt"
    "github.com/gagliardetto/solana-go"
)

func main() {
    privKey, err := solana.PrivateKeyFromBase58("$i")
    if err != nil {
        fmt.Printf("ERROR: %v\n", err)
        return
    }
    
    wallet := solana.NewWallet()
    wallet.PrivateKey = privKey
    fmt.Printf("%s\n", wallet.PublicKey().String())
}
INNER_EOF

    PUBLIC_KEY=$(go run temp_key_extractor.go 2>/dev/null)
    if [ $? -eq 0 ]; then
        echo "✅ Public key: $PUBLIC_KEY"
        PUBLIC_KEYS+=("$PUBLIC_KEY")
    else
        echo "❌ Error extracting public key from private key"
    fi
    
    rm -f temp_key_extractor.go
    echo ""
done

if [ ${#PUBLIC_KEYS[@]} -gt 0 ]; then
    echo "📋 For --authorized-keys parameter:"
    echo ""
    
    # Join array with commas
    AUTHORIZED_KEYS_STRING=$(IFS=','; echo "${PUBLIC_KEYS[*]}")
    echo "--authorized-keys \"$AUTHORIZED_KEYS_STRING\""
    echo ""
    
    echo "🚀 Now you can run your nodes with proper authorization!"
fi
