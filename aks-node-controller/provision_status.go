package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const truncatedProvisionOutput = "[...output truncated...]\n"

func provisionFailureDescription(result *ProvisionResult) (string, error) {
	status := cseStatusFromOutput(result.Output, result.ExitCode)
	if status == nil {
		status = map[string]any{
			"ExitCode": result.ExitCode,
			"Error":    result.Error,
			"Output":   result.Output,
		}
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal CSE failure: %w", err)
	}
	if healthDescriptionFits(string(payload)) {
		return string(payload), nil
	}

	output, _ := status["Output"].(string)
	status["Output"] = truncatedProvisionOutput
	status["OutputTruncated"] = true
	payload, err = json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal truncated CSE failure: %w", err)
	}
	if !healthDescriptionFits(string(payload)) {
		// Oversized metadata must not crowd out the final provisioning error.
		errorRunes := []rune(result.Error)
		status = map[string]any{
			"ExitCode":         result.ExitCode,
			"Error":            string(errorRunes[max(0, len(errorRunes)-256):]),
			"Output":           truncatedProvisionOutput,
			"OutputTruncated":  true,
			"DetailsTruncated": true,
		}
	}

	runes := []rune(output)
	low, high := 0, len(runes)
	for low < high {
		mid := low + (high-low+1)/2
		status["Output"] = truncatedProvisionOutput + string(runes[len(runes)-mid:])
		payload, err = json.Marshal(status)
		if err != nil {
			return "", fmt.Errorf("marshal truncated CSE failure: %w", err)
		}
		if healthDescriptionFits(string(payload)) {
			low = mid
		} else {
			high = mid - 1
		}
	}
	status["Output"] = truncatedProvisionOutput + string(runes[len(runes)-low:])
	payload, err = json.Marshal(status)
	if err != nil {
		return "", fmt.Errorf("marshal truncated CSE failure: %w", err)
	}
	if !healthDescriptionFits(string(payload)) {
		return "", fmt.Errorf("CSE failure metadata exceeds the %d-byte health description limit", healthDescriptionLimit)
	}
	return string(payload), nil
}

func cseStatusFromOutput(output, exitCode string) map[string]any {
	for offset := 0; offset < len(output); {
		start := strings.IndexByte(output[offset:], '{')
		if start < 0 {
			break
		}
		offset += start
		decoder := json.NewDecoder(strings.NewReader(output[offset:]))
		decoder.UseNumber()
		var status map[string]any
		if err := decoder.Decode(&status); err == nil {
			code, hasCode := status["ExitCode"].(string)
			_, hasOutput := status["Output"].(string)
			_, hasError := status["Error"].(string)
			if hasCode && code == exitCode && hasOutput && hasError {
				return status
			}
		}
		offset++
	}
	return nil
}
