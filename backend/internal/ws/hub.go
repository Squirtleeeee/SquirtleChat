package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"squirtlechat/internal/service"
	"squirtlechat/pkg/apperr"
	"squirtlechat/pkg/auth"
	"squirtlechat/pkg/routing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Frame struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[string]*Client
	auth    *service.AuthService
	msg     MessageHandler
	router  *routing.Router
}

type MessageHandler interface {
	HandleSend(userID int64, deviceID string, raw json.RawMessage) ([]byte, error)
	HandleTyping(userID int64, deviceID string, raw json.RawMessage) error
}

func NewHub(authSvc *service.AuthService, msg MessageHandler, router *routing.Router) *Hub {
	return &Hub{
		clients: make(map[int64]map[string]*Client),
		auth:    authSvc,
		msg:     msg,
		router:  router,
	}
}

func (h *Hub) RegisterRoutes(r *gin.Engine) {
	r.GET("/ws", h.serveWS)
}

func (h *Hub) serveWS(c *gin.Context) {
	token := c.Query("token")
	deviceID := c.Query("device_id")
	if token == "" {
		c.JSON(401, gin.H{"error": "请先登录后再连接"})
		return
	}
	if deviceID == "" {
		deviceID = "default"
	}
	claims, err := h.auth.ParseToken(token)
	if err != nil {
		c.JSON(401, gin.H{"error": "登录状态无效，请重新登录"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	client := &Client{
		hub:      h,
		conn:     conn,
		userID:   claims.UserID,
		deviceID: deviceID,
		send:     make(chan []byte, 256),
	}
	h.register(client)
	go client.writePump()
	go client.readPump()
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = make(map[string]*Client)
	}
	if old, ok := h.clients[c.userID][c.deviceID]; ok {
		old.close()
	}
	h.clients[c.userID][c.deviceID] = c
	h.mu.Unlock()
	if h.router != nil {
		_ = h.router.Register(context.Background(), c.userID, c.deviceID)
	}
	if h.auth != nil {
		h.auth.TouchDevice(context.Background(), c.userID, c.deviceID, "")
	}
	log.Printf("ws connected user=%d device=%s", c.userID, c.deviceID)
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.clients[c.userID]; ok {
		if cur, ok := m[c.deviceID]; ok && cur == c {
			delete(m, c.deviceID)
		}
		if len(m) == 0 {
			delete(h.clients, c.userID)
		}
	}
	if h.router != nil {
		_ = h.router.Unregister(context.Background(), c.userID, c.deviceID)
	}
	log.Printf("ws disconnected user=%d device=%s", c.userID, c.deviceID)
}

func (h *Hub) HasUser(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

func (h *Hub) PushToUser(userID int64, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients[userID] {
		h.send(c, data)
	}
}

func (h *Hub) PushToUserExceptDevice(userID int64, exceptDevice string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for did, c := range h.clients[userID] {
		if did == exceptDevice {
			continue
		}
		h.send(c, data)
	}
}

func (h *Hub) PushToDevice(userID int64, deviceID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c, ok := h.clients[userID][deviceID]; ok {
		h.send(c, data)
	}
}

// KickDevice notifies and closes a specific device connection.
func (h *Hub) KickDevice(userID int64, deviceID string) {
	payload, _ := json.Marshal(map[string]interface{}{
		"type": "kick",
		"payload": map[string]string{
			"reason": "device_revoked",
		},
	})
	h.mu.Lock()
	c, ok := h.clients[userID][deviceID]
	h.mu.Unlock()
	if !ok {
		return
	}
	h.send(c, payload)
	go func() {
		time.Sleep(200 * time.Millisecond)
		c.close()
	}()
}

func (h *Hub) send(c *Client, data []byte) {
	select {
	case c.send <- data:
	default:
		log.Printf("ws drop user=%d device=%s", c.userID, c.deviceID)
	}
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	userID   int64
	deviceID string
	send     chan []byte
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.close()
	}()
	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var frame Frame
		if err := json.Unmarshal(data, &frame); err != nil {
			continue
		}
		switch frame.Type {
		case "ping":
			if c.hub.router != nil {
				_ = c.hub.router.Refresh(context.Background(), c.userID, c.deviceID)
			}
			c.sendJSON(Frame{Type: "pong"})
		case "message":
			if c.hub.msg != nil {
				out, err := c.hub.msg.HandleSend(c.userID, c.deviceID, frame.Payload)
				if err != nil {
					msg := apperr.ToUserMessage(apperr.ErrInvalidParam, err)
					c.sendJSON(Frame{Type: "error", Payload: mustRaw(msg)})
					continue
				}
				if out != nil {
					c.sendJSON(Frame{Type: "ack", Payload: out})
				}
			}
		case "typing":
			if c.hub.msg != nil {
				if err := c.hub.msg.HandleTyping(c.userID, c.deviceID, frame.Payload); err != nil {
					// Typing is best-effort; ignore validation noise.
					continue
				}
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if c.hub.router != nil {
				_ = c.hub.router.Refresh(context.Background(), c.userID, c.deviceID)
			}
			c.sendJSON(Frame{Type: "ping"})
		}
	}
}

func (c *Client) sendJSON(v interface{}) {
	b, _ := json.Marshal(v)
	select {
	case c.send <- b:
	default:
	}
}

func (c *Client) close() {
	_ = c.conn.Close()
}

func mustRaw(s string) json.RawMessage {
	b, _ := json.Marshal(gin.H{"msg": s})
	return b
}

var _ = (*auth.Claims)(nil)
