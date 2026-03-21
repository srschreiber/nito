package keys

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/srschreiber/nito/utils"
)

var NonceMap = map[string]map[string]struct{}{}

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

// GenerateMessageEncryptionKey generates a message encryption key derived from the room key and user ID
// using HMAC-SHA256
func GenerateMessageEncryptionKey(roomKey []byte, userID string) []byte {
	hash := hmac.New(sha256.New, roomKey)
	hash.Write([]byte(userID))
	return hash.Sum(nil)
}

// EncryptMessageWithRoomKey encrypts the message with the room key
// The function looks like
// key_message1 = HMAC(roomKey, userID || userMessageCount)
// key_message2 = HMAC(key_message1, userID || userMessageCount)
// key_message3 = HMAC(key_message2, userID || userMessageCount)
// it is on a per-user basis to avoid race conditions that can affect message ordering
// once the key is obtained, ChaCha20-Poly1305 can be used to encrypt the message with the derived key
// Apparently, ChaCha20-Poly1305 is fast for software-only environments with short messages
// This can be used as a util for a ratchet scheme if userMessageCount is set and increments, using the
// output key as the new room key for the next message.
// This way, even if a key is compromised, only messages encrypted with that key are at risk, and future messages remain secure.
func EncryptMessageWithRoomKey(message []byte, userID string, roomKey []byte, userMessageCount *int) ([]byte, error) {
	encKey := GenerateMessageEncryptionKey(roomKey, fmt.Sprintf("%s%d", userID, utils.DerefOrZero(userMessageCount)))
	aead, err := chacha20poly1305.New(encKey)
	if err != nil {
		return nil, fmt.Errorf("create AEAD cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, message, nil)
	return append(nonce, ciphertext...), nil
}

func DecryptMessageWithRoomKey(message []byte, userID string, roomKey []byte, userMessageCount *int) ([]byte, error) {
	encKey := GenerateMessageEncryptionKey(roomKey, fmt.Sprintf("%s%d", userID, utils.DerefOrZero(userMessageCount)))
	aead, err := chacha20poly1305.New(encKey)
	if err != nil {
		return nil, fmt.Errorf("create AEAD cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(message) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := message[:nonceSize], message[nonceSize:]
	userNonces, ok := NonceMap[userID]
	if !ok {
		userNonces = make(map[string]struct{})
		NonceMap[userID] = userNonces
	}
	if _, exists := userNonces[string(nonce)]; exists {
		return nil, fmt.Errorf("nonce reuse detected for user %s", userID)
	}
	userNonces[string(nonce)] = struct{}{}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt message: %w", err)
	}
	return plaintext, nil
}
