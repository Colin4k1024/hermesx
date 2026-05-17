package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Colin4k1024/hermesx/internal/config"
	"github.com/Colin4k1024/hermesx/internal/llm"
	"github.com/Colin4k1024/hermesx/internal/observability"
)

// ---------- Configuration ----------

// MCPServerConfig represents a single MCP server configuration.
type MCPServerConfig struct {
	Command   string            `json:"command" yaml:"command"`
	Args      []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	NoMCP     []string          `json:"no_mcp,omitempty" yaml:"no_mcp,omitempty"`
	Transport string            `json:"transport,omitempty" yaml:"transport,omitempty"` // "stdio" (default) or "sse"
	URL       string            `json:"url,omitempty" yaml:"url,omitempty"`             // for SSE transport
}

// MCPConfig holds the full MCP configuration.
type MCPConfig struct {
	Servers map[string]MCPServerConfig `json:"mcpServers" yaml:"mcpServers"`
}

// LoadMCPConfig loads MCP server configurations from the config directory.
func LoadMCPConfig() (*MCPConfig, error) {
	configPath := filepath.Join(config.HermesHome(), "mcp.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPConfig{Servers: make(map[string]MCPServerConfig)}, nil
		}
		return nil, fmt.Errorf("read MCP config: %w", err)
	}

	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse MCP config: %w", err)
	}

	if cfg.Servers == nil {
		cfg.Servers = make(map[string]MCPServerConfig)
	}

	return &cfg, nil
}

