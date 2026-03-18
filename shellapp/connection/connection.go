package connection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	UserID    string
	BrokerURL string
}

type RegisterResponse struct {
	ID                string `json:"id"`
	Username          string `json:"username"`
	AlreadyRegistered bool   `json:"alreadyRegistered"`
}

var (
	mu      sync.Mutex
	wmu     sync.Mutex // serializes all writes to conn
	conn    *websocket.Conn
	session *Session
	msgChan chan []byte
)

func normalizeURL(url string) string {
	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	return url
}

// Register sends username and public key to the broker, creating a DB entry if the user
// doesn't exist yet. Returns the user's ID and whether they were already registered.
func Register(brokerURL, username, publicKey string) (*RegisterResponse, error) {
	brokerURL = normalizeURL(brokerURL)
	body, _ := json.Marshal(map[string]string{
		"username":  username,
		"publicKey": publicKey,
	})
	resp, err := http.Post("http://"+brokerURL+"/api/v0/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("register: broker returned %s", resp.Status)
	}
	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("register: decode response: %w", err)
	}
	return &result, nil
}

// Connect establishes a persistent WebSocket connection to the broker.
// A background goroutine reads all incoming frames and cleans up on error.
func Connect(brokerURL, userID string) error {
	mu.Lock()
	defer mu.Unlock()

	if conn != nil {
		conn.Close()
		conn = nil
		session = nil
	}

	brokerURL = normalizeURL(brokerURL)
	dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	c, _, err := dialer.Dial("ws://"+brokerURL+"/ws?user_id="+userID, nil)
	if err != nil {
		return err
	}

	c.SetPingHandler(func(data string) error {
		wmu.Lock()
		defer wmu.Unlock()
		return c.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})

	ch := make(chan []byte, 16)
	conn = c
	session = &Session{UserID: userID, BrokerURL: brokerURL}
	msgChan = ch

	go readLoop(c, ch)
	return nil
}

// readLoop runs in the background, feeding messages into ch and cleaning up
// when the connection dies (ping timeout, network error, etc.).
func readLoop(c *websocket.Conn, ch chan []byte) {
	defer func() {
		mu.Lock()
		if conn == c {
			conn = nil
			session = nil
		}
		mu.Unlock()
		close(ch)
	}()
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		ch <- data
	}
}

func Disconnect() {
	mu.Lock()
	defer mu.Unlock()
	if conn != nil {
		conn.Close()
		conn = nil
		session = nil
	}
}

func IsConnected() bool {
	mu.Lock()
	defer mu.Unlock()
	return conn != nil
}

func CurrentSession() *Session {
	mu.Lock()
	defer mu.Unlock()
	return session
}

// Send writes a JSON-encoded message to the active WebSocket connection.
func Send(data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	if conn == nil {
		return errors.New("not connected")
	}
	wmu.Lock()
	defer wmu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

// Receive reads one application message from the background reader.
func Receive(timeout time.Duration) ([]byte, error) {
	mu.Lock()
	ch := msgChan
	mu.Unlock()
	if ch == nil {
		return nil, errors.New("not connected")
	}
	select {
	case data, ok := <-ch:
		if !ok {
			return nil, errors.New("disconnected")
		}
		return data, nil
	case <-time.After(timeout):
		return nil, errors.New("receive timeout")
	}
}

func BrokerURL() string {
	mu.Lock()
	defer mu.Unlock()
	if session == nil {
		return ""
	}
	return session.BrokerURL
}
