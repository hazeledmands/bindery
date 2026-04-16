package transmission

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient creates a Client pointing at the given test server URL.
func newTestClient(serverURL, username, password string) *Client {
	c := New("localhost", 9091, username, password, false)
	c.baseURL = serverURL
	return c
}

// rpcHandler returns an http.HandlerFunc that simulates the Transmission RPC endpoint.
// It handles the 409 session-token challenge and dispatches by method name.
func rpcHandler(t *testing.T, handlers map[string]func(json.RawMessage) (int, interface{})) http.HandlerFunc {
	t.Helper()
	const sessionID = "test-session-id"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transmission/rpc" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Session token challenge
		if r.Header.Get("X-Transmission-Session-Id") != sessionID {
			w.Header().Set("X-Transmission-Session-Id", sessionID)
			w.WriteHeader(http.StatusConflict)
			return
		}

		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		args, _ := json.Marshal(req.Arguments)
		if h, ok := handlers[req.Method]; ok {
			code, resp := h(args)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			t.Errorf("unhandled method: %s", req.Method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}
}

func TestNew(t *testing.T) {
	c := New("myhost", 9091, "admin", "secret", false)
	if c.baseURL != "http://myhost:9091" {
		t.Errorf("baseURL: want %q, got %q", "http://myhost:9091", c.baseURL)
	}
	if c.username != "admin" || c.password != "secret" {
		t.Error("credentials not stored correctly")
	}
	if c.sessionID != "" {
		t.Error("sessionID should be empty on construction")
	}

	cs := New("securehost", 443, "u", "p", true)
	if cs.baseURL != "https://securehost:443" {
		t.Errorf("SSL baseURL: got %q", cs.baseURL)
	}
}

func TestTest_Success(t *testing.T) {
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"session-get": func(_ json.RawMessage) (int, interface{}) {
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
}

func TestTest_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.Test(context.Background()); err == nil {
		t.Fatal("expected Test to fail on 500")
	}
}

func TestTest_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "bad", "creds")
	if err := c.Test(context.Background()); err == nil {
		t.Fatal("expected Test to fail on 401")
	}
}

func TestAddTorrent_Success(t *testing.T) {
	var gotFilename string
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-add": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotFilename, _ = a["filename"].(string)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.AddTorrent(context.Background(), "magnet:?xt=urn:btih:abc123", "", ""); err != nil {
		t.Fatalf("AddTorrent: %v", err)
	}
	if gotFilename != "magnet:?xt=urn:btih:abc123" {
		t.Errorf("filename: want magnet URI, got %q", gotFilename)
	}
}

func TestAddTorrent_WithDownloadDir(t *testing.T) {
	var gotDir string
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-add": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotDir, _ = a["download-dir"].(string)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.AddTorrent(context.Background(), "magnet:?xt=urn:btih:abc", "", "/downloads/books"); err != nil {
		t.Fatalf("AddTorrent: %v", err)
	}
	if gotDir != "/downloads/books" {
		t.Errorf("download-dir: want '/downloads/books', got %q", gotDir)
	}
}

func TestAddTorrent_CategoryAsDir(t *testing.T) {
	var gotDir string
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-add": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotDir, _ = a["download-dir"].(string)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.AddTorrent(context.Background(), "magnet:?xt=urn:btih:abc", "/data/books", ""); err != nil {
		t.Fatalf("AddTorrent: %v", err)
	}
	if gotDir != "/data/books" {
		t.Errorf("download-dir: want '/data/books', got %q", gotDir)
	}
}

func TestAddTorrent_Failure(t *testing.T) {
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-add": func(_ json.RawMessage) (int, interface{}) {
			return http.StatusOK, rpcResponse{Result: "invalid or corrupt torrent file"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.AddTorrent(context.Background(), "magnet:?xt=urn:btih:bad", "", ""); err == nil {
		t.Fatal("expected error on non-success result")
	}
}

func TestAddTorrentFile_Success(t *testing.T) {
	var gotMetainfo string
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-add": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotMetainfo, _ = a["metainfo"].(string)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	data := []byte("fake torrent data")
	if err := c.AddTorrentFile(context.Background(), data, "", ""); err != nil {
		t.Fatalf("AddTorrentFile: %v", err)
	}
	if gotMetainfo == "" {
		t.Error("expected metainfo in request")
	}
}

func TestGetTorrents_Success(t *testing.T) {
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-get": func(_ json.RawMessage) (int, interface{}) {
			return http.StatusOK, map[string]interface{}{
				"result": "success",
				"arguments": map[string]interface{}{
					"torrents": []Torrent{
						{ID: 1, Name: "My Book", HashString: "abc123", Status: StatusDownloading, PercentDone: 0.5},
						{ID: 2, Name: "Another Book", HashString: "def456", Status: StatusSeeding, PercentDone: 1.0},
					},
				},
			}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	torrents, err := c.GetTorrents(context.Background(), "")
	if err != nil {
		t.Fatalf("GetTorrents: %v", err)
	}
	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}
	if torrents[0].HashString != "abc123" {
		t.Errorf("first hash: want 'abc123', got %q", torrents[0].HashString)
	}
	if torrents[1].Name != "Another Book" {
		t.Errorf("second name: want 'Another Book', got %q", torrents[1].Name)
	}
}

