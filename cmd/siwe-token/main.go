package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/openrelayxyz/din-caddy-plugins/lib/auth/siwe"
	"go.uber.org/zap"
)

func main() {
	url := os.Args[1]
	keyFile := os.Args[2]

	// Read the key file
	hexKeyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to read secret key file: %v", err.Error()))
	}
	// Trim the 0x prefix and any whitespace
	hexKey := string(hexKeyBytes)
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		panic(fmt.Sprintf("Failed to decode secret key: %v", err.Error()))
	}

	// Create a new signer with the key
	signer := &siwe.SigningConfig{
		PrivateKey: key,
		SignerURL:  "http://localhost",
	}

	// Generate a new keypair
	signer.GenPrivKey()

	// Print the address
	os.Stderr.WriteString("Your signing address: ")
	os.Stderr.WriteString(signer.Address)
	os.Stderr.WriteString("\n")

	// Create a new SIWE client
	client := siwe.NewSIWEClient(url, 0, signer)

	// Start the client
	client.Start(zap.NewNop())

	// Get a token from the client
	token, err := client.GetToken(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to get token: %v", err.Error()))
	}
	result := make([]string, 0, len(token.Headers))
	for k, v := range token.Headers {
		result = append(result, fmt.Sprintf("%v: %v", k, v))
	}
	os.Stderr.WriteString("Add to CURL:\n-H '")
	os.Stdout.WriteString(strings.Join(result, " "))
	os.Stderr.WriteString("'\n")
}
