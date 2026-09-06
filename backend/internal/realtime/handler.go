package realtime

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// TokenVerifier valida o access token passado como query param (o browser não
// consegue mandar header Authorization no handshake de WebSocket).
type TokenVerifier func(token string) (userID string, ok bool)

type Handler struct {
	hub    *Hub
	verify TokenVerifier
	up     websocket.Upgrader
}

func NewHandler(hub *Hub, verify TokenVerifier, allowedOrigins []string) *Handler {
	origins := map[string]bool{}
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return &Handler{
		hub:    hub,
		verify: verify,
		up: websocket.Upgrader{
			HandshakeTimeout: 5 * time.Second,
			CheckOrigin: func(r *http.Request) bool {
				o := r.Header.Get("Origin")
				return o == "" || origins[o] // apps nativos não mandam Origin
			},
		},
	}
}

func (h *Handler) Register(g *echo.Group) {
	g.GET("/ws", h.serve)
}

func (h *Handler) serve(c echo.Context) error {
	userID, ok := h.verify(c.QueryParam("token"))
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "token inválido")
	}

	conn, err := h.up.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return nil // upgrade já respondeu
	}

	cl := &client{send: make(chan []byte, 16)}
	h.hub.add(userID, cl)
	defer func() {
		h.hub.remove(userID, cl)
		_ = conn.Close()
	}()

	// leitura: só drena pings/close do cliente e aplica read deadline.
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				close(cl.send)
				return
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"hello"}`))
	for {
		select {
		case msg, ok := <-cl.send:
			if !ok {
				return nil
			}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return nil
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return nil
			}
		}
	}
}
