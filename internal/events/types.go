package events

// Event represents a Dovecot event from the event API.
type Event struct {
	Event    string `json:"event"`
	Username string `json:"username"`
	// Additional fields from Dovecot can be added here as needed
	Timestamp string `json:"timestamp,omitempty"`
}

// FilteredEvent represents an event that passed filter validation.
type FilteredEvent struct {
	Event    string
	Username string
	Raw      Event
}
