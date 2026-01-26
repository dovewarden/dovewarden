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

// Additional edge-case coverage focusing on malformed inputs and corner cases
func TestFilterEdgeCases(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		data := []byte("")
		res, err := Filter(data)
		if err == nil || res != nil {
			t.Fatalf("expected JSON unmarshal error for empty input, got res=%v err=%v", res, err)
		}
	})

	t.Run("whitespace input", func(t *testing.T) {
		data := []byte("   \n\t  ")
		res, err := Filter(data)
		if err == nil || res != nil {
			t.Fatalf("expected JSON unmarshal error for whitespace input, got res=%v err=%v", res, err)
		}
	})

	t.Run("garbage input", func(t *testing.T) {
		data := []byte("not json")
		res, err := Filter(data)
		if err == nil || res != nil {
			t.Fatalf("expected JSON unmarshal error for garbage input, got res=%v err=%v", res, err)
		}
	})

	t.Run("empty object -> empty event error", func(t *testing.T) {
		data := []byte("{}")
		res, err := Filter(data)
		if err != ErrEmptyEvent {
			t.Fatalf("expected ErrEmptyEvent, got res=%v err=%v", res, err)
		}
		if res != nil {
			t.Fatalf("expected nil result, got %v", res)
		}
	})

	t.Run("missing fields -> empty username error", func(t *testing.T) {
		payload := map[string]any{
			"event": "imap_command_finished",
		}
		data, _ := json.Marshal(payload)
		res, err := Filter(data)
		if err != ErrEmptyUsername {
			t.Fatalf("expected ErrEmptyUsername, got res=%v err=%v", res, err)
		}
	})

	t.Run("lowercase accepted cmd_name should pass", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "alice", CmdName: "append"},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != nil || res == nil {
			t.Fatalf("expected success, got res=%v err=%v", res, err)
		}
		if res.CmdName != "append" { // Filter preserves original case in result
			t.Fatalf("expected CmdName to be original 'append', got %q", res.CmdName)
		}
	})

	t.Run("cmd_name with trailing space should be rejected", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "bob", CmdName: "APPEND "},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != ErrInvalidCmdName || res != nil {
			t.Fatalf("expected ErrInvalidCmdName, got res=%v err=%v", res, err)
		}
	})

	t.Run("another accepted command (RENAME)", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "carol", CmdName: "RENAME"},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != nil || res == nil {
			t.Fatalf("expected success for RENAME, got res=%v err=%v", res, err)
		}
	})

	t.Run("DELETE command (lowercase)", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "dave", CmdName: "delete"},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != nil || res == nil {
			t.Fatalf("expected success for DELETE, got res=%v err=%v", res, err)
		}
		if res.CmdName != "delete" {
			t.Fatalf("expected CmdName to be original 'delete', got %q", res.CmdName)
		}
	})

	t.Run("UID DELETE command", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "eve", CmdName: "UID DELETE"},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != nil || res == nil {
			t.Fatalf("expected success for UID DELETE, got res=%v err=%v", res, err)
		}
	})

	t.Run("UID DELETE command (lowercase)", func(t *testing.T) {
		ev := Event{
			Event:  "imap_command_finished",
			Fields: Fields{User: "frank", CmdName: "uid delete"},
		}
		data, _ := json.Marshal(ev)
		res, err := Filter(data)
		if err != nil || res == nil {
			t.Fatalf("expected success for uid delete, got res=%v err=%v", res, err)
		}
	})
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
			name: "valid DELETE event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "DELETE",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid UID DELETE event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "UID DELETE",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid COPY event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "COPY",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid UID COPY event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "UID COPY",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid MOVE event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "MOVE",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid UID MOVE event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "UID MOVE",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid UID EXPUNGE event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "UID EXPUNGE",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid DELETEACL event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "DELETEACL",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid SETACL event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "SETACL",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid SETMETADATA event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "SETMETADATA",
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid SETQUOTA event",
			event: Event{
				Event: "imap_command_finished",
				Fields: Fields{
					User:    "testuser",
					CmdName: "SETQUOTA",
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
