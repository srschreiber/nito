package keys

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
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
	// if shellapp not at end of path, append it (e.g. if user runs from project root instead of shellapp/)
	if filepath.Base(cwd) != "shellapp" {
		cwd = filepath.Join(cwd, "shellapp")
	}
	dir := filepath.Join(cwd, keyDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", "", fmt.Errorf("create key dir: %w", err)
	}
	return filepath.Join(dir, "private_key.pem"), filepath.Join(dir, "public_key.pem"), nil
}

// HaveKeys returns true if a local key pair exists (i.e. the user has registered).
func HaveKeys() bool {
	privPath, _, err := keyPaths()
	if err != nil {
		return false
	}
	_, err = os.Stat(privPath)
	return err == nil
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

// Sign signs message with the local RSA private key using PSS-SHA256 and returns
// the base64-encoded signature. Used to authenticate API and RPC requests.
func Sign(message string) (string, error) {
	privPath, _, err := keyPaths()
	if err != nil {
		return "", err
	}
	privPEM, err := os.ReadFile(privPath)
	if err != nil {
		return "", fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return "", fmt.Errorf("decode private key PEM: no block found")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("expected RSA private key")
	}
	h := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPSS(rand.Reader, rsaKey, crypto.SHA256, h[:], nil)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// DecryptRoomKey decrypts a base64-encoded RSA-OAEP ciphertext using the local private key.
func DecryptRoomKey(encryptedKeyB64 string) ([]byte, error) {
	privPath, _, err := keyPaths()
	if err != nil {
		return nil, err
	}
	privPEM, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, fmt.Errorf("decode private key PEM: no block found")
	}
	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	rsaKey, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("expected RSA private key")
	}
	ct, err := base64.StdEncoding.DecodeString(encryptedKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	roomKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, rsaKey, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt room key: %w", err)
	}
	return roomKey, nil
}

// EncryptRoomKeyForPEM encrypts roomKey with the given RSA public key PEM using OAEP-SHA256.
func EncryptRoomKeyForPEM(roomKey []byte, publicKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", fmt.Errorf("decode public key PEM: no block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("expected RSA public key")
	}
	ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, roomKey, nil)
	if err != nil {
		return "", fmt.Errorf("encrypt room key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// GenerateRoomKey generates a random 32-byte key suitable for AES-256-GCM,
// which provides authenticated encryption fast enough for real-time use (e.g. VoIP).
func GenerateRoomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate room key: %w", err)
	}
	return key, nil
}

// EncryptRoomKey encrypts roomKey with the RSA-2048 public key on disk using
// OAEP-SHA256, and returns a base64-encoded ciphertext safe to send to the broker.
func EncryptRoomKey(roomKey []byte) (string, error) {
	_, pubPath, err := keyPaths()
	if err != nil {
		return "", err
	}
	pubPEM, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return "", fmt.Errorf("decode public key PEM: no block found")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("expected RSA public key")
	}
	ct, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, roomKey, nil)
	if err != nil {
		return "", fmt.Errorf("encrypt room key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}
