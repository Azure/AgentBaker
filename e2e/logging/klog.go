package logging

import (
	"context"

	"github.com/go-logr/logr/funcr"
	"k8s.io/klog/v2"
)

func withKlogLogger(ctx context.Context) context.Context {
	logger := FromContext(ctx)
	structured := funcr.New(func(prefix, message string) {
		if prefix != "" {
			logger.Logf("%s: %s", prefix, message)
		} else {
			logger.Log(message)
		}
	}, funcr.Options{})
	return klog.NewContext(ctx, structured)
}
