package keys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const keyDir = ".nito"

func keyPaths() (privPath, pubPath string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("get working dir: %w", err)
	}
	dir := filepath.Join(cwd, keyDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", fmt.Errorf("create key dir: %w", err)
	}
	return filepath.Join(dir, "private_key.pem"), filepath.Join(dir, "public_key.pem"), nil
}

// LoadOrGenerate loads the RSA-2048 key pair from .nito/ in the working directory,
// generating and saving them if they don't exist yet.
// Returns the public key as a PEM string ready to send to the broker.
func LoadOrGenerate() (pub string, err error) {
	privPath, pubPath, err := keyPaths()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(privPath); err == nil {
		// Keys already exist — load them.
		pubPEM, err := os.ReadFile(pubPath)
		if err != nil {
			return "", fmt.Errorf("read public key: %w", err)
		}
		return string(pubPEM), nil
	}

	// Generate new key pair.
	fmt.Printf("generating RSA-2048 key pair in %s/\n", filepath.Join(keyDir))
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return "", fmt.Errorf("save private key: %w", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
		return "", fmt.Errorf("save public key: %w", err)
	}

	cwd, _ := os.Getwd()
	fmt.Printf("keys saved to %s\n", filepath.Join(cwd, keyDir))
	return string(pubPEM), nil
}
