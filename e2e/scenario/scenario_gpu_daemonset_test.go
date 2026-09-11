package scenario

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunGPUCheck(t *testing.T) {
	checkErr := errors.New("driver unavailable")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "failure", err: checkErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &Scenario{}
			ctx := t.Context()
			calls := 0
			err := runGPUCheck(ctx, s, "driver/example", func(gotCtx context.Context, gotScenario *Scenario) error {
				calls++
				require.Equal(t, ctx, gotCtx)
				require.Same(t, s, gotScenario)
				return tc.err
			})
			require.Equal(t, 1, calls)
			require.Len(t, s.adoTestCases, 1)
			result := s.adoTestCases[0]
			require.Equal(t, "driver/example", result.Name)
			require.Equal(t, "e2e.gpu", result.ClassName)
			require.GreaterOrEqual(t, result.Duration.Nanoseconds(), int64(0))
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				require.Contains(t, err.Error(), "driver/example")
				require.Equal(t, tc.err.Error(), result.Message)
			} else {
				require.NoError(t, err)
				require.Empty(t, result.Message)
			}
		})
	}
}
