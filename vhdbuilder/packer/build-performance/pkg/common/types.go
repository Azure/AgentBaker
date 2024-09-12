package common

type SKU struct {
	Name               string `kusto:"SIG_IMAGE_NAME"`
	SKUPerformanceData string `kusto:"BUILD_PERFORMANCE"`
}
