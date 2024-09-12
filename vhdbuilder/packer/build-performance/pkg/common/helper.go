package common

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func SetupConfig() (*Config, error) {
	kustoTable := os.Getenv("BUILD_PERFORMANCE_TABLE_NAME")
	kustoEndpoint := os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT")
	kustoDatabase := os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME")
	kustoClientID := os.Getenv("BUILD_PERFORMANCE_CLIENT_ID")
	sigImageName := os.Getenv("SIG_IMAGE_NAME")
	localBuildPerformanceFile := os.Getenv("LOCAL_BUILD_PERFORMANCE_FILE")
	sourceBranch := os.Getenv("GIT_BRANCH")

	missingVar := false
	for _, envVar := range []string{kustoTable, kustoEndpoint, kustoDatabase, kustoClientID, sigImageName, localBuildPerformanceFile, sourceBranch} {
		if envVar == "" {
			fmt.Println("Missing environment variable \"%s\".", envVar)
			missingVar = true
		}
	}
	if missingVar {
		return nil, fmt.Errorf("Required environment variables were not set.")
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

func CreateDataMaps() *DataMaps {
	return &DataMaps{
		LocalPerformanceDataMap:   make(map[string]map[string]float64),
		QueriedPerformanceDataMap: make(map[string]map[string][]float64),
		HoldingMap:                make(map[string]map[string]string),
		RegressionMap:             make(map[string]map[string]float64),
	}
}

// Prepare local JSON data for evaluation
func DecodeVHDPerformanceData(filePath string, holdingMap map[string]map[string]string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Could not open %s", filePath)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	err = decoder.Decode(&holdingMap)
	if err != nil {
		fmt.Printf("Error decoding %s", filePath)
	}
}

// Put data in new map with seconds instead of timestamps
func ConvertTimestampsToSeconds(holdingMap map[string]map[string]string, localBuildPerformanceData map[string]map[string]float64) {
	for key, value := range holdingMap {
		script := make(map[string]float64)
		for section, timeElapsed := range value {
			t, err := time.Parse("15:04:05", timeElapsed)
			if err != nil {
				fmt.Println("Error parsing time in local build JSON data")
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
		fmt.Println(err)
	}
}

// Helper function for EvaluatePerformance
func SumArray(arr []float64) float64 {
	var sum float64
	for _, x := range arr {
		sum += x
	}
	return sum
}

// Evaluate performance data
func EvaluatePerformance(localPerformanceData map[string]map[string]float64, queriedPerformanceData map[string]map[string][]float64, regressions map[string]map[string]float64) map[string]map[string]float64 {
	for scriptName, scriptData := range localPerformanceData {
		for section, timeElapsed := range scriptData {
			maxTimeAllowed := SumArray(queriedPerformanceData[scriptName][section])
			if timeElapsed > maxTimeAllowed {
				if regressions[scriptName] == nil {
					regressions[scriptName] = make(map[string]float64)
				}
				regressions[scriptName][section] = timeElapsed - maxTimeAllowed
			}
		}
	}
	return regressions
}

// Print regressions
func PrintRegressions(regressions map[string]map[string]float64) {
	prefix := ""
	indent := "  "

	data, err := json.MarshalIndent(regressions, prefix, indent)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(data))
}
