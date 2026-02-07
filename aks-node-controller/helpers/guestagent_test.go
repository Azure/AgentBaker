package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGuestAgentEvent(t *testing.T) {
	eventsDir := filepath.Join(t.TempDir(), "events")
	createGuestAgentEventWithDir(eventsDir, "Provision", "test message", EventLevelError,
		time.Date(2099, 2, 3, 10, 30, 45, 0, time.UTC),
		time.Date(2099, 2, 3, 10, 35, 50, 0, time.UTC))

	events, err := ReadEvents(eventsDir)
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, "AKS.AKSNodeController.Provision", event.TaskName)
	assert.Equal(t, "Error", event.EventLevel)
	assert.Contains(t, event.Message, "test message")
	assert.Contains(t, event.Message, "durationMs=305000")
	assert.Equal(t, "2099-02-03 10:30:45.000", event.Timestamp)
	assert.Equal(t, "2099-02-03 10:35:50.000", event.OperationId)
	assert.Equal(t, "1.23", event.Version)
}

func TestCreateGuestAgentEvent_DirectoryCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking-file")
	err := os.WriteFile(blockingFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Try to create events directory under a file (which will fail)
	eventsDir := filepath.Join(blockingFile, "events")
	createGuestAgentEventWithDir(eventsDir, "TestTask", "Test message", EventLevelError, time.Now(), time.Now())

	// Directory creation should fail, so reading should fail
	_, err = ReadEvents(eventsDir)
	assert.Error(t, err)
}

func TestCreateGuestAgentEvent_MultipleEvents(t *testing.T) {
	eventsDir := filepath.Join(t.TempDir(), "events")
	createEvent := NewCreateEventFunc(eventsDir)

	createEvent("Provision", "Starting", EventLevelInformational, time.Now(), time.Now())
	createEvent("Provision", "Completed", EventLevelInformational, time.Now(), time.Now())
	createEvent("Provision", "Failed", EventLevelError, time.Now(), time.Now())

	events, err := ReadEvents(eventsDir)
	require.NoError(t, err)
	assert.Len(t, events, 3)

	// Verify all events have correct task name prefix
	for _, event := range events {
		assert.Equal(t, "AKS.AKSNodeController.Provision", event.TaskName)
	}
}

func TestReadEvents_EmptyDirectory(t *testing.T) {
	eventsDir := t.TempDir()
	events, err := ReadEvents(eventsDir)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestReadEvents_NonExistentDirectory(t *testing.T) {
	_, err := ReadEvents("/nonexistent/path")
	assert.Error(t, err)
}
