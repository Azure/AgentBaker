package service

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// Set conditions for program to run successfully wtih SetupConfig and CreateDataMaps functions
func SetupConfig() (*Config, error) {
	envVars := map[string]string{
		"kustoTable":            os.Getenv("BUILD_PERFORMANCE_TABLE_NAME"),
		"kustoEndpoint":         os.Getenv("BUILD_PERFORMANCE_KUSTO_ENDPOINT"),
		"kustoDatabase":         os.Getenv("BUILD_PERFORMANCE_DATABASE_NAME"),
		"kustoClientId":         os.Getenv("BUILD_PERFORMANCE_CLIENT_ID"),
		"kustoIngestionMapping": os.Getenv("BUILD_PERFORMANCE_INGESTION_MAPPING"),
		"sourceBranch":          os.Getenv("GIT_BRANCH"),
		"sigImageName":          os.Getenv("SIG_IMAGE_NAME"),
	}
	missingVar := false
	for name, value := range envVars {
		if value == "" {
			log.Printf("missing environment variable %q.", name)
			missingVar = true
		}
	}
	if missingVar {
		return nil, fmt.Errorf("required environment variables were not set")
	}

	return &Config{
		KustoTable:                envVars["kustoTable"],
		KustoEndpoint:             envVars["kustoEndpoint"],
		KustoDatabase:             envVars["kustoDatabase"],
		KustoClientId:             envVars["kustoClientId"],
		KustoIngestionMapping:     envVars["kustoIngestionMapping"],
		SigImageName:              envVars["sigImageName"],
		LocalBuildPerformanceFile: envVars["sigImageName"] + "-build-performance.json",
		SourceBranch:              envVars["sourceBranch"],
	}, nil
}

func CreateDataMaps() *DataMaps {
	return &DataMaps{
		// LocalPerformanceDataMap will hold the performance data from the local JSON file
		LocalPerformanceDataMap: EvaluationMap{},
		// QueriedPerformanceDataMap will hold the aggregated performance data queried from Kusto
		QueriedPerformanceDataMap: QueryMap{},
		// RegressionMap will hold all identified regressions in the current build
		RegressionMap: EvaluationMap{},
	}
}

// Prepare performance data for evaluation with PreparePerformanceDataForEvaluation and associated helper functions
func (maps *DataMaps) PreparePerformanceDataForEvaluation(localBuildPerformanceFile string, queriedData *SKU) error {
	if err := maps.DecodeLocalPerformanceData(localBuildPerformanceFile); err != nil {
		return fmt.Errorf("error decoding local performance data: %w", err)
	}
	if err := maps.ParseKustoData(queriedData); err != nil {
		return fmt.Errorf("error parsing kusto data: %w", err)
	}
	return nil
}

func (maps *DataMaps) DecodeLocalPerformanceData(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open local JSON file: %w", err)
	}
	defer file.Close()

	var m map[string]json.RawMessage
	err = json.NewDecoder(file).Decode(&m)
	if err != nil {
		return fmt.Errorf("error decoding local JSON file: %w", err)
	}

	key := "scripts"
	raw := m[key]

	holdingMap := HolderMap{}

	if err = json.Unmarshal(raw, &holdingMap); err != nil {
		return fmt.Errorf("error unmarshalling local JSON file into temporary holding map")
	}

	if err = maps.ConvertTimestampsToSeconds(holdingMap); err != nil {
		return fmt.Errorf("failed to convert timestamps to floats for evaluation: %w", err)
	}
	return nil
}

func (maps *DataMaps) ConvertTimestampsToSeconds(holdingMap HolderMap) error {
	for key, value := range holdingMap {
		script := map[string]float64{}
		for section, timeElapsed := range value {
			t, err := time.Parse("15:04:05", timeElapsed)
			if err != nil {
				return fmt.Errorf("error parsing timestamp in local build JSON data: %w", err)
			}
			totalSeconds := float64(t.Hour()*3600 + t.Minute()*60 + t.Second())
			script[section] = totalSeconds
		}
		maps.LocalPerformanceDataMap[key] = script
	}
	return nil
}

func (maps *DataMaps) ParseKustoData(data *SKU) error {
	data.SKUPerformanceData = data.CleanData()
	kustoData := []byte(data.SKUPerformanceData)
	if err := json.Unmarshal(kustoData, &maps.QueriedPerformanceDataMap); err != nil {
		return fmt.Errorf("error unmarshalling kusto data: %w", err)
	}
	return nil
}

func (sku *SKU) CleanData() string {
	return strings.ReplaceAll(sku.SKUPerformanceData, "NaN", "-1")
}

// After preparing performance data, evaluate it with EvaluatePerformance and associated helper functions
func (maps *DataMaps) EvaluatePerformance() error {
	// Iterate over LocalPerformanceDataMap and compare it against identical sections in QueriedPerformanceDataMap
	for scriptName, scriptData := range maps.LocalPerformanceDataMap {
		for section, timeElapsed := range scriptData {
			// The value of QueriedPerformanceDataMap[scriptName][section] is an array with two elements: [avg, 2*stdev]
			// First we check that the queried data contains this section
			sectionDataSlice, ok := maps.QueriedPerformanceDataMap[scriptName][section]
			if !ok {
				log.Printf("no data available for %s in %s\n", section, scriptName)
				continue
			}
			// Adding the slice values together gives us the maximum time allowed for the section
			maxTimeAllowed, err := SumSlice(sectionDataSlice)
			if err != nil {
				log.Printf("error calculating max time allowed for %s in %s: %v\n", section, scriptName, err)
				continue
			}
			if maxTimeAllowed == -1 {
				log.Printf("not enough data available for %s in %s\n", section, scriptName)
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
	if len(maps.RegressionMap) > 0 {
		log.Println("##vso[task.logissue type=warning;sourcepath=buildperformance;]Regressions listed below. Values are the excess time over 2 stdev above the mean")
		if err := maps.DisplayRegressions(); err != nil {
			return fmt.Errorf("error printing regressions: %w", err)
		}
		return nil
	}
	log.Println("\nNo regressions found for this pipeline run")
	return nil
}

func SumSlice(slice []float64) (float64, error) {
	var sum float64
	if len(slice) != 2 {
		return sum, fmt.Errorf("expected 2 elements in slice, got %d: %v", len(slice), slice)
	}
	if slice[0] == -1 || slice[1] == -1 {
		sum = -1
		return sum, nil
	}
	sum = slice[0] + slice[1]*2
	return sum, nil
}

func (maps DataMaps) DisplayRegressions() error {
	// Marshall JSON data and indent for better readability
	data, err := json.MarshalIndent(maps.RegressionMap, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling regression data: %w", err)
	}

	// Print JSON to the console for review by the user
	log.Println(string(data))
	return nil
}
