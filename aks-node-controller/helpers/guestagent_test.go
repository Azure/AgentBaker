package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventLogger_Log(t *testing.T) {
	logger := NewEventLogger(filepath.Join(t.TempDir(), "events"))
	logger.LogEvent("Provision", "test message", EventLevelError,
		time.Date(2099, 2, 3, 10, 30, 45, 0, time.UTC),
		time.Date(2099, 2, 3, 10, 35, 50, 0, time.UTC))

	events, err := logger.Events()
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

func TestEventLogger_DirectoryCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking-file")
	err := os.WriteFile(blockingFile, []byte("test"), 0644)
	require.NoError(t, err)

	// Try to create events directory under a file (which will fail)
	logger := NewEventLogger(filepath.Join(blockingFile, "events"))
	logger.LogEvent("TestTask", "Test message", EventLevelError, time.Now(), time.Now())

	// Directory creation should fail, so reading should fail
	_, err = logger.Events()
	assert.Error(t, err)
}

func TestEventLogger_MultipleEvents(t *testing.T) {
	logger := NewEventLogger(filepath.Join(t.TempDir(), "events"))

	logger.LogEvent("Provision", "Starting", EventLevelInformational, time.Now(), time.Now())
	logger.LogEvent("Provision", "Completed", EventLevelInformational, time.Now(), time.Now())
	logger.LogEvent("Provision", "Failed", EventLevelError, time.Now(), time.Now())

	events, err := logger.Events()
	require.NoError(t, err)
	assert.Len(t, events, 3)

	for _, event := range events {
		assert.Equal(t, "AKS.AKSNodeController.Provision", event.TaskName)
	}
}

func TestEventLogger_Events_EmptyDirectory(t *testing.T) {
	logger := NewEventLogger(t.TempDir())
	events, err := logger.Events()
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestEventLogger_Events_NonExistentDirectory(t *testing.T) {
	logger := NewEventLogger("/nonexistent/path")
	_, err := logger.Events()
	assert.Error(t, err)
}
