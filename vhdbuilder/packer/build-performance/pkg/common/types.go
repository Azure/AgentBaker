package common

type Config struct {
	KustoTable                string
	KustoEndpoint             string
	KustoDatabase             string
	KustoClientID             string
	SigImageName              string
	LocalBuildPerformanceFile string
	SourceBranch              string
}

type SKU struct {
	Name               string `kusto:"SIG_IMAGE_NAME"`
	SKUPerformanceData string `kusto:"BUILD_PERFORMANCE"`
}

type DataMaps struct {
	LocalPerformanceDataMap   map[string]map[string]float64
	QueriedPerformanceDataMap map[string]map[string][]float64
	HoldingMap                map[string]map[string]string
	RegressionMap             map[string]map[string]float64
}

//var err error
