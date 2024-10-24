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
		"commonIdentityId":      os.Getenv("AZURE_MSI_RESOURCE_STRING"),
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

	log.Println("Program config set")

	return &Config{
		KustoTable:                envVars["kustoTable"],
		KustoEndpoint:             envVars["kustoEndpoint"],
		KustoDatabase:             envVars["kustoDatabase"],
		CommonIdentityId:          envVars["commonIdentityId"],
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
		// StagingMap is a temporary holding map that will hold the data from the local JSON file before it is converted to floats
		StagingMap: StagingMap{},
	}
}

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
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	if err = json.Unmarshal(data, &maps); err != nil {
		return fmt.Errorf("error unmarshaling JSON to temoporary map: %w", err)
	}

	if err = maps.ConvertTimestampsToSeconds(maps.StagingMap); err != nil {
		return fmt.Errorf("failed to convert timestamps to floats for evaluation: %w", err)
	}

	return nil
}

func (maps *DataMaps) ConvertTimestampsToSeconds(holdingMap StagingMap) error {
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

func (maps *DataMaps) EvaluatePerformance() error {
	// Iterate over LocalPerformanceDataMap and compare it against identical sections in QueriedPerformanceDataMap
	// The value of QueriedPerformanceDataMap[scriptName][section] is an array with two elements: [avg, 3*stdev]
	for scriptName, scriptData := range maps.LocalPerformanceDataMap {
		for section, timeElapsed := range scriptData {
			sectionDataSlice, ok := maps.QueriedPerformanceDataMap[scriptName][section]
			if !ok {
				log.Printf("no data available for %s in %s\n", section, scriptName)
				continue
			}
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

	if err := maps.CheckRegressionsMap(); err != nil {
		return fmt.Errorf("error checking regression map: %w", err)
	}

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

	// Adding the slice values together gives us the maximum time allowed for the section
	sum = slice[0] + slice[1]*3

	return sum, nil
}

func (maps *DataMaps) CheckRegressionsMap() error {
	if len(maps.RegressionMap) > 0 {
		message := fmt.Sprintln("Regressions listed below. Values are listed in seconds and represent the excess time over 3 stdev above the mean")
		log.Printf("##vso[task.logissue type=warning;sourcepath=buildperformance;]%s", message)
		if err := maps.DisplayRegressions(); err != nil {
			return fmt.Errorf("error printing regressions: %w", err)
		}
	} else {
		log.Println("No regressions found for this pipeline run")
	}

	return nil
}

func (maps DataMaps) DisplayRegressions() error {
	// Marshall JSON data and indent for better readability
	data, err := json.MarshalIndent(maps.RegressionMap, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling regression data: %w", err)
	}

	// Log regressions
	log.Println(string(data))

	for script, section := range maps.RegressionMap {
		for sectionName := range section {
			queriedData := maps.QueriedPerformanceDataMap[script][sectionName]
			fmt.Printf("\nRegression detected: %s\n", sectionName)
			fmt.Printf("     Average duration: %f seconds, Standard deviation: %f seconds, Duration for this pipeline run: %f seconds\n",
				queriedData[0], // Average duration of section
				queriedData[1], // Standard Deviation of section
				maps.LocalPerformanceDataMap[script][sectionName], // Duration of section for this pipeline run
			)
		}
	}

	return nil
}
