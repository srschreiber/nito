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

var (
	mu      sync.Mutex
	conn    *websocket.Conn
	session *Session
)

func normalizeURL(url string) string {
	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	return url
}

// Register calls POST /api/v0/register on the broker to add the user to the
// in-memory store. This must be called before Connect.
func Register(brokerURL, userID string) error {
	brokerURL = normalizeURL(brokerURL)
	body, _ := json.Marshal(map[string]string{"userId": userID})
	resp, err := http.Post("http://"+brokerURL+"/api/v0/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register: broker returned %s", resp.Status)
	}
	return nil
}

// Connect establishes a persistent WebSocket connection to the broker.
// The broker validates that userID was previously registered.
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

	conn = c
	session = &Session{UserID: userID, BrokerURL: brokerURL}
	return nil
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
	return conn.WriteMessage(websocket.TextMessage, data)
}

// Receive reads one message from the active WebSocket connection.
func Receive(timeout time.Duration) ([]byte, error) {
	mu.Lock()
	c := conn
	mu.Unlock()
	if c == nil {
		return nil, errors.New("not connected")
	}
	_ = c.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := c.ReadMessage()
	_ = c.SetReadDeadline(time.Time{})
	return data, err
}

func BrokerURL() string {
	mu.Lock()
	defer mu.Unlock()
	if session == nil {
		return ""
	}
	return session.BrokerURL
}
