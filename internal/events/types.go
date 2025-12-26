package events

// Fields represents nested fields in a Dovecot event.
type Fields struct {
	User    string `json:"user"`
	CmdName string `json:"cmd_name"`
	// Additional fields can be added here as needed
}

// Event represents a Dovecot event from the event API.
type Event struct {
	Event    string `json:"event"`
	Fields   Fields `json:"fields"`
	Hostname string `json:"hostname,omitempty"`
	// Additional fields from Dovecot can be added here as needed
}

// FilteredEvent represents an event that passed filter validation.
type FilteredEvent struct {
	Event    string
	Username string
	CmdName  string
	Raw      Event
}
