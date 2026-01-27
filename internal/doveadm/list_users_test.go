package doveadm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListUsersSuccess verifies that a successful user list request works
func TestListUsersSuccess(t *testing.T) {
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
		_, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("expected basic auth")
		}
		if pass != "testpass" {
			t.Errorf("unexpected credentials: %s", pass)
		}

		// Send success response with user list
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[["user",[{"username":"user-a","uid":"1000","gid":"1000","home":"/home/user-a"},{"username":"user-b","uid":"1001","gid":"1001","home":"/home/user-b"}],"dovewarden-list-users"]]`)
	}))
	defer server.Close()

	// Create client and test ListUsers
	client := NewClient(server.URL, "testpass")
	ctx := context.Background()

	users, err := client.ListUsers(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Username != "user-a" {
		t.Errorf("expected first user to be 'user-a', got %s", users[0].Username)
	}
	if users[1].Username != "user-b" {
		t.Errorf("expected second user to be 'user-b', got %s", users[1].Username)
	}
}

// TestListUsersServerError verifies error handling for server errors
func TestListUsersServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, "Internal Server Error")
	}))
	defer server.Close()

	client := NewClient(server.URL, "testpass")
	ctx := context.Background()

	_, err := client.ListUsers(ctx)
	if err == nil {
		t.Error("expected error for 500 status")
	}
}

// TestListUsersEmpty verifies handling of empty user list
func TestListUsersEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[["user",[],"dovewarden-list-users"]]`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "testpass")
	ctx := context.Background()

	users, err := client.ListUsers(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}
}
