package events

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFilterWithFixtures(t *testing.T) {
	tests := []struct {
		name            string
		fixtureFile     string
		shouldPass      bool
		expectedErr     error
		expectedCmdName string
	}{
		{
			name:            "events.jsonl should pass filter",
			fixtureFile:     "../../fixtures/events.jsonl",
			shouldPass:      true,
			expectedErr:     nil,
			expectedCmdName: "APPEND",
		},
		{
			name:            "events-ignore.jsonl should fail filter",
			fixtureFile:     "../../fixtures/events-ignore.jsonl",
			shouldPass:      false,
			expectedErr:     ErrInvalidCmdName,
			expectedCmdName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read fixture file
			data, err := os.ReadFile(tt.fixtureFile)
			if err != nil {
				t.Fatalf("failed to read fixture file %s: %v", tt.fixtureFile, err)
			}

			// Split by newlines and process each event
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}

				// Test the filter
				result, err := Filter([]byte(line))

				if tt.shouldPass {
					if err != nil {
						t.Errorf("Filter() returned unexpected error: %v", err)
					}
					if result == nil {
						t.Error("Filter() returned nil for event that should pass")
					}
					if result.CmdName != tt.expectedCmdName {
						t.Errorf("expected CmdName %s, got %s", tt.expectedCmdName, result.CmdName)
					}
					if result.Username == "" {
						t.Error("expected non-empty username")
					}
				} else {
					if err == nil {
						t.Errorf("Filter() should have returned error, got nil")
					}
					if err != tt.expectedErr {
						t.Errorf("expected error %v, got %v", tt.expectedErr, err)
					}
				}
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
