package connection

import (
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	mu      sync.Mutex
	conn    *websocket.Conn
	baseURL string
)

func Connect(url string) error {
	mu.Lock()
	defer mu.Unlock()

	if conn != nil {
		conn.Close()
		conn = nil
	}

	url = strings.TrimPrefix(url, "ws://")
	url = strings.TrimPrefix(url, "wss://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	dialer := &websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	c, _, err := dialer.Dial("ws://"+url+"/ws", nil)
	if err != nil {
		return err
	}

	conn = c
	baseURL = url
	return nil
}

func Disconnect() {
	mu.Lock()
	defer mu.Unlock()

	if conn != nil {
		conn.Close()
		conn = nil
		baseURL = ""
	}
}

func IsConnected() bool {
	mu.Lock()
	defer mu.Unlock()
	return conn != nil
}

func BrokerURL() string {
	mu.Lock()
	defer mu.Unlock()
	return baseURL
}
