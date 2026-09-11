package helpers

import (
	"errors"
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

func TestEventLogger_RunTimedOperation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		logger := NewEventLogger(t.TempDir())

		err := logger.RunTimedOperation("Hotfix.BinaryOperation", func() (string, error) {
			return "route=package-manager outcome=success target=202604.01.1", nil
		})

		require.NoError(t, err)
		events := logger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.Hotfix.BinaryOperation", events[0].TaskName)
		assert.Equal(t, string(EventLevelInformational), events[0].EventLevel)
		assert.Contains(t, events[0].Message, "route=package-manager")
		assert.Contains(t, events[0].Message, "outcome=success")
		assert.Contains(t, events[0].Message, "target=202604.01.1")
		assert.Contains(t, events[0].Message, "durationMs=")
	})

	t.Run("failure", func(t *testing.T) {
		logger := NewEventLogger(t.TempDir())
		wantErr := errors.New("operation failed")

		err := logger.RunTimedOperation("Hotfix.BinaryOperation", func() (string, error) {
			return "route=package-manager outcome=failed target=202604.01.1", wantErr
		})

		require.ErrorIs(t, err, wantErr)
		events := logger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.Hotfix.BinaryOperation", events[0].TaskName)
		assert.Equal(t, string(EventLevelError), events[0].EventLevel)
		assert.Contains(t, events[0].Message, "route=package-manager")
		assert.Contains(t, events[0].Message, "outcome=failed")
		assert.Contains(t, events[0].Message, wantErr.Error())
		assert.Contains(t, events[0].Message, "durationMs=")
	})

	t.Run("nil logger still runs operation", func(t *testing.T) {
		var logger *EventLogger
		called := false

		err := logger.RunTimedOperation("Hotfix.BinaryOperation", func() (string, error) {
			called = true
			return "", nil
		})

		require.NoError(t, err)
		assert.True(t, called)
	})
}
