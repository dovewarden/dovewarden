package events

import (
	"encoding/json"
	"os"
	"testing"
)

func TestFilterWithFixtures(t *testing.T) {
	// Test accepted events from fixtures/events/*.json
	acceptedDir := "../../fixtures/events"
	acceptedFiles, err := os.ReadDir(acceptedDir)
	if err != nil {
		t.Fatalf("failed to read fixtures/events directory: %v", err)
	}

	for _, file := range acceptedFiles {
		if file.IsDir() {
			continue
		}

		t.Run("accepted: "+file.Name(), func(t *testing.T) {
			// Read fixture file
			data, err := os.ReadFile(acceptedDir + "/" + file.Name())
			if err != nil {
				t.Fatalf("failed to read fixture file %s: %v", file.Name(), err)
			}

			// Test the filter
			result, err := Filter(data)

			if err != nil {
				t.Fatalf("Filter() returned unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("Filter() returned nil for event that should pass")
			}
			if result.Username == "" {
				t.Fatal("expected non-empty username")
			}
		})
	}

	// Test ignored events from fixtures/events/ignore/*.json
	ignoredDir := "../../fixtures/events/ignore"
	ignoredFiles, err := os.ReadDir(ignoredDir)
	if err != nil {
		t.Fatalf("failed to read fixtures/events/ignore directory: %v", err)
	}

	for _, file := range ignoredFiles {
		if file.IsDir() {
			continue
		}

		t.Run("ignored: "+file.Name(), func(t *testing.T) {
			// Read fixture file
			data, err := os.ReadFile(ignoredDir + "/" + file.Name())
			if err != nil {
				t.Fatalf("failed to read fixture file %s: %v", file.Name(), err)
			}

			// Test the filter
			result, err := Filter(data)

			if err == nil {
				t.Errorf("Filter() should have returned error, got nil")
			}
			if result != nil {
				t.Error("Filter() should have returned nil for invalid event")
			}
		})
	}
}

func TestFilterValidation(t *testing.T) {
	tests := []struct {
		name        string
		event       interface{}
		expectedErr error
	}{
		{
			name: "valid APPEND event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "APPEND",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid SELECT event is rejected",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "SELECT",
				},
			},
			expectedErr: ErrInvalidCmdName,
		},
		{
			name: "empty event type",
			event: Event{
				Event: "",
				Fields: Fields{
					User:    "testuser",
					CmdName: "APPEND",
				},
			},
			expectedErr: ErrEmptyEvent,
		},
		{
			name: "empty username",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "",
					CmdName: "APPEND",
				},
			},
			expectedErr: ErrEmptyUsername,
		},
		{
			name: "invalid event type",
			event: Event{
				Event: "some_other_event",
				Fields: Fields{
					User:    "testuser",
					CmdName: "APPEND",
				},
			},
			expectedErr: ErrInvalidEventType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal event to JSON
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}

			// Test the filter
			result, err := Filter(data)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("Filter() returned unexpected error: %v", err)
				}
				if result == nil {
					t.Error("Filter() returned nil for valid event")
				}
			} else {
				if err != tt.expectedErr {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				if result != nil {
					t.Error("Filter() should have returned nil for invalid event")
				}
			}
		})
	}
}

func TestFilteredEventProperties(t *testing.T) {
	event := Event{
		Event:    "imap_command_finished",
		Hostname: "test-host",
		Fields: Fields{
			User:    "user-a",
			CmdName: "APPEND",
		},
	}

	data, _ := json.Marshal(event)
	result, _ := Filter(data)

	if result.Event != "imap_command_finished" {
		t.Errorf("expected Event 'imap_command_finished', got %s", result.Event)
	}
	if result.Username != "user-a" {
		t.Errorf("expected Username 'user-a', got %s", result.Username)
	}
	if result.CmdName != "APPEND" {
		t.Errorf("expected CmdName 'APPEND', got %s", result.CmdName)
	}
	if result.Raw.Hostname != "test-host" {
		t.Errorf("expected Raw.Hostname 'test-host', got %s", result.Raw.Hostname)
	}
}
