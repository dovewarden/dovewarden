package queue

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// TestLastReplicationTime verifies storing and retrieving last replication time
func TestLastReplicationTime(t *testing.T) {
	logger := slog.Default()
	q, err := NewInMemoryQueue("test", "", logger)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		_ = q.Close()
	}()

	ctx := context.Background()
	username := "test-user"

	// Initially, should return zero time
	lastTime, err := q.GetLastReplicationTime(ctx, username)
	if err != nil {
		t.Errorf("unexpected error getting initial time: %v", err)
	}
	if !lastTime.IsZero() {
		t.Errorf("expected zero time, got %v", lastTime)
	}

	// Set a replication time
	now := time.Now()
	if err := q.SetLastReplicationTime(ctx, username, now); err != nil {
		t.Errorf("unexpected error setting time: %v", err)
	}

	// Retrieve and verify
	retrieved, err := q.GetLastReplicationTime(ctx, username)
	if err != nil {
		t.Errorf("unexpected error getting time: %v", err)
	}

	// Unix timestamps lose sub-second precision
	if retrieved.Unix() != now.Unix() {
		t.Errorf("expected time %v, got %v", now.Unix(), retrieved.Unix())
	}
}

// TestLastReplicationTimeUpdate verifies updating an existing timestamp
func TestLastReplicationTimeUpdate(t *testing.T) {
	logger := slog.Default()
	q, err := NewInMemoryQueue("test", "", logger)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		_ = q.Close()
	}()

	ctx := context.Background()
	username := "test-user"

	// Set first timestamp
	time1 := time.Now().Add(-1 * time.Hour)
	if err := q.SetLastReplicationTime(ctx, username, time1); err != nil {
		t.Errorf("unexpected error setting first time: %v", err)
	}

	// Set second timestamp
	time2 := time.Now()
	if err := q.SetLastReplicationTime(ctx, username, time2); err != nil {
		t.Errorf("unexpected error setting second time: %v", err)
	}

	// Retrieve and verify it's the second timestamp
	retrieved, err := q.GetLastReplicationTime(ctx, username)
	if err != nil {
		t.Errorf("unexpected error getting time: %v", err)
	}

	if retrieved.Unix() != time2.Unix() {
		t.Errorf("expected time %v, got %v", time2.Unix(), retrieved.Unix())
	}
}

// TestLastReplicationTimeMultipleUsers verifies independent timestamps per user
func TestLastReplicationTimeMultipleUsers(t *testing.T) {
	logger := slog.Default()
	q, err := NewInMemoryQueue("test", "", logger)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	defer func() {
		_ = q.Close()
	}()

	ctx := context.Background()

	// Set different times for different users
	time1 := time.Now().Add(-2 * time.Hour)
	time2 := time.Now().Add(-1 * time.Hour)

	if err := q.SetLastReplicationTime(ctx, "user-a", time1); err != nil {
		t.Errorf("unexpected error setting time for user-a: %v", err)
	}
	if err := q.SetLastReplicationTime(ctx, "user-b", time2); err != nil {
		t.Errorf("unexpected error setting time for user-b: %v", err)
	}

	// Verify each user has their own timestamp
	retrieved1, err := q.GetLastReplicationTime(ctx, "user-a")
	if err != nil {
		t.Errorf("unexpected error getting time for user-a: %v", err)
	}
	if retrieved1.Unix() != time1.Unix() {
		t.Errorf("user-a: expected time %v, got %v", time1.Unix(), retrieved1.Unix())
	}

	retrieved2, err := q.GetLastReplicationTime(ctx, "user-b")
	if err != nil {
		t.Errorf("unexpected error getting time for user-b: %v", err)
	}
	if retrieved2.Unix() != time2.Unix() {
		t.Errorf("user-b: expected time %v, got %v", time2.Unix(), retrieved2.Unix())
	}
}
