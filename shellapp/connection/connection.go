package connection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	apitypes "github.com/srschreiber/nito/api_types"
	"github.com/srschreiber/nito/shellapp/keys"
	wstypes "github.com/srschreiber/nito/websocket_types"
)

type Session struct {
	UserID    string // username (used as the identity token sent to the broker)
	BrokerURL string
	RoomID    *string // currently selected room
}

var (
	mu              sync.Mutex
	wmu             sync.Mutex // serializes all writes to conn
	conn            *websocket.Conn
	session         *Session
	notifChan       chan []byte // server-push notification text
	echoChan        chan []byte // echo messages from the server (for testing connectivity)
	roomMessageChan chan []byte // incoming room messages (raw JSON for the TUI model to dispatch
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
func Register(brokerURL, username, publicKey string) (*apitypes.RegisterResponse, error) {
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
	var result apitypes.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("register: decode response: %w", err)
	}
	return &result, nil
}

// Connect establishes a persistent WebSocket connection to the broker.
func Connect(brokerURL, userID string) error {
	mu.Lock()
	defer mu.Unlock()

	if conn != nil {
		conn.Close()
		conn = nil
		session = nil
	}

	brokerURL = normalizeURL(brokerURL)
	sig, err := keys.Sign(userID + ":/ws")
	if err != nil {
		return fmt.Errorf("sign handshake: %w", err)
	}
	headers := http.Header{}
	headers.Set("X-Username", userID)
	headers.Set("X-Signature", sig)
	dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	c, _, err := dialer.Dial("ws://"+brokerURL+"/ws?user_id="+userID, headers)
	if err != nil {
		return err
	}

	// WriteControl is safe to call concurrently with WriteMessage, so no wmu needed.
	c.SetPingHandler(func(data string) error {
		return c.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})

	roomMessageChan = make(chan []byte, 16)
	nc := make(chan []byte, 16)
	echoChan = make(chan []byte, 16)
	conn = c
	session = &Session{UserID: userID, BrokerURL: brokerURL}
	notifChan = nc

	go readLoop(c, echoChan, roomMessageChan, nc)
	return nil
}

// readLoop runs in the background, routing messages:
//   - "notification" RPCs → nc (notification text)
//   - everything else → ic (raw JSON for the TUI model to dispatch)
func readLoop(c *websocket.Conn, echoChan, roomMessageChan, nc chan []byte) {
	defer func() {
		mu.Lock()
		if conn == c {
			conn = nil
			session = nil
		}
		mu.Unlock()
		close(roomMessageChan)
		close(nc)
	}()
	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var message wstypes.ToClientWsMessage
		if json.Unmarshal(data, &message) != nil {
			log.Println("unmarshal WS message:", err)
			continue
		}

		switch message.RPCName {
		case "notification":
			var notificationPayload wstypes.NotificationPayload
			if json.Unmarshal(data, &notificationPayload) != nil {
				log.Println("unmarshal notification payload:", err)
				continue
			}
			nc <- message.Payload
			continue
		case "echo":
			var echoPayload wstypes.EchoPayload
			if json.Unmarshal(message.Payload, &echoPayload) != nil {
				log.Printf("Echo from server: %s", echoPayload.Text)
				continue
			}
			echoChan <- message.Payload
			continue
		case "room_message":
			var roomMessagePayload wstypes.RoomMessagePayload
			if json.Unmarshal(message.Payload, &roomMessagePayload) != nil {
				log.Printf("Message from %s in room %s: %s", roomMessagePayload.FromUserID, roomMessagePayload.RoomID, roomMessagePayload.EncryptedText)
				continue
			}
			roomMessageChan <- message.Payload
			continue
		}
	}
}

// NotifChan returns a receive-only channel of server-push notification text.
// Returns nil when not connected.
func NotifChan() <-chan []byte {
	mu.Lock()
	defer mu.Unlock()
	return notifChan
}

