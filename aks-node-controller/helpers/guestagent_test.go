package helpers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventLogger_Log(t *testing.T) {
	logger := NewEventLogger(t.TempDir())
	logger.LogEvent("Provision", "Starting", EventLevelError,
		time.Date(2099, 2, 3, 10, 30, 45, 0, time.UTC),
		time.Date(2099, 2, 3, 10, 35, 50, 0, time.UTC))
	logger.LogEvent("Provision", "Completed", EventLevelInformational, time.Now(), time.Now())

	events := logger.Events()
	require.Len(t, events, 2)

	assert.Equal(t, "AKS.AKSNodeController.Provision", events[0].TaskName)
	assert.Equal(t, "Error", events[0].EventLevel)
	assert.Contains(t, events[0].Message, "Starting")
	assert.Contains(t, events[0].Message, "durationMs=305000")
	assert.Equal(t, "2099-02-03 10:30:45.000", events[0].Timestamp)
	assert.Equal(t, "2099-02-03 10:35:50.000", events[0].OperationId)
	assert.Equal(t, "1.23", events[0].Version)
}

func TestEventLogger_Events_EmptyDirectory(t *testing.T) {
	logger := NewEventLogger(t.TempDir())
	events := logger.Events()
	assert.Empty(t, events)
}
