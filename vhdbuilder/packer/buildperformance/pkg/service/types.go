package service

type Config struct {
	KustoTable                string
	KustoEndpoint             string
	KustoDatabase             string
	KustoIngestionMapping     string
	CommonIdentityId          string
	SigImageName              string
	LocalBuildPerformanceFile string
	SourceBranch              string
}

type SKU struct {
	Name               string `kusto:"SIG_IMAGE_NAME"`
	SKUPerformanceData string `kusto:"BUILD_PERFORMANCE"`
}

type DataMaps struct {
	LocalPerformanceDataMap   EvaluationMap
	QueriedPerformanceDataMap QueryMap
	RegressionMap             EvaluationMap
	StagingMap                StagingMap `json:"scripts"`
}

type StagingMap map[string]map[string]string

type QueryMap map[string]map[string][]float64

type EvaluationMap map[string]map[string]float64
