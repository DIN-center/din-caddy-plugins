package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/auth/siwe"
)

func main() {
	// Define flags
	help := flag.Bool("help", false, "Display help text")

	// Parse flags
	flag.Parse()

	if *help || len(flag.Args()) < 2 {
		fmt.Println("Usage: app <SIWE client URL> <Private key file path>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	url := flag.Arg(0)
	privateKeyFile := flag.Arg(1)

	// Read the private key file
	hexKeyBytes, err := os.ReadFile(filepath.Clean(privateKeyFile))
	if err != nil {
		panic(fmt.Sprintf("Failed to read private key file: %v", err.Error()))
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
	if err := (&siwe.SIWESignerClient{}).GenPrivKey(signer); err != nil {
		panic(fmt.Sprintf("Failed to generate private key: %v", err.Error()))
	}

	// Print the address
	if err := errors.Join(
		fWriteString(os.Stderr, "Your signing address: "),
		fWriteString(os.Stderr, signer.Address),
		fWriteString(os.Stderr, "\n"),
	); err != nil {
		panic(fmt.Sprintf("Failed to display signer address on stder: %v", err.Error()))
	}

	// Create a new SIWE client
	client := siwe.NewSIWEClient(url, 0, signer)

	// Start the client
	if err := client.Start(zap.NewNop()); err != nil {
		panic(fmt.Sprintf("Failed to start SIWE client: %v", err.Error()))
	}

	// Get a token from the client
	token, err := client.GetToken(nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to get token: %v", err.Error()))
	}
	result := make([]string, 0, len(token.Headers))
	for k, v := range token.Headers {
		result = append(result, fmt.Sprintf("%v: %v", k, v))
	}

	if err := errors.Join(
		fWriteString(os.Stderr, "Add to CURL:\n-H '"),
		fWriteString(os.Stdout, strings.Join(result, " ")),
		fWriteString(os.Stderr, "'\n"),
	); err != nil {
		panic(fmt.Sprintf("Failed to display cURL headers on stderr: %v", err.Error()))
	}
}

func fWriteString(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))

	return err
}