func Disconnect() {
	mu.Lock()
	defer mu.Unlock()
	if conn != nil {
		conn.Close()
		conn = nil
	}
	session = nil
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

func EchoChan() <-chan []byte {
	mu.Lock()
	defer mu.Unlock()
	return echoChan
}

func RoomMessageChan() <-chan []byte {
	mu.Lock()
	defer mu.Unlock()
	return roomMessageChan
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
func ListRooms() ([]apitypes.RoomEntry, error) {
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
		Rooms []apitypes.RoomEntry `json:"rooms"`
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

// SetCurrentRoom stores the selected room ID in the session.
func SetCurrentRoom(roomID string) {
	mu.Lock()
	defer mu.Unlock()
	if session != nil {
		session.RoomID = &roomID
	}
}

// GetCurrentRoomID returns the currently selected room ID, or nil if none selected.
func GetCurrentRoomID() *string {
	mu.Lock()
	defer mu.Unlock()
	if session == nil {
		return nil
	}
	return session.RoomID
}

// GetUserPublicKey fetches the public key PEM for a given username from the broker.
func GetUserPublicKey(username string) (string, error) {
	s := CurrentSession()
	if s == nil {
		return "", errors.New("not connected")
	}
	resp, err := http.Get("http://" + s.BrokerURL + "/api/v0/users/public-key?username=" + username)
	if err != nil {
		return "", fmt.Errorf("get public key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get public key: broker returned %s", resp.Status)
	}
	var result struct {
		PublicKey string `json:"publicKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("get public key: decode: %w", err)
	}
	return result.PublicKey, nil
}

// GetMyRoomKey fetches the caller's encrypted room key for the given room.
func GetMyRoomKey(roomID string) (string, error) {
	s := CurrentSession()
	if s == nil {
		return "", errors.New("not connected")
	}
	resp, err := signedGet(
		"http://"+s.BrokerURL+"/api/v0/rooms/key?room_id="+roomID,
		s.UserID,
		"/api/v0/rooms/key",
	)
	if err != nil {
		return "", fmt.Errorf("get room key: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get room key: broker returned %s", resp.Status)
	}
	var result struct {
		EncryptedRoomKey string `json:"encryptedRoomKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("get room key: decode: %w", err)
	}
	return result.EncryptedRoomKey, nil
}

// InviteUser invites a user to a room, sending their encrypted copy of the room key.
func InviteUser(roomID, invitedUsername, encryptedRoomKey string) error {
	s := CurrentSession()
	if s == nil {
		return errors.New("not connected")
	}
	body, _ := json.Marshal(map[string]string{
		"roomId":           roomID,
		"invitedUsername":  invitedUsername,
		"encryptedRoomKey": encryptedRoomKey,
	})
	resp, err := signedPost("http://"+s.BrokerURL+"/api/v0/rooms/invite", s.UserID, "/api/v0/rooms/invite", body)
	if err != nil {
		return fmt.Errorf("invite user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invite user: broker returned %s", resp.Status)
	}
	return nil
}

// ListRoomMembers returns the joined members of a room.
func ListRoomMembers(roomID string) ([]apitypes.RoomMemberEntry, error) {
	s := CurrentSession()
	if s == nil {
		return nil, errors.New("not connected")
	}
	resp, err := signedGet(
		"http://"+s.BrokerURL+"/api/v0/rooms/members?room_id="+roomID,
		s.UserID,
		"/api/v0/rooms/members",
	)
	if err != nil {
		return nil, fmt.Errorf("list room members: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list room members: broker returned %s", resp.Status)
	}
	var result struct {
		Members []apitypes.RoomMemberEntry `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("list room members: decode: %w", err)
	}
	return result.Members, nil
}

// ListPendingInvites returns rooms the current user has been invited to but not yet joined.
func ListPendingInvites() ([]apitypes.PendingInvite, error) {
	s := CurrentSession()
	if s == nil {
		return nil, errors.New("not connected")
	}
	resp, err := signedGet(
		"http://"+s.BrokerURL+"/api/v0/rooms/invites?user_id="+s.UserID,
		s.UserID,
		"/api/v0/rooms/invites",
	)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list invites: broker returned %s", resp.Status)
	}
	var result struct {
		Invites []apitypes.PendingInvite `json:"invites"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("list invites: decode: %w", err)
	}
	return result.Invites, nil
}

// AcceptInvite accepts a pending room invitation.
func AcceptInvite(roomID string) error {
	s := CurrentSession()
	if s == nil {
		return errors.New("not connected")
	}
	body, _ := json.Marshal(map[string]string{"roomId": roomID})
	resp, err := signedPost("http://"+s.BrokerURL+"/api/v0/rooms/invites/accept", s.UserID, "/api/v0/rooms/invites/accept", body)
	if err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("accept invite: broker returned %s", resp.Status)
	}
	return nil
}
