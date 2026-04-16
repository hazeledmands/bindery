// Package transmission provides a client for the Transmission RPC API,
// used to submit magnet/torrent URLs and poll status for torrent downloads.
package transmission

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client interacts with the Transmission RPC API.
// Authentication uses HTTP Basic (optional) plus a session token obtained
// via a 409 challenge on the first request.
//
// Field mapping for DownloadClient storage:
//   - APIKey  -> password  (Transmission uses username/password, not an API key)
//   - URLBase -> username  (reused since Transmission ignores URL base)
type Client struct {
	baseURL   string
	username  string
	password  string
	http      *http.Client
	mu        sync.Mutex
	sessionID string
}

// rpcRequest is the JSON envelope sent to the Transmission RPC endpoint.
type rpcRequest struct {
	Method    string      `json:"method"`
	Arguments interface{} `json:"arguments,omitempty"`
}

// rpcResponse is the JSON envelope returned by the Transmission RPC endpoint.
type rpcResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// New creates a Transmission client.
// username and password map to the DownloadClient's URLBase and APIKey fields
// respectively (see comment on the Client struct).
func New(host string, port int, username, password string, useSSL bool) *Client {
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return &Client{
		baseURL:  fmt.Sprintf("%s://%s:%d", scheme, host, port),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

// Test verifies connectivity by calling session-get.
func (c *Client) Test(ctx context.Context) error {
	_, err := c.rpc(ctx, "session-get", nil)
	if err != nil {
		return fmt.Errorf("could not reach Transmission at %s — %w (in Docker use the service/container name, not localhost)", c.baseURL, err)
	}
	return nil
}

// AddTorrent submits a magnet link or torrent URL to Transmission for download.
// The category parameter is used as the download directory (Transmission has no
// built-in category concept).
func (c *Client) AddTorrent(ctx context.Context, magnetOrURL, category, savePath string) error {
	args := map[string]interface{}{
		"filename": magnetOrURL,
	}
	dir := savePath
	if dir == "" {
		dir = category
	}
	if dir != "" {
		args["download-dir"] = dir
	}

	resp, err := c.rpc(ctx, "torrent-add", args)
	if err != nil {
		return fmt.Errorf("add torrent: %w", err)
	}
	if resp.Result != "success" {
		return fmt.Errorf("add torrent failed: %s", resp.Result)
	}
	return nil
}

// AddTorrentFile submits a .torrent file (as raw bytes) to Transmission.
func (c *Client) AddTorrentFile(ctx context.Context, data []byte, category, savePath string) error {
	args := map[string]interface{}{
		"metainfo": base64.StdEncoding.EncodeToString(data),
	}
	dir := savePath
	if dir == "" {
		dir = category
	}
	if dir != "" {
		args["download-dir"] = dir
	}

	resp, err := c.rpc(ctx, "torrent-add", args)
	if err != nil {
		return fmt.Errorf("add torrent file: %w", err)
	}
	if resp.Result != "success" {
		return fmt.Errorf("add torrent file failed: %s", resp.Result)
	}
	return nil
}

// GetTorrents returns all torrents. The category parameter is accepted for
// interface compatibility but is unused (Transmission has no built-in
// category concept).
func (c *Client) GetTorrents(ctx context.Context, _ string) ([]Torrent, error) {
	args := map[string]interface{}{
		"fields": []string{
			"id", "name", "hashString", "status", "percentDone",
			"rateDownload", "peersConnected", "error", "errorString", "downloadDir",
		},
	}

	resp, err := c.rpc(ctx, "torrent-get", args)
	if err != nil {
		return nil, err
	}
	if resp.Result != "success" {
		return nil, fmt.Errorf("torrent-get failed: %s", resp.Result)
	}

	var result struct {
		Torrents []Torrent `json:"torrents"`
	}
	if err := json.Unmarshal(resp.Arguments, &result); err != nil {
		return nil, fmt.Errorf("decode torrents: %w", err)
	}
	return result.Torrents, nil
}

// DeleteTorrent removes a torrent by hash, optionally deleting its files.
// Transmission accepts hash strings as torrent identifiers.
func (c *Client) DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	args := map[string]interface{}{
		"ids":               []string{hash},
		"delete-local-data": deleteFiles,
	}

	resp, err := c.rpc(ctx, "torrent-remove", args)
	if err != nil {
		return fmt.Errorf("delete torrent: %w", err)
	}
	if resp.Result != "success" {
		return fmt.Errorf("delete torrent failed: %s", resp.Result)
	}
	return nil
}

// rpc sends an RPC request to Transmission and returns the parsed response.
// It handles the 409 session-token challenge transparently.
func (c *Client) rpc(ctx context.Context, method string, arguments interface{}) (*rpcResponse, error) {
	resp, err := c.doRPC(ctx, method, arguments)
	if err != nil {
		return nil, err
	}

	// Handle 409 Conflict — Transmission returns the session ID in the header.
	if resp.StatusCode == http.StatusConflict {
		sid := resp.Header.Get("X-Transmission-Session-Id")
		resp.Body.Close()
		if sid == "" {
			return nil, fmt.Errorf("409 conflict but no X-Transmission-Session-Id header")
		}
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()

		resp, err = c.doRPC(ctx, method, arguments)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed (HTTP 401)")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &rpcResp, nil
}

// doRPC performs a single HTTP POST to the Transmission RPC endpoint.
func (c *Client) doRPC(ctx context.Context, method string, arguments interface{}) (*http.Response, error) {
	body, err := json.Marshal(rpcRequest{Method: method, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/transmission/rpc", bytes.NewReader(body)) // #nosec
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		req.Header.Set("X-Transmission-Session-Id", sid)
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	return c.http.Do(req) // #nosec
}
