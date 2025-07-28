package worker

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketClient struct {
	url    string
	token  string
	conn   *websocket.Conn
	logger *slog.Logger
}

type Signal struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

type Command struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

func NewWebSocketClient(serverURL, token string, logger *slog.Logger) *WebSocketClient {
	return &WebSocketClient{
		url:    serverURL,
		token:  token,
		logger: logger,
	}
}

func (c *WebSocketClient) Connect(ctx context.Context) error {
	u, err := url.Parse(c.url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// TODO: Security enhancement tracked in GitHub issue #20: move token to Authorization header
	q := u.Query()
	q.Set("token", c.token)
	u.RawQuery = q.Encode()

	c.logger.Debug("Connecting to WebSocket", slog.String("url", u.String()))

	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn
	c.logger.Info("WebSocket connected", slog.String("url", c.url))
	return nil
}

func (c *WebSocketClient) ReadSignal(ctx context.Context) (*Signal, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	var signal Signal
	err := c.conn.ReadJSON(&signal)
	if err != nil {
		return nil, fmt.Errorf("failed to read signal: %w", err)
	}

	c.logger.Debug("Received signal", slog.String("type", signal.Type))
	return &signal, nil
}

func (c *WebSocketClient) WriteCommand(ctx context.Context, cmd *Command) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	c.logger.Debug("Sending command", slog.String("type", cmd.Type))
	
	err := c.conn.WriteJSON(cmd)
	if err != nil {
		return fmt.Errorf("failed to write command: %w", err)
	}

	return nil
}

func (c *WebSocketClient) Close() error {
	if c.conn == nil {
		return nil
	}

	c.logger.Info("Closing WebSocket connection")
	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *WebSocketClient) Ping(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	return c.conn.WriteMessage(websocket.PingMessage, nil)
}