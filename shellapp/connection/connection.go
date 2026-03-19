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
	"github.com/srschreiber/nito/shellapp/keys"
	shelltypes "github.com/srschreiber/nito/shellapp/types"
)

type Session struct {
	UserID    string // username (used as the identity token sent to the broker)
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

// signedPost builds a POST request with X-Username and X-Signature headers.
// apiPath is the bare path (e.g. "/api/v0/rooms") used as the signature payload.
func signedPost(url, username, apiPath string, body []byte) (*http.Response, error) {
	sig, err := keys.Sign(username + ":" + apiPath)
	if err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Username", username)
	req.Header.Set("X-Signature", sig)
	return http.DefaultClient.Do(req)
}

// signedGet builds a GET request with X-Username and X-Signature headers.
func signedGet(url, username, apiPath string) (*http.Response, error) {
	sig, err := keys.Sign(username + ":" + apiPath)
	if err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Username", username)
	req.Header.Set("X-Signature", sig)
	return http.DefaultClient.Do(req)
}

// CreateRoom creates a new room on the broker. Requires an active session.
// encryptedRoomKey is the base64-encoded RSA-OAEP ciphertext of the room's AES key.
func CreateRoom(name, encryptedRoomKey string) (id, roomName string, err error) {
	s := CurrentSession()
	if s == nil {
		return "", "", errors.New("not connected")
	}
	body, _ := json.Marshal(map[string]string{
		"name":             name,
		"userId":           s.UserID,
		"encryptedRoomKey": encryptedRoomKey,
	})
	resp, err := signedPost("http://"+s.BrokerURL+"/api/v0/rooms", s.UserID, "/api/v0/rooms", body)
	if err != nil {
		return "", "", fmt.Errorf("create room: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("create room: broker returned %s", resp.Status)
	}
	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("create room: decode: %w", err)
	}
	return result.ID, result.Name, nil
}

// ListRooms returns all rooms the current user is a member of.
func ListRooms() ([]shelltypes.RoomEntry, error) {
	s := CurrentSession()
	if s == nil {
		return nil, errors.New("not connected")
	}
	resp, err := signedGet(
		"http://"+s.BrokerURL+"/api/v0/rooms/list?user_id="+s.UserID,
		s.UserID,
		"/api/v0/rooms/list",
	)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list rooms: broker returned %s", resp.Status)
	}
	var result struct {
		Rooms []shelltypes.RoomEntry `json:"rooms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("list rooms: decode: %w", err)
	}
	return result.Rooms, nil
}

func BrokerURL() string {
	mu.Lock()
	defer mu.Unlock()
	if session == nil {
		return ""
	}
	return session.BrokerURL
}
