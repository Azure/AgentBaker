package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGuestAgentEvent(t *testing.T) {
	tests := []struct {
		name       string
		taskName   string
		message    string
		eventLevel EventLevel
		startTime  time.Time
		endTime    time.Time
	}{
		{
			name:       "error event",
			taskName:   "AKS.AKSNodeController.Provision",
			message:    "aks-node-controller exited with code 1",
			eventLevel: EventLevelError,
			startTime:  time.Date(2026, 2, 3, 10, 30, 45, 123000000, time.UTC),
			endTime:    time.Date(2026, 2, 3, 10, 35, 50, 456000000, time.UTC),
		},
		{
			name:       "informational event",
			taskName:   "AKS.AKSNodeController.Provision",
			message:    "Completed",
			eventLevel: EventLevelInformational,
			startTime:  time.Date(2026, 2, 3, 14, 20, 15, 789000000, time.UTC),
			endTime:    time.Date(2026, 2, 3, 14, 25, 30, 987000000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for this test
			tmpDir := t.TempDir()
			eventsDir := filepath.Join(tmpDir, "events")

			// Call the function with test directory
			CreateGuestAgentEventWithDir(eventsDir, tt.taskName, tt.message, tt.eventLevel, tt.startTime, tt.endTime)
			files, err := os.ReadDir(eventsDir)
			require.NoError(t, err, "should be able to read events directory")
			require.Len(t, files, 1, "should have created exactly one event file")

			// Read and parse the event file
			eventFilePath := filepath.Join(eventsDir, files[0].Name())
			data, err := os.ReadFile(eventFilePath)
			require.NoError(t, err, "should be able to read event file")

			var event GuestAgentEvent
			err = json.Unmarshal(data, &event)
			require.NoError(t, err, "event file should contain valid JSON")

			// Verify the event contents
			assert.Equal(t, tt.taskName, event.TaskName, "TaskName should match")

			// Verify message includes timing information
			durationMs := tt.endTime.Sub(tt.startTime).Milliseconds()
			expectedMessage := fmt.Sprintf("%s | startTime=%s endTime=%s durationMs=%d",
				tt.message,
				tt.startTime.Format("2006-01-02 15:04:05.000"),
				tt.endTime.Format("2006-01-02 15:04:05.000"),
				durationMs,
			)
			assert.Equal(t, expectedMessage, event.Message, "Message should include timing information")

			assert.Equal(t, string(tt.eventLevel), event.EventLevel, "EventLevel should match")
			assert.Equal(t, "1.23", event.Version, "Version should be 1.23")
			assert.Equal(t, "0", event.EventPid, "EventPid should be 0")
			assert.Equal(t, "0", event.EventTid, "EventTid should be 0")

			// Verify timestamp formatting
			expectedTimestamp := tt.startTime.Format("2006-01-02 15:04:05.000")
			assert.Equal(t, expectedTimestamp, event.Timestamp, "Timestamp should be formatted correctly")

			// Verify OperationId format (endTime formatted timestamp)
			expectedOperationId := tt.endTime.Format("2006-01-02 15:04:05.000")
			assert.Equal(t, expectedOperationId, event.OperationId, "OperationId should be formatted correctly")

			// Verify filename is a millisecond timestamp (based on current time, not startTime)
			filename := files[0].Name()
			assert.True(t, len(filename) > 10, "filename should be a millisecond timestamp")
			assert.Equal(t, ".json", filepath.Ext(filename), "file should have .json extension")
		})
	}
}

func TestCreateGuestAgentEvent_DirectoryCreationError(t *testing.T) {
	// This test verifies that the function handles directory creation errors gracefully
	// Create a file where we want the directory to be (to cause MkdirAll to fail)
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking-file")
	err := os.WriteFile(blockingFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Try to create events directory under a file (which will fail)
	eventsDir := filepath.Join(blockingFile, "events")
	startTime := time.Now()
	endTime := time.Now()

	// Should not panic, just log error
	CreateGuestAgentEventWithDir(eventsDir, "TestTask", "Test message", EventLevelError, startTime, endTime)

	// Verify no event file was created in the events directory (since directory creation failed)
	// The directory creation will fail, so reading the directory should fail
	files, err := os.ReadDir(eventsDir)
	assert.Error(t, err, "should not be able to read events directory that failed to create")
	assert.Empty(t, files, "no files should exist")
}

func TestCreateGuestAgentEvent_MultipleEventsWithSameStartTime(t *testing.T) {
	// This test verifies that multiple events with the same startTime don't overwrite each other.
	// This matches the behavior in main.go where multiple events share the same startTime.
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	startTime := time.Date(2026, 2, 3, 10, 30, 45, 123000000, time.UTC)
	endTime1 := time.Date(2026, 2, 3, 10, 30, 45, 124000000, time.UTC)
	endTime2 := time.Date(2026, 2, 3, 10, 35, 50, 456000000, time.UTC)

	// Create three events with the same startTime (similar to main.go)
	CreateGuestAgentEventWithDir(eventsDir, "AKS.AKSNodeController.Provision", "Starting", EventLevelInformational, startTime, startTime)
	time.Sleep(2 * time.Millisecond) // Ensure unique timestamp for filename
	CreateGuestAgentEventWithDir(eventsDir, "AKS.AKSNodeController.Provision", "Completed", EventLevelInformational, startTime, endTime2)
	time.Sleep(2 * time.Millisecond) // Ensure unique timestamp for filename
	CreateGuestAgentEventWithDir(eventsDir, "AKS.AKSNodeController.Provision", "aks-node-controller exited with code 1", EventLevelError, startTime, endTime1)

	// Verify all three events were created as separate files
	files, err := os.ReadDir(eventsDir)
	require.NoError(t, err, "should be able to read events directory")
	assert.Len(t, files, 3, "should have created three separate event files")

	// Verify all filenames are unique
	filenames := make(map[string]bool)
	for _, file := range files {
		filenames[file.Name()] = true
	}
	assert.Len(t, filenames, 3, "all filenames should be unique")

	// Verify each file contains valid event data
	messages := make(map[string]bool)
	for _, file := range files {
		eventFilePath := filepath.Join(eventsDir, file.Name())
		data, err := os.ReadFile(eventFilePath)
		require.NoError(t, err, "should be able to read event file")

		var event GuestAgentEvent
		err = json.Unmarshal(data, &event)
		require.NoError(t, err, "event file should contain valid JSON")

		messages[event.Message] = true
	}

	// Verify we captured all three different messages
	assert.Len(t, messages, 3, "should have three distinct event messages")

	// Verify the specific messages exist
	var hasStarting, hasCompleted, hasError bool
	for msg := range messages {
		if strings.Contains(msg, "Starting") {
			hasStarting = true
		}
		if strings.Contains(msg, "Completed") {
			hasCompleted = true
		}
		if strings.Contains(msg, "exited with code") {
			hasError = true
		}
	}
	assert.True(t, hasStarting, "should have the 'Starting' event")
	assert.True(t, hasCompleted, "should have the 'Completed' event")
	assert.True(t, hasError, "should have the error event")
}
