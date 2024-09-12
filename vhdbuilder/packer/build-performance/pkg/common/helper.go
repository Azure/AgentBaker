package common

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// Setup program configuration in a concise manner
func SetupConfig() (*Config, error) {
	// Set env vars
	kustoTable := os.Getenv("BUILD_PERFORMANCE_TABLE_NAME")
	kustoEndpoint := os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME")
	kustoClientID := os.Getenv("BUILD_PERFORMANCE_CLIENT_ID")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	localBuildPerformanceFile := sigImageName + "-build-performance.json"
	sourceBranch := os.Getenv("GIT_BRANCH")

	// Check if all required environment variables are set
	missingVar := false
	for _, envVar := range []string{kustoTable, kustoEndpoint, kustoDatabase, kustoClientID, sigImageName, localBuildPerformanceFile, sourceBranch} {
		if envVar == "" {
			fmt.Printf("Missing environment variable \"%s\".", envVar)
			missingVar = true
		}
	}
	if missingVar {
		return nil, fmt.Errorf("required environment variables were not set")
	}

	return &Config{
		KustoTable:                sigImageName,
		KustoEndpoint:             kustoEndpoint,
		KustoDatabase:             kustoDatabase,
		KustoClientID:             kustoClientID,
		SigImageName:              sigImageName,
		LocalBuildPerformanceFile: localBuildPerformanceFile,
		SourceBranch:              sourceBranch,
	}, nil
}

// Encapsulate map creation in a function in order to keep main clean and readable
func CreateDataMaps() *DataMaps {
	return &DataMaps{
		// QueriedPerformanceDataMap will hold the aggregated performance data queried from Kusto
		QueriedPerformanceDataMap: make(map[string]map[string][]float64),
		// LocalPerformanceDataMap will hold the performance data from the local JSON file
		LocalPerformanceDataMap: make(map[string]map[string]float64),
		// HoldingMap is necessary after decoding the local JSON file, which still has a map[string]map[string]string structure
		HoldingMap: make(map[string]map[string]string),
		// RegressionMap will hold all identified regressions in the current build
		RegressionMap: make(map[string]map[string]float64),
	}
}

// Prepare local JSON data for evaluation
func DecodeVHDPerformanceData(filePath string, holdingMap map[string]map[string]string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Could not open %s", filePath)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	err = decoder.Decode(&holdingMap)
	if err != nil {
		log.Fatalf("Error decoding %s", filePath)
	}
}

// Put data in a new map with seconds instead of timestamps
func ConvertTimestampsToSeconds(holdingMap map[string]map[string]string, localBuildPerformanceData map[string]map[string]float64) {
	for key, value := range holdingMap {
		script := make(map[string]float64)
		for section, timeElapsed := range value {
			t, err := time.Parse("15:04:05", timeElapsed)
			if err != nil {
				log.Fatalf("Error parsing time in local build JSON data")
			}
			d := t.Sub(time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()))
			script[section] = d.Seconds()
		}
		localBuildPerformanceData[key] = script
	}
}

// Parse Kusto data
func ParseKustoData(data *SKU, queriedPerformanceData map[string]map[string][]float64) {
	kustoData := []byte(data.SKUPerformanceData)
	err := json.Unmarshal(kustoData, &queriedPerformanceData)
	if err != nil {
		log.Fatalf("Error parsing Kusto data")
	}
}

// Helper function for EvaluatePerformance
func SumArray(arr []float64) float64 {
	var sum float64
	if len(arr) != 2 {
		log.Fatalf("Expected 2 elements in array, got %d", len(arr))
	}
	for _, x := range arr {
		sum += x
	}
	return sum
}

// Evaluate performance data
func EvaluatePerformance(localPerformanceData map[string]map[string]float64, queriedPerformanceData map[string]map[string][]float64, regressions map[string]map[string]float64) map[string]map[string]float64 {
	// Iterate over localPerformanceData and compare it against identical sections in queriedPerformanceData
	for scriptName, scriptData := range localPerformanceData {
		for section, timeElapsed := range scriptData {
			// The value of queriedPerformanceData[scriptName][section] is an array with two elements: [avg, stdev]
			// Adding these together gives us the maximum time allowed for the section
			maxTimeAllowed := SumArray(queriedPerformanceData[scriptName][section])
			if timeElapsed > maxTimeAllowed {
				if regressions[scriptName] == nil {
					regressions[scriptName] = make(map[string]float64)
				}
				// Record the amount of time the section exceeded the maximum allowed time by
				regressions[scriptName][section] = timeElapsed - maxTimeAllowed
			}
		}
	}
	return regressions
}

// Print regressions identified during evaluation
func PrintRegressions(regressions map[string]map[string]float64) {
	prefix := ""
	indent := "  "

	data, err := json.MarshalIndent(regressions, prefix, indent)
	if err != nil {
		log.Fatalf("Error marshalling regression data")
	}

	fmt.Println(string(data))
}
