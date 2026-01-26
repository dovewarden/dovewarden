package events

import (
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrEmptyEvent       = errors.New("event field is empty")
	ErrEmptyUsername    = errors.New("username field is empty")
	ErrInvalidEventType = errors.New("event type not accepted by filter")
	ErrInvalidCmdName   = errors.New("cmd_name not accepted by filter")
)

// AcceptedEvents is the list of event types that pass the filter.
var AcceptedEvents = map[string]bool{
	"imap_command_finished": true,
}

// AcceptedCmdNames is the list of IMAP commands that should be queued.
var AcceptedCmdNames = map[string]bool{
	"APPEND":       true,
	"AUTHENTICATE": false,
	"CAPABILITY":   false,
	"CLOSE":        true,
	"COPY":         true,
	"CREATE":       true,
	"DELETE":       true,
	"DELETEACL":    true,
	"ENABLE":       false,
	"EXAMINE":      false,
	"EXPUNGE":      true,
	"FETCH":        false,
	"GETACL":       false,
	"GETMETADATA":  false,
	"GETQUOTA":     false,
	"GETQUOTAROOT": false,
	"ID":           false,
	"IDLE":         false,
	"LIST":         false,
	"LISTRIGHTS":   false,
	"LSUB":         false,
	"LOGIN":        false,
	"LOGOUT":       false,
	"MOVE":         true,
	"MYRIGHTS":     false,
	"NAMESPACE":    false,
	"NOOP":         false,
	"RENAME":       true,
	"SEARCH":       false,
	"SELECT":       false,
	"SETACL":       true,
	"SETMETADATA":  true,
	"SETQUOTA":     true,
	"STARTTLS":     false,
	"STORE":        true,
	"STATUS":       false,
	"SUBSCRIBE":    true,
	"UID COPY":     true,
	"UID DELETE":   true,
	"UID EXPUNGE":  true,
	"UID MOVE":     true,
	"UID STORE":    true,
	"UNSELECT":     false,
	"UNSUBSCRIBE":  true,
}

// Filter validates and filters incoming events.
// Returns a FilteredEvent if the event passes, or an error if it doesn't.
func Filter(data []byte) (*FilteredEvent, error) {
	var evt Event
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, err
	}

	if evt.Event == "" {
		return nil, ErrEmptyEvent
	}

	if !AcceptedEvents[evt.Event] {
		return nil, ErrInvalidEventType
	}

	if evt.Fields.User == "" {
		return nil, ErrEmptyUsername
	}

	if !AcceptedCmdNames[strings.ToUpper(evt.Fields.CmdName)] {
		return nil, ErrInvalidCmdName
	}

	return &FilteredEvent{
		Event:    evt.Event,
		Username: evt.Fields.User,
		CmdName:  evt.Fields.CmdName,
		Raw:      evt,
	}, nil
}