// SaveMCPConfig writes MCP configuration to disk.
func SaveMCPConfig(cfg *MCPConfig) error {
	configPath := filepath.Join(config.HermesHome(), "mcp.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// ---------- JSON-RPC protocol ----------

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------- MCP Client ----------

// MCPClient manages the lifecycle and communication with an MCP server.
type MCPClient struct {
	name            string
	config          MCPServerConfig
	transport       mcpTransport
	mu              sync.Mutex
	connected       bool
	tools           []mcpToolDef
	nextID          atomic.Int64
	samplingHandler *SamplingHandler
}

type mcpToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpTransport interface {
	Connect(ctx context.Context) error
	Send(req jsonRPCRequest) error
	Receive() (*jsonRPCResponse, error)
	Close() error
}

// ---------- Stdio transport ----------

type stdioTransport struct {
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdout          *bufio.Scanner
	env             map[string]string
	onServerRequest func(data []byte) // callback for server-to-client requests
}

func newStdioTransport(cfg MCPServerConfig) *stdioTransport {
	return &stdioTransport{env: cfg.Env}
}

func (t *stdioTransport) Connect(ctx context.Context) error {
	args := make([]string, len(t.cmd.Args)-1)
	copy(args, t.cmd.Args[1:])

	cmd := exec.CommandContext(ctx, t.cmd.Path, args...)
	// Use sandboxed environment: strip sensitive API keys/tokens,
	// then apply user-specified overrides from MCP config.
	cmd.Env = buildSafeEnv(t.env)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start MCP server: %w", err)
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = bufio.NewScanner(stdout)
	t.stdout.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB buffer

	return nil
}

func (t *stdioTransport) Send(req jsonRPCRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	_, err = t.stdin.Write(data)
	return err
}

func (t *stdioTransport) Receive() (*jsonRPCResponse, error) {
	for {
		if !t.stdout.Scan() {
			if err := t.stdout.Err(); err != nil {
				return nil, fmt.Errorf("read response: %w", err)
			}
			return nil, fmt.Errorf("MCP server closed connection")
		}

		line := t.stdout.Text()

		// Check if this is a server-to-client request (e.g. sampling/createMessage).
		var probe struct {
			Method string `json:"method"`
			ID     any    `json:"id"`
		}
		if json.Unmarshal([]byte(line), &probe) == nil && probe.Method != "" && probe.ID != nil {
			// This is a request from the server, not a response. Handle via callback.
			if t.onServerRequest != nil {
				t.onServerRequest([]byte(line))
			} else {
				slog.Debug("stdio: ignoring server request (no handler)", "method", probe.Method)
			}
			continue
		}

		var resp jsonRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			return nil, fmt.Errorf("parse response: %w (line: %s)", err, truncateOutput(line, 200))
		}
		return &resp, nil
	}
}

func (t *stdioTransport) Close() error {
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
	return nil
}

// Old sseTransport removed — replaced by sseTransportV2 in mcp_sse.go.
// sseTransportV2 adds: proper goroutine lifecycle (rule 4), SSE event type
// parsing, notification channel for tools/list_changed, header passthrough.

// ---------- MCP Client methods ----------

// NewMCPClient creates a new MCP client for the given server configuration.
func NewMCPClient(name string, cfg MCPServerConfig) *MCPClient {
	return &MCPClient{
		name:   name,
		config: cfg,
	}
}

// SetSamplingHandler attaches a sampling handler to this client.
// Must be called before Connect.
func (c *MCPClient) SetSamplingHandler(h *SamplingHandler) {
	c.samplingHandler = h
}

// mcpServerRequest is a JSON-RPC request sent by the MCP server to the client.
type mcpServerRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// parseSamplingRequest parses raw JSON into an mcpServerRequest and returns it
// only if the method is sampling/createMessage. Returns nil for other methods.
func parseSamplingRequest(data []byte) *mcpServerRequest {
	var req mcpServerRequest
	if err := json.Unmarshal(data, &req); err != nil {
		slog.Warn("MCP sampling: failed to parse server request", "error", err)
		return nil
	}
	if req.Method != "sampling/createMessage" {
		slog.Debug("MCP server request ignored (not sampling)", "method", req.Method)
		return nil
	}
	return &req
}

// handleSamplingOnStdio handles a sampling request on the stdio transport by
// writing the JSON-RPC response directly to stdin.
func handleSamplingOnStdio(serverName string, handler *SamplingHandler, st *stdioTransport, data []byte) {
	req := parseSamplingRequest(data)
	if req == nil {
		return
	}

	resp := handler.HandleRequest(context.Background(), serverName, req.ID, req.Params)

	respData, err := json.Marshal(resp)
	if err != nil {
		slog.Error("MCP sampling: failed to marshal response", "error", err)
		return
	}

	respData = append(respData, '\n')
	if _, err := st.stdin.Write(respData); err != nil {
		slog.Error("MCP sampling: failed to send response via stdio", "error", err)
	}
}

// handleSamplingOnSSE handles a sampling request received via SSE and POSTs
// the JSON-RPC response back to the MCP server.
func handleSamplingOnSSE(serverName string, handler *SamplingHandler, sseT *sseTransportV2, data []byte) {
	req := parseSamplingRequest(data)
	if req == nil {
		return
	}

	resp := handler.HandleRequest(context.Background(), serverName, req.ID, req.Params)

	respData, err := json.Marshal(resp)
	if err != nil {
		slog.Error("MCP sampling SSE: failed to marshal response", "error", err)
		return
	}

	httpReq, err := http.NewRequestWithContext(sseT.ctx, "POST", sseT.postURL, bytes.NewReader(respData))
	if err != nil {
		slog.Error("MCP sampling SSE: failed to create POST request", "error", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range sseT.headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := sseT.httpClient.Do(httpReq)
	if err != nil {
		slog.Error("MCP sampling SSE: failed to POST response", "error", err)
		return
	}
	httpResp.Body.Close()
}

// Connect establishes a connection to the MCP server and performs initialization.
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	transport := c.config.Transport
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		st := newStdioTransport(c.config)
		cmd := exec.Command(c.config.Command, c.config.Args...)
		st.cmd = cmd
		// Wire sampling handler for server-to-client requests on stdio.
		if c.samplingHandler != nil {
			serverName := c.name
			handler := c.samplingHandler
			st.onServerRequest = func(data []byte) {
				handleSamplingOnStdio(serverName, handler, st, data)
			}
		}
		c.transport = st
	case "sse":
		if c.config.URL == "" {
			return fmt.Errorf("sse transport requires a 'url' field")
		}
		headers := make(map[string]string)
		for k, v := range c.config.Env {
			if strings.HasPrefix(strings.ToUpper(k), "HEADER_") {
				headers[strings.TrimPrefix(k, "HEADER_")] = v
			}
		}
		sseT := newSSETransportV2(c.config.URL, headers)
		// Wire sampling handler for server-to-client requests on SSE.
		if c.samplingHandler != nil {
			serverName := c.name
			handler := c.samplingHandler
			sseT.onServerRequest = func(data []byte) {
				handleSamplingOnSSE(serverName, handler, sseT, data)
			}
		}
		c.transport = sseT
	default:
		return fmt.Errorf("unknown MCP transport: %s", transport)
	}

	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("connect transport: %w", err)
	}

	// Send initialize
	id := c.nextID.Add(1)
	initReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"sampling": map[string]any{}},
			"clientInfo": map[string]any{
				"name":    "hermesx",
				"version": "1.0.0",
			},
		},
	}

	if err := c.transport.Send(initReq); err != nil {
		c.transport.Close()
		return fmt.Errorf("send initialize: %w", err)
	}

	resp, err := c.transport.Receive()
	if err != nil {
		c.transport.Close()
		return fmt.Errorf("receive initialize response: %w", err)
	}

	if resp.Error != nil {
		c.transport.Close()
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	slog.Info("MCP server initialized", "name", c.name)

	// Send initialized notification
	notifyReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  "notifications/initialized",
	}
	c.transport.Send(notifyReq)

	c.connected = true
	return nil
}