func TestGetTorrents_Failure(t *testing.T) {
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-get": func(_ json.RawMessage) (int, interface{}) {
			return http.StatusOK, rpcResponse{Result: "some error"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if _, err := c.GetTorrents(context.Background(), ""); err == nil {
		t.Fatal("expected error on non-success result")
	}
}

func TestDeleteTorrent_Success(t *testing.T) {
	var gotIDs []interface{}
	var gotDelete bool
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-remove": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotIDs, _ = a["ids"].([]interface{})
			gotDelete, _ = a["delete-local-data"].(bool)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.DeleteTorrent(context.Background(), "abc123", true); err != nil {
		t.Fatalf("DeleteTorrent: %v", err)
	}
	if len(gotIDs) != 1 || gotIDs[0] != "abc123" {
		t.Errorf("ids: want [abc123], got %v", gotIDs)
	}
	if !gotDelete {
		t.Error("delete-local-data: want true, got false")
	}
}

func TestDeleteTorrent_KeepFiles(t *testing.T) {
	var gotDelete bool
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-remove": func(args json.RawMessage) (int, interface{}) {
			var a map[string]interface{}
			_ = json.Unmarshal(args, &a)
			gotDelete, _ = a["delete-local-data"].(bool)
			return http.StatusOK, rpcResponse{Result: "success"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	_ = c.DeleteTorrent(context.Background(), "abc", false)
	if gotDelete {
		t.Error("delete-local-data: want false, got true")
	}
}

func TestDeleteTorrent_Failure(t *testing.T) {
	srv := httptest.NewServer(rpcHandler(t, map[string]func(json.RawMessage) (int, interface{}){
		"torrent-remove": func(_ json.RawMessage) (int, interface{}) {
			return http.StatusOK, rpcResponse{Result: "torrent not found"}
		},
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.DeleteTorrent(context.Background(), "abc", false); err == nil {
		t.Fatal("expected error on non-success result")
	}
}

// TestSessionTokenChallenge verifies that a 409 triggers session token acquisition.
func TestSessionTokenChallenge(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Header.Get("X-Transmission-Session-Id") != "my-session" {
			w.Header().Set("X-Transmission-Session-Id", "my-session")
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{Result: "success"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.Test(context.Background()); err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests (409 + retry), got %d", requestCount)
	}
	if c.sessionID != "my-session" {
		t.Errorf("sessionID: want 'my-session', got %q", c.sessionID)
	}
}

// TestBasicAuth verifies that credentials are sent when configured.
func TestBasicAuth(t *testing.T) {
	var gotUser, gotPass string
	var gotAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "s")
			w.WriteHeader(http.StatusConflict)
			return
		}
		gotUser, gotPass, gotAuth = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{Result: "success"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "admin", "secret")
	if err := c.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if !gotAuth {
		t.Fatal("expected Basic auth header")
	}
	if gotUser != "admin" || gotPass != "secret" {
		t.Errorf("auth: want admin/secret, got %s/%s", gotUser, gotPass)
	}
}

// TestNoAuthWhenUsernameEmpty verifies no Basic auth when username is empty.
func TestNoAuthWhenUsernameEmpty(t *testing.T) {
	var gotAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "s")
			w.WriteHeader(http.StatusConflict)
			return
		}
		_, _, gotAuth = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rpcResponse{Result: "success"})
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, "", "")
	if err := c.Test(context.Background()); err != nil {
		t.Fatalf("Test: %v", err)
	}
	if gotAuth {
		t.Error("did not expect Basic auth header when username is empty")
	}
}
