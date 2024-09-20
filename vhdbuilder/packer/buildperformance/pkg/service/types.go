package service

type Config struct {
	KustoTable                string
	KustoEndpoint             string
	KustoDatabase             string
	KustoClientId             string
	KustoIngestionMapping     string
	SigImageName              string
	LocalBuildPerformanceFile string
	SourceBranch              string
}

type HolderMap map[string]map[string]string

type QueryMap map[string]map[string][]float64

type EvaluationMap map[string]map[string]float64

type DataMaps struct {
	LocalPerformanceDataMap   EvaluationMap
	QueriedPerformanceDataMap QueryMap
	RegressionMap             EvaluationMap
}

type SKU struct {
	Name               string `kusto:"SIG_IMAGE_NAME"`
	SKUPerformanceData string `kusto:"BUILD_PERFORMANCE"`
}