// DiscoverTools calls tools/list to get available tools from the MCP server.
func (c *MCPClient) DiscoverTools(ctx context.Context) ([]mcpToolDef, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	id := c.nextID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/list",
	}

	if err := c.transport.Send(req); err != nil {
		return nil, fmt.Errorf("send tools/list: %w", err)
	}

	resp, err := c.transport.Receive()
	if err != nil {
		return nil, fmt.Errorf("receive tools/list: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []mcpToolDef `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list result: %w", err)
	}

	c.tools = result.Tools
	slog.Info("MCP tools discovered", "server", c.name, "count", len(result.Tools))
	return result.Tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]any) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return "", fmt.Errorf("not connected")
	}

	id := c.nextID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	if err := c.transport.Send(req); err != nil {
		return "", fmt.Errorf("send tools/call: %w", err)
	}

	resp, err := c.transport.Receive()
	if err != nil {
		return "", fmt.Errorf("receive tools/call: %w", err)
	}

	if resp.Error != nil {
		return "", fmt.Errorf("tools/call error: %s", resp.Error.Message)
	}

	// Parse the MCP tool result
	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &callResult); err != nil {
		// Return raw result if we can't parse
		return string(resp.Result), nil
	}

	// Combine text content
	var texts []string
	for _, c := range callResult.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	result := strings.Join(texts, "\n")
	if callResult.IsError {
		return "", fmt.Errorf("MCP tool error: %s", result)
	}

	return result, nil
}

// Shutdown gracefully shuts down the MCP server connection.
func (c *MCPClient) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	slog.Info("MCP server shutting down", "name", c.name)
	return c.transport.Close()
}

// RefreshTools re-discovers tools from the MCP server and re-registers them.
// Called when a tools/list_changed notification is received.
func (c *MCPClient) RefreshTools(ctx context.Context) error {
	// Call DiscoverTools first (it acquires its own lock).
	tools, err := c.DiscoverTools(ctx)
	if err != nil {
		return fmt.Errorf("refresh tools: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return fmt.Errorf("mcp client not connected")
	}

	// Deregister old tools for this server.
	registry := Registry()
	for _, name := range registry.GetAllToolNames() {
		if registry.GetToolsetForTool(name) == "mcp:"+c.name {
			registry.Deregister(name)
		}
	}

	// Re-register.
	for _, tool := range tools {
		registerMCPTool(c.name, c, tool)
	}

	slog.Info("MCP tools refreshed", "server", c.name, "count", len(tools))
	return nil
}

// startNotificationWatcher starts a goroutine that listens for MCP
// notifications (e.g. tools/list_changed) on the SSE transport.
// The goroutine exits when the context is cancelled.
func (c *MCPClient) startNotificationWatcher(ctx context.Context, wg *sync.WaitGroup) {
	sse, ok := c.transport.(*sseTransportV2)
	if !ok {
		return // stdio transport has no notification channel
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case notif, ok := <-sse.Notifications():
				if !ok {
					return
				}
				switch notif.Method {
				case "notifications/tools/list_changed":
					slog.Info("MCP tools list changed, refreshing", "server", c.name)
					if err := c.RefreshTools(ctx); err != nil {
						slog.Warn("MCP tool refresh failed", "server", c.name, "error", err)
					}
				default:
					slog.Debug("MCP notification", "server", c.name, "method", notif.Method)
				}
			}
		}
	}()
}

