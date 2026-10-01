package logging

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func TestKlogMessagesStayWithScenario(t *testing.T) {
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			output := &stateLogger{}
			ctx, cancel := context.WithCancel(WithLogger(t.Context(), output))
			cancel()

			logger := klog.FromContext(ctx)
			runtime.HandleErrorWithLogger(logger, net.ErrClosed, "Copying stderr failed")
			runtime.HandleErrorWithLogger(logger, net.ErrClosed, "Copying stdout failed")
			logger.Error(net.ErrClosed, "Websocket Ping failed")
			rest.WarningLogger{}.HandleWarningHeaderWithContext(ctx, 299, "", "API warning for "+name)

			require.Len(t, output.logs, 4)
			for _, message := range output.logs[:3] {
				assert.Contains(t, message, net.ErrClosed.Error())
			}
			assert.Contains(t, output.logs[0], "Copying stderr failed")
			assert.Contains(t, output.logs[1], "Copying stdout failed")
			assert.Contains(t, output.logs[2], "Websocket Ping failed")
			assert.Contains(t, output.logs[3], "API warning for "+name)
			assert.ErrorIs(t, ctx.Err(), context.Canceled)
		})
	}
}

func TestKlogPreservesNamesAndValues(t *testing.T) {
	output := &stateLogger{}
	ctx := WithLogger(t.Context(), output)
	logger := klog.FromContext(ctx).WithName("stream").WithValues("id", 2, "pod", "test")
	logger.Info("connected")
	logger.V(5).Info("verbose trace")

	require.Len(t, output.logs, 1)
	assert.True(t, strings.HasPrefix(output.logs[0], "stream: "))
	assert.Contains(t, output.logs[0], `"id"=2`)
	assert.Contains(t, output.logs[0], `"pod"="test"`)
}

func TestKlogOperationFieldsDoNotChangeScenarioContext(t *testing.T) {
	output := &stateLogger{}
	ctx := WithLogger(t.Context(), output)
	execCtx := klog.NewContext(ctx, klog.FromContext(ctx).WithValues("operation", "pod exec", "namespace", "test", "pod", "example"))

	klog.FromContext(execCtx).Error(net.ErrClosed, "Copying stderr failed")
	klog.FromContext(ctx).Info("another request")

	require.Len(t, output.logs, 2)
	assert.Contains(t, output.logs[0], `"operation"="pod exec"`)
	assert.Contains(t, output.logs[0], `"namespace"="test"`)
	assert.Contains(t, output.logs[0], `"pod"="example"`)
	assert.NotContains(t, output.logs[1], `"operation"`)
	assert.NotContains(t, output.logs[1], `"namespace"`)
	assert.NotContains(t, output.logs[1], `"pod"`)
}
