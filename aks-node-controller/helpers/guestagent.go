package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultEventsLoggingDir is the standard path for guest agent events.
	// This matches the path used by Azure VM CustomScript extension.
	DefaultEventsLoggingDir = "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
)

// EventLevel represents the severity level of a guest agent event.
type EventLevel string

const (
	// EventLevelInformational indicates a successful operation or informational message.
	EventLevelInformational EventLevel = "Informational"
	// EventLevelError indicates a failure or error condition.
	EventLevelError EventLevel = "Error"
)

// guestAgentEvent represents an event to be logged for the Azure VM guest agent.
type guestAgentEvent struct {
	Timestamp   string `json:"Timestamp"`
	OperationId string `json:"OperationId"`
	Version     string `json:"Version"`
	TaskName    string `json:"TaskName"`
	EventLevel  string `json:"EventLevel"`
	Message     string `json:"Message"`
	EventPid    string `json:"EventPid"`
	EventTid    string `json:"EventTid"`
}

// CreateGuestAgentEvent creates an event file for the Azure VM guest agent in the default directory.
// This mimics the format expected by the CustomScript extension event logging.
func CreateGuestAgentEvent(taskName, message string, eventLevel EventLevel, startTime, endTime time.Time) {
	createGuestAgentEventWithDir(DefaultEventsLoggingDir, taskName, message, eventLevel, startTime, endTime)
}

// createGuestAgentEventWithDir creates an event file in the specified directory.
// This function is separated to allow custom event directories for testing or special use cases.
//
// The implementation matches the bash pattern used across the codebase:
//   - Filename: Uses current time (milliseconds) to ensure uniqueness
//   - Timestamp: Event start time in format "2006-01-02 15:04:05.000"
//   - OperationId: Event end time in format "2006-01-02 15:04:05.000"
//   - Message: Includes timing information (startTime, endTime, durationMs)
func createGuestAgentEventWithDir(eventsDir, taskName, message string, eventLevel EventLevel, startTime, endTime time.Time) {
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		slog.Error("failed to create events logging directory", "path", eventsDir, "error", err)
		return
	}

	// Use millisecond timestamp as filename, based on current time to ensure uniqueness
	// This matches the bash implementation: eventsFileName=$(date +%s%3N)
	eventsFileName := fmt.Sprintf("%d.json", time.Now().UnixNano()/int64(time.Millisecond))
	eventFilePath := filepath.Join(eventsDir, eventsFileName)

	durationMs := endTime.Sub(startTime).Milliseconds()
	timingInfo := fmt.Sprintf("startTime=%s endTime=%s durationMs=%d",
		startTime.Format("2006-01-02 15:04:05.000"),
		endTime.Format("2006-01-02 15:04:05.000"),
		durationMs,
	)
	fullMessage := message
	if fullMessage == "" {
		fullMessage = timingInfo
	} else {
		fullMessage = fmt.Sprintf("%s | %s", message, timingInfo)
	}

	operationID := endTime.Format("2006-01-02 15:04:05.000")

	event := guestAgentEvent{
		Timestamp:   startTime.Format("2006-01-02 15:04:05.000"), // strange but this is Go's reference time for formatting
		OperationId: operationID,
		Version:     "1.23",
		TaskName:    taskName,
		EventLevel:  string(eventLevel),
		Message:     fullMessage,
		EventPid:    "0",
		EventTid:    "0",
	}

	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal guest agent event", "error", err)
		return
	}

	if err := os.WriteFile(eventFilePath, data, 0600); err != nil {
		slog.Error("failed to write guest agent event file", "path", eventFilePath, "error", err)
	}
}