// backoffParams controls the exponential backoff used by reconnectWithBackoff.
type backoffParams struct {
	initial time.Duration // first sleep duration
	factor  float64       // multiplier applied after each attempt
	max     time.Duration // upper cap on sleep duration
	maxTry  int           // stop after this many attempts (0 = infinite)
}

// defaultBackoff is the reconnection backoff used by startHealthMonitor.
var defaultBackoff = backoffParams{
	initial: 1 * time.Second,
	factor:  2.0,
	max:     30 * time.Second,
	maxTry:  10,
}

// pingInterval is how often the health monitor sends a JSON-RPC ping.
const pingInterval = 30 * time.Second

// startHealthMonitor launches a goroutine that:
//  1. Sends a periodic JSON-RPC "ping" to keep the connection alive and
//     detect failures early.
//  2. Watches for the SSE stream to close unexpectedly.
//  3. On connection loss, calls reconnectWithBackoff to re-establish the
//     connection and refresh tool definitions.
//
// The goroutine exits when ctx is cancelled.
func (c *MCPClient) startHealthMonitor(ctx context.Context, wg *sync.WaitGroup) {
	sse, ok := c.transport.(*sseTransportV2)
	if !ok {
		return // only SSE transport needs a health monitor
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				// Send a ping; on failure treat it as a connection loss.
				if err := c.sendPing(); err != nil {
					select {
					case <-ctx.Done():
						return
					default:
					}
					slog.Warn("MCP ping failed, reconnecting",
						"server", c.name, "error", err)
					c.reconnectWithBackoff(ctx, defaultBackoff)
					// After reconnect, obtain the new SSE transport.
					c.mu.Lock()
					newSSE, ok := c.transport.(*sseTransportV2)
					c.mu.Unlock()
					if !ok {
						return
					}
					sse = newSSE
					ticker.Reset(pingInterval)
				}

			case <-sse.StreamDone():
				// SSE reader goroutine exited — connection was lost.
				select {
				case <-ctx.Done():
					return
				default:
				}
				slog.Warn("MCP SSE stream closed unexpectedly, reconnecting",
					"server", c.name)
				c.reconnectWithBackoff(ctx, defaultBackoff)
				// Update sse reference after reconnect.
				c.mu.Lock()
				newSSE, ok := c.transport.(*sseTransportV2)
				c.mu.Unlock()
				if !ok {
					return
				}
				sse = newSSE
				ticker.Reset(pingInterval)
			}
		}
	}()
}

// sendPing issues a JSON-RPC "ping" to the MCP server and awaits the response.
// It uses a short timeout so a hung connection is detected quickly.
func (c *MCPClient) sendPing() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	id := c.nextID.Add(1)
	transport := c.transport
	c.mu.Unlock()

	pingReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "ping",
	}
	if err := transport.Send(pingReq); err != nil {
		return fmt.Errorf("ping send: %w", err)
	}
	// Receive the pong; Receive() already has a 60s timeout internally.
	resp, err := transport.Receive()
	if err != nil {
		return fmt.Errorf("ping receive: %w", err)
	}
	if resp.Error != nil {
		// Some servers return method-not-found for ping — treat as alive.
		if resp.Error.Code == -32601 {
			return nil
		}
		return fmt.Errorf("ping error: %s", resp.Error.Message)
	}
	return nil
}

