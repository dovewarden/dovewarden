package doveadm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSyncSuccess verifies that a successful sync request works
func TestSyncSuccess(t *testing.T) {
	// Create a mock Doveadm server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/doveadm/v1" {
			t.Errorf("expected path /doveadm/v1, got %s", r.URL.Path)
		}

		// Verify authentication
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth")
		}
		if user != "testuser" || pass != "testpass" {
			t.Errorf("unexpected credentials: %s:%s", user, pass)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var payload []interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(payload) != 1 {
			t.Fatalf("expected 1 element in payload, got %d", len(payload))
		}

		// Send success response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[["sync",{"status":"ok"},"dovewarden-sync"]]`)
	}))
	defer server.Close()

	// Create client and test sync
	client := NewClient(server.URL, "testuser", "testpass")
	ctx := context.Background()

	err := client.Sync(ctx, "user-a", "imap")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSyncServerError verifies error handling for server errors
func TestSyncServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Internal Server Error")
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	ctx := context.Background()

	err := client.Sync(ctx, "user-a", "imap")
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

// TestSyncUnauthorized verifies error handling for authentication failures
func TestSyncUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintf(w, "Unauthorized")
	}))
	defer server.Close()

	client := NewClient(server.URL, "wronguser", "wrongpass")
	ctx := context.Background()

	err := client.Sync(ctx, "user-a", "imap")
	if err == nil {
		t.Error("expected error for 401 status")
	}
}

// TestSyncPayloadFormat verifies the correct payload format is sent
func TestSyncPayloadFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload []interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify structure: [[command, params, tag]]
		cmdArray, ok := payload[0].([]interface{})
		if !ok {
			t.Fatal("expected array as first element")
		}

		if len(cmdArray) != 3 {
			t.Fatalf("expected 3 elements in command array, got %d", len(cmdArray))
		}

		// Verify command
		cmd, ok := cmdArray[0].(string)
		if !ok || cmd != "sync" {
			t.Errorf("expected 'sync' command, got %v", cmdArray[0])
		}

		// Verify params
		params, ok := cmdArray[1].(map[string]interface{})
		if !ok {
			t.Fatal("expected map for params")
		}

		user, ok := params["user"].(string)
		if !ok || user != "test-user" {
			t.Errorf("expected user 'test-user', got %v", params["user"])
		}

		dest, ok := params["destination"].([]interface{})
		if !ok || len(dest) != 1 {
			t.Errorf("expected destination array with 1 element, got %v", params["destination"])
		}

		if destVal, ok := dest[0].(string); !ok || destVal != "imap" {
			t.Errorf("expected destination 'imap', got %v", dest[0])
		}

		// Verify tag
		tag, ok := cmdArray[2].(string)
		if !ok || tag != "dovewarden-sync" {
			t.Errorf("expected tag 'dovewarden-sync', got %v", cmdArray[2])
		}

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `[["sync",{"status":"ok"},"dovewarden-sync"]]`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass")
	ctx := context.Background()

	err := client.Sync(ctx, "test-user", "imap")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
