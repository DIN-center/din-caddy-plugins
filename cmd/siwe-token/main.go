package main

import (
	"fmt"
	"github.com/openrelayxyz/din-caddy-plugins/lib/auth/siwe"
	"os"
	"io/ioutil"
	"strings"
	"encoding/hex"
)


func main() {
	url := os.Args[1]
	keyFile := os.Args[2]
	

	hexKeyBytes, err := ioutil.ReadFile(keyFile)
	if err != nil {
		panic(fmt.Sprintf("Failed to read secrets file: %v", err.Error()))
	}
	hexKey := string(hexKeyBytes)
	hexKey = strings.TrimSpace(strings.TrimPrefix(hexKey, "0x"))
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		panic(err)
	}
	signer := &siwe.SigningConfig{
		PrivateKey: key,
		SignerURL: "http://localhost",
	}
	signer.GenPrivKey()
	os.Stderr.WriteString("Your signing address: ")
	os.Stderr.WriteString(signer.Address)
	os.Stderr.WriteString("\n")

	client := siwe.NewSIWEClient(url, 0, signer)
	client.Start(nil)
	token, err := client.GetToken(nil)
	if err != nil {
		panic(err)
	}
	result := make([]string, 0, len(token.Headers))
	for k, v := range token.Headers {
		result = append(result, fmt.Sprintf("%v: %v", k, v))
	}
	os.Stderr.WriteString("Add to CURL:\n-H '")
	os.Stdout.WriteString(strings.Join(result, " "))
	os.Stderr.WriteString("'\n")
}