// reconnectWithBackoff tears down the current transport, then retries Connect
// and DiscoverTools with exponential backoff and ±25 % jitter.
// On success it increments the Prometheus reconnect counter and calls
// RefreshTools. On permanent failure (maxTry exceeded) it logs and returns.
func (c *MCPClient) reconnectWithBackoff(ctx context.Context, bp backoffParams) {
	// Mark disconnected so Connect() will proceed.
	c.mu.Lock()
	if c.transport != nil {
		c.transport.Close() //nolint:errcheck
	}
	c.connected = false
	c.mu.Unlock()

	delay := bp.initial
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			slog.Info("MCP reconnect cancelled", "server", c.name)
			return
		default:
		}

		if bp.maxTry > 0 && attempt > bp.maxTry {
			slog.Error("MCP reconnect giving up after max attempts",
				"server", c.name, "attempts", attempt-1)
			return
		}

		slog.Info("MCP reconnecting", "server", c.name, "attempt", attempt, "delay", delay)

		connCtx, connCancel := context.WithTimeout(ctx, 30*time.Second)
		err := c.Connect(connCtx)
		connCancel()

		if err == nil {
			observability.MCPServerReconnectsTotal.WithLabelValues(c.name).Inc()
			slog.Info("MCP reconnected", "server", c.name, "attempt", attempt)

			// Refresh tool definitions on the new connection.
			if refreshErr := c.RefreshTools(ctx); refreshErr != nil {
				slog.Warn("MCP tool refresh after reconnect failed",
					"server", c.name, "error", refreshErr)
			}
			return
		}

		slog.Warn("MCP reconnect attempt failed",
			"server", c.name, "attempt", attempt, "error", err)

		// Apply ±25 % jitter: delay * [0.75, 1.25).
		jitter := delay/4 + time.Duration(rand.Int64N(int64(delay/2)))
		sleep := delay - delay/4 + jitter
		if sleep > bp.max {
			sleep = bp.max
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(sleep):
		}

		delay = time.Duration(float64(delay) * bp.factor)
		if delay > bp.max {
			delay = bp.max
		}
	}
}

// ---------- MCP Manager ----------

// mcpManager manages all MCP server connections.
var mcpManagerInstance = &mcpManager{
	clients: make(map[string]*MCPClient),
}

type mcpManager struct {
	mu      sync.RWMutex
	clients map[string]*MCPClient
}

func getMCPManager() *mcpManager {
	return mcpManagerInstance
}

// RegisterMCPTools discovers and registers tools from MCP server configurations.
// It connects to each configured server, discovers tools, and registers them.
func RegisterMCPTools(platform string) {
	RegisterMCPToolsWithSampling(platform, nil)
}

