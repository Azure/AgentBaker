package common

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func SetupConfig() (*Config, error) {
	kustoTable := os.Getenv("BUILD_PERFORMANCE_TABLE_NAME")
	kustoEndpoint := os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME")
	kustoClientID := os.Getenv("BUILD_PERFORMANCE_CLIENT_ID")
	kustoIngestionMapping := os.Getenv("BUILD_PERFORMANCE_INGESTION_MAPPING")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	localBuildPerformanceFile := sigImageName + "-build-performance.json"
	sourceBranch := os.Getenv("GIT_BRANCH")

	missingVar := false
	for _, envVar := range []string{kustoTable, kustoEndpoint, kustoDatabase, kustoClientID, sigImageName, localBuildPerformanceFile, sourceBranch, kustoIngestionMapping} {
		if envVar == "" {
			fmt.Printf("Missing environment variable \"%s\".", envVar)
			missingVar = true
		}
	}
	if missingVar {
		return nil, fmt.Errorf("required environment variables were not set")
	}

	return &Config{
		KustoTable:                kustoTable,
		KustoEndpoint:             kustoEndpoint,
		KustoDatabase:             kustoDatabase,
		KustoClientID:             kustoClientID,
		KustoIngestionMapping:     kustoIngestionMapping,
		SigImageName:              sigImageName,
		LocalBuildPerformanceFile: localBuildPerformanceFile,
		SourceBranch:              sourceBranch,
	}, nil
}

func CreateDataMaps() *DataMaps {
	return &DataMaps{
		// LocalPerformanceDataMap will hold the performance data from the local JSON file
		LocalPerformanceDataMap: make(map[string]map[string]float64),
		// QueriedPerformanceDataMap will hold the aggregated performance data queried from Kusto
		QueriedPerformanceDataMap: make(map[string]map[string][]float64),
		// RegressionMap will hold all identified regressions in the current build
		RegressionMap: make(map[string]map[string]float64),
	}
}

// Prepare local JSON data for evaluation
func (maps *DataMaps) DecodeLocalPerformanceData(filePath string) {

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("could not open %s", filePath)
	}
	defer file.Close()

	var m map[string]json.RawMessage
	err = json.NewDecoder(file).Decode(&m)
	if err != nil {
		log.Fatalf("error decoding %s", filePath)
	}

	key := "scripts"
	raw := m[key]

	holdingMap := map[string]map[string]string{}

	err = json.Unmarshal(raw, &holdingMap)
	if err != nil {
		log.Fatalf("error unmarshalling into temporary holding map")
	}

	maps.ConvertTimestampsToSeconds(holdingMap)
}

func (maps *DataMaps) ConvertTimestampsToSeconds(holdingMap map[string]map[string]string) {
	for key, value := range holdingMap {
		script := map[string]float64{}
		for section, timeElapsed := range value {
			t, err := time.Parse("15:04:05", timeElapsed)
			if err != nil {
				log.Fatalf("error parsing time in local build JSON data")
			}
			d := t.Sub(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()))
			script[section] = d.Seconds()
		}
		maps.LocalPerformanceDataMap[key] = script
	}
}

func (sku *SKU) CleanData() string {
	var auditedData string = strings.ReplaceAll(sku.SKUPerformanceData, "NaN", "-1")
	return auditedData
}

// Parse Kusto data
func (maps *DataMaps) ParseKustoData(data *SKU) {
	data.SKUPerformanceData = data.CleanData()
	kustoData := []byte(data.SKUPerformanceData)
	err := json.Unmarshal(kustoData, &maps.QueriedPerformanceDataMap)
	if err != nil {
		log.Fatalf(err.Error())
	}
}

func (maps *DataMaps) PreparePerformanceDataForEvaluation(localBuildPerformanceFile string, queriedData *SKU) {
	maps.DecodeLocalPerformanceData(localBuildPerformanceFile)
	maps.ParseKustoData(queriedData)
}

// Helper function for EvaluatePerformance
func SumArray(arr []float64) float64 {
	var sum float64
	if len(arr) != 2 {
		fmt.Printf("expected 2 elements in array, got %d", len(arr))
	}
	if arr[0] == -1 || arr[1] == -1 {
		return -1
	}
	sum = arr[0] + arr[1]*2
	return sum
}

// Evaluate performance data
func (maps *DataMaps) EvaluatePerformance() {
	// Iterate over LocalPerformanceDataMap and compare it against identical sections in QueriedPerformanceDataMap
	for scriptName, scriptData := range maps.LocalPerformanceDataMap {
		for section, timeElapsed := range scriptData {
			// The value of QueriedPerformanceDataMap[scriptName][section] is an array with two elements: [avg, stdev]
			// Adding these together gives us the maximum time allowed for the section
			maxTimeAllowed := SumArray(maps.QueriedPerformanceDataMap[scriptName][section])
			if maxTimeAllowed == -1 {
				fmt.Printf("No data available for %s in %s\n", section, scriptName)
				continue
			}
			if timeElapsed > maxTimeAllowed {
				if maps.RegressionMap[scriptName] == nil {
					maps.RegressionMap[scriptName] = map[string]float64{}
				}
				// Record the amount of time the section exceeded the maximum allowed time by
				maps.RegressionMap[scriptName][section] = timeElapsed - maxTimeAllowed
			}
		}
	}
	if len(maps.RegressionMap) == 0 {
		fmt.Printf("No regressions found for this pipeline run\n\n")
	} else {
		fmt.Printf("Regressions listed below. Section values represent the amount of time the section exceeded 1 stdev by.\n\n")
		maps.PrintRegressions()
	}
}

// Print regressions identified during evaluation
func (maps DataMaps) PrintRegressions() {
	prefix := ""
	indent := "  "

	data, err := json.MarshalIndent(maps.RegressionMap, prefix, indent)
	if err != nil {
		log.Fatalf("error marshalling regression data")
	}

	fmt.Println(string(data))
}
