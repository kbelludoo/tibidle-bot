package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"tibidle-bot/pkg/protocol"

	"github.com/gorilla/websocket"
)

type SocketClient struct {
	url      string
	ticket   string
	conn     *websocket.Conn
	mu       sync.Mutex
	isClosed bool

	onMessage func(msg protocol.InboundMessage)
	onClose   func(err error)
}

func NewSocketClient(url, ticket string, onMsg func(protocol.InboundMessage), onClose func(error)) *SocketClient {
	return &SocketClient{
		url:       url,
		ticket:    ticket,
		onMessage: onMsg,
		onClose:   onClose,
	}
}

// Connect dials the game WebSocket and sends auth
func (s *SocketClient) Connect() error {
	headers := make(http.Header)
	headers.Set("Origin", "https://play.tibidle.com")
	headers.Set("User-Agent", UserAgent)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}

	conn, _, err := dialer.Dial(s.url, headers)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	s.conn = conn
	s.isClosed = false

	// Send Auth message immediately
	authPayload := protocol.OutboundMessage{
		Type: "auth",
		Data: protocol.AuthPayload{
			Ticket:           s.ticket,
			TradeFlowVersion: 2,
		},
	}
	if err := s.SendRaw(authPayload); err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to send auth: %w", err)
	}

	go s.readLoop()
	go s.keepaliveLoop()

	return nil
}

// Send sends a typed message over the socket
func (s *SocketClient) Send(msgType string, data any) error {
	return s.SendRaw(protocol.OutboundMessage{
		Type: msgType,
		Data: data,
	})
}

// SendRaw writes JSON message thread-safely
func (s *SocketClient) SendRaw(msg any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isClosed || s.conn == nil {
		return fmt.Errorf("socket is closed")
	}

	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.conn.WriteMessage(websocket.TextMessage, b)
}

func (s *SocketClient) readLoop() {
	defer func() {
		s.Close()
		if s.onClose != nil {
			s.onClose(nil)
		}
	}()

	for {
		_, payload, err := s.conn.ReadMessage()
		if err != nil {
			return
		}

		var inMsg protocol.InboundMessage
		if err := json.Unmarshal(payload, &inMsg); err != nil {
			continue
		}

		if s.onMessage != nil {
			s.onMessage(inMsg)
		}
	}
}

func (s *SocketClient) keepaliveLoop() {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		closed := s.isClosed
		s.mu.Unlock()
		if closed {
			return
		}

		// Keepalive packet & snapshot request to guarantee anti-idle
		_ = s.Send("keepalive", map[string]any{})
		_ = s.Send("party_get_snapshot", map[string]any{})
	}
}

// Close closes the WebSocket
func (s *SocketClient) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isClosed {
		s.isClosed = true
		if s.conn != nil {
			_ = s.conn.Close()
		}
		log.Println("[WebSocket] Connection closed.")
	}
}