// RegisterMCPToolsWithSampling is like RegisterMCPTools but accepts an optional
// LLM client to enable MCP sampling support. When client is non-nil, connected
// MCP servers can issue sampling/createMessage requests.
func RegisterMCPToolsWithSampling(platform string, samplingClient *llm.Client) {
	mcpCfg, err := LoadMCPConfig()
	if err != nil {
		slog.Debug("No MCP configuration found", "error", err)
		return
	}

	if len(mcpCfg.Servers) == 0 {
		slog.Debug("No MCP servers configured")
		return
	}

	// Create a shared sampling handler if an LLM client was provided.
	var samplingHandler *SamplingHandler
	if samplingClient != nil {
		samplingHandler = NewSamplingHandler(samplingClient)
		slog.Info("MCP sampling support enabled")
	}

	mgr := getMCPManager()

	for name, server := range mcpCfg.Servers {
		// Check if this server is excluded for the current platform
		excluded := false
		for _, noMCP := range server.NoMCP {
			if noMCP == platform {
				excluded = true
				break
			}
		}
		if excluded {
			slog.Debug("MCP server excluded for platform", "server", name, "platform", platform)
			continue
		}

		slog.Info("Connecting to MCP server", "name", name, "command", server.Command)

		client := NewMCPClient(name, server)
		if samplingHandler != nil {
			client.SetSamplingHandler(samplingHandler)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := client.Connect(ctx); err != nil {
			cancel()
			slog.Warn("Failed to connect to MCP server", "name", name, "error", err)
			// Register a placeholder that explains the connection failure
			registerMCPPlaceholder(name, server, err)
			continue
		}

		tools, err := client.DiscoverTools(ctx)
		cancel()

		if err != nil {
			slog.Warn("Failed to discover MCP tools", "name", name, "error", err)
			registerMCPPlaceholder(name, server, err)
			continue
		}

		mgr.mu.Lock()
		mgr.clients[name] = client
		mgr.mu.Unlock()

		// Register each discovered tool
		for _, tool := range tools {
			registerMCPTool(name, client, tool)
		}

		// Start background goroutines for SSE-based servers.
		// bgCtx lives for the process lifetime; ShutdownAllMCP closes the
		// transport which causes both goroutines to exit via ctx / streamDone.
		bgCtx := context.Background()
		var bgWG sync.WaitGroup
		client.startNotificationWatcher(bgCtx, &bgWG)
		client.startHealthMonitor(bgCtx, &bgWG)

		slog.Info("MCP server registered", "name", name, "tools", len(tools))
	}
}

// registerMCPTool registers a single discovered MCP tool.
func registerMCPTool(serverName string, client *MCPClient, tool mcpToolDef) {
	// Namespace the tool name to avoid collisions
	fullName := fmt.Sprintf("mcp_%s_%s", serverName, tool.Name)

	schema := map[string]any{
		"name":        fullName,
		"description": fmt.Sprintf("[MCP:%s] %s", serverName, tool.Description),
	}

	if tool.InputSchema != nil {
		schema["parameters"] = tool.InputSchema
	} else {
		schema["parameters"] = map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	mcpToolName := tool.Name // capture for closure
	mcpClient := client      // capture for closure

	Register(&ToolEntry{
		Name:    fullName,
		Toolset: "mcp",
		Schema:  schema,
		Handler: func(ctx context.Context, args map[string]any, tctx *ToolContext) string {
			callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()

			result, err := mcpClient.CallTool(callCtx, mcpToolName, args)
			if err != nil {
				// Attempt reconnection
				slog.Warn("MCP tool call failed, attempting reconnect",
					"tool", mcpToolName, "server", serverName, "error", err)

				reconnCtx, reconnCancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer reconnCancel()

				mcpClient.mu.Lock()
				mcpClient.connected = false
				mcpClient.mu.Unlock()

				if reconnErr := mcpClient.Connect(reconnCtx); reconnErr != nil {
					return toJSON(map[string]any{
						"error":  fmt.Sprintf("MCP tool call failed and reconnect failed: %v (original: %v)", reconnErr, err),
						"server": serverName,
						"tool":   mcpToolName,
					})
				}

				// Retry after reconnect
				result, err = mcpClient.CallTool(callCtx, mcpToolName, args)
				if err != nil {
					return toJSON(map[string]any{
						"error":  fmt.Sprintf("MCP tool call failed after reconnect: %v", err),
						"server": serverName,
						"tool":   mcpToolName,
					})
				}
			}

			return result
		},
		Emoji: "\U0001f50c",
	})
}

// registerMCPPlaceholder registers a placeholder tool when server connection fails.
func registerMCPPlaceholder(name string, server MCPServerConfig, connErr error) {
	serverName := name
	Register(&ToolEntry{
		Name:    fmt.Sprintf("mcp_%s", serverName),
		Toolset: "mcp",
		Schema: map[string]any{
			"name":        fmt.Sprintf("mcp_%s", serverName),
			"description": fmt.Sprintf("MCP server '%s' - connection failed: %v", serverName, connErr),
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tool": map[string]any{
						"type":        "string",
						"description": "The MCP tool to invoke",
					},
					"arguments": map[string]any{
						"type":        "object",
						"description": "Arguments to pass to the MCP tool",
					},
				},
				"required": []string{"tool"},
			},
		},
		Handler: func(ctx context.Context, args map[string]any, tctx *ToolContext) string {
			return toJSON(map[string]any{
				"error":   fmt.Sprintf("MCP server '%s' is not connected: %v", serverName, connErr),
				"server":  serverName,
				"command": server.Command,
				"hint":    "Check that the MCP server binary is installed and accessible. Restart Hermes to retry.",
			})
		},
		Emoji: "\U0001f50c",
	})
}

// ShutdownAllMCP cleanly shuts down all MCP server connections.
func ShutdownAllMCP() {
	mgr := getMCPManager()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for name, client := range mgr.clients {
		slog.Info("Shutting down MCP server", "name", name)
		client.Shutdown()
	}
	mgr.clients = make(map[string]*MCPClient)
}
