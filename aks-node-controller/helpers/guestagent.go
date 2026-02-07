package helpers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// EventLevel represents the severity level of a guest agent event.
type EventLevel string

const (
	// EventLevelInformational indicates a successful operation or informational message.
	EventLevelInformational EventLevel = "Informational"
	// EventLevelError indicates a failure or error condition.
	EventLevelError EventLevel = "Error"
)

// GuestAgentEvent represents an event to be logged for the Azure VM guest agent.
type GuestAgentEvent struct {
	Timestamp   string `json:"Timestamp"`
	OperationId string `json:"OperationId"`
	Version     string `json:"Version"`
	TaskName    string `json:"TaskName"`
	EventLevel  string `json:"EventLevel"`
	Message     string `json:"Message"`
	EventPid    string `json:"EventPid"`
	EventTid    string `json:"EventTid"`
}

// EventLogger writes guest agent events to a directory.
type EventLogger struct {
	Dir string
}

// NewEventLogger creates an EventLogger that writes to the specified directory.
func NewEventLogger(dir string) *EventLogger {
	return &EventLogger{Dir: dir}
}

// LogEvent creates an event file for the Azure VM guest agent.
//
// The implementation matches the bash pattern used across the codebase:
//   - Filename: Uses current time (nanoseconds) to ensure uniqueness
//   - Timestamp: Event start time in format "2006-01-02 15:04:05.000"
//   - OperationId: Event end time in format "2006-01-02 15:04:05.000"
//   - Message: Includes timing information (startTime, endTime, durationMs)
func (l *EventLogger) LogEvent(taskName, message string, eventLevel EventLevel, startTime, endTime time.Time) {
	if err := os.MkdirAll(l.Dir, 0755); err != nil {
		slog.Error("failed to create events logging directory", "path", l.Dir, "error", err)
		return
	}

	// Use nanosecond timestamp as filename, based on current time to ensure uniqueness
	// This provides better collision avoidance than milliseconds
	eventsFileName := fmt.Sprintf("%d.json", time.Now().UnixNano())
	eventFilePath := filepath.Join(l.Dir, eventsFileName)

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

	event := GuestAgentEvent{
		Timestamp:   startTime.Format("2006-01-02 15:04:05.000"),
		OperationId: operationID,
		Version:     "1.23",
		TaskName:    "AKS.AKSNodeController." + taskName,
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

	// Event log files need to be readable by Azure monitoring services.
	// #nosec G306 -- Operational event data without sensitive information
	if err := os.WriteFile(eventFilePath, data, 0644); err != nil {
		slog.Error("failed to write guest agent event file", "path", eventFilePath, "error", err)
	}
}

// Events reads all guest agent event files from the directory.
// Events are returned in filename order (which corresponds to creation time since
// filenames are nanosecond timestamps). This method is primarily useful for testing.
func (l *EventLogger) Events() []GuestAgentEvent {
	files, err := os.ReadDir(l.Dir)
	if err != nil {
		slog.Error("failed to read events directory", "path", l.Dir, "error", err)
		return nil
	}

	var events []GuestAgentEvent
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(l.Dir, file.Name()))
		if err != nil {
			slog.Error("failed to read event file", "file", file.Name(), "error", err)
			return nil
		}

		var event GuestAgentEvent
		if err := json.Unmarshal(data, &event); err != nil {
			slog.Error("failed to unmarshal event file", "file", file.Name(), "error", err)
			return nil
		}

		events = append(events, event)
	}

	return events
}
