package events

import (
	"encoding/json"
	"errors"
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
	"APPEND": true,
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

	if !AcceptedCmdNames[evt.Fields.CmdName] {
		return nil, ErrInvalidCmdName
	}

	return &FilteredEvent{
		Event:    evt.Event,
		Username: evt.Fields.User,
		CmdName:  evt.Fields.CmdName,
		Raw:      evt,
	}, nil
}
