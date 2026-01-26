package doveadm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client handles communication with the Doveadm API
type Client struct {
	baseURL  string
	password string
	client   *http.Client
}

// NewClient creates a new Doveadm API client
func NewClient(baseURL, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		password: password,
		client:   &http.Client{},
	}
}

// SyncRequest represents a dsync request to Doveadm
type SyncRequest struct {
	Command     string   `json:"command"` // "sync"
	Destination []string `json:"destination"`
	User        string   `json:"user"`
}

// ResponseError represents an error entry returned by Doveadm
// [ [ "error", {"type":"exitCode","exitCode":75}, "dovewarden-sync" ] ]
type ResponseError struct {
	Type     string `json:"type"`
	ExitCode int    `json:"exitCode"`
}

// SyncResponse represents the response from a sync operation
type SyncResponse struct {
	State string // Replication state for incremental sync
}

// responseEntry models a single Doveadm response array.
type responseEntry struct {
	Status   string
	Error    *ResponseError
	Response map[string]interface{}
	Tag      string
}

func (r *responseEntry) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) < 3 {
		return fmt.Errorf("unexpected response format: %s", string(data))
	}

	if err := json.Unmarshal(raw[0], &r.Status); err != nil {
		return fmt.Errorf("failed to parse status: %w", err)
	}

	// Parse the second element based on status
	// If status is "error", parse as error payload; otherwise parse as response object
	if r.Status == "error" {
		var errPayload ResponseError
		if err := json.Unmarshal(raw[1], &errPayload); err != nil {
			return fmt.Errorf("failed to parse error payload: %w", err)
		}
		r.Error = &errPayload
	} else {
		// Parse as response object for successful responses
		var respObj map[string]interface{}
		if err := json.Unmarshal(raw[1], &respObj); err == nil {
			r.Response = respObj
		}
	}

	if err := json.Unmarshal(raw[2], &r.Tag); err != nil {
		return fmt.Errorf("failed to parse tag: %w", err)
	}

	return nil
}

// Sync performs a dsync operation for the given user to the specified destination.
// If state is provided (non-empty), it will be used for incremental sync.
// Returns the new state string for the next sync operation.
func (c *Client) Sync(ctx context.Context, username string, destination string, state string) (*SyncResponse, error) {
	// Build the request payload according to Doveadm API format:
	// [["sync",{"destination":["$destination"],"user":"$username","state":"$state"},"tag1"]]
	params := map[string]interface{}{
		"destination": []string{destination},
		"user":        username,
	}

	// Add state parameter if provided
	if state != "" {
		params["state"] = state
	}

	payload := []interface{}{
		[]interface{}{
			"sync",
			params,
			"dovewarden-sync",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/doveadm/v1", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("doveadm", c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response body for debugging
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("doveadm sync failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Doveadm API returns error with HTTP 200 but indicates failure in the response body
	var respPayload []responseEntry
	if err := json.Unmarshal(respBody, &respPayload); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	syncResp := &SyncResponse{}
	for _, entry := range respPayload {
		if entry.Status == "error" {
			if entry.Error != nil {
				return nil, fmt.Errorf("doveadm sync error (tag %s): %s (exitCode %d)", entry.Tag, entry.Error.Type, entry.Error.ExitCode)
			}
			return nil, fmt.Errorf("doveadm sync error (tag %s): unknown reason", entry.Tag)
		}

		// Extract state from response if available
		if entry.Response != nil {
			if stateVal, ok := entry.Response["state"].(string); ok {
				syncResp.State = stateVal
			}
		}
	}

	return syncResp, nil
}
