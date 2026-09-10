package e2e

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Skipped   int         `xml:"skipped,attr"`
	Time      string      `xml:"time,attr"`
	Timestamp string      `xml:"timestamp,attr,omitempty"`
	Cases     []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name       string        `xml:"name,attr"`
	Classname  string        `xml:"classname,attr"`
	Time       string        `xml:"time,attr"`
	Properties []junitProp   `xml:"properties>property,omitempty"`
	Failure    *junitFailure `xml:"failure,omitempty"`
	Skipped    *junitSkipped `xml:"skipped,omitempty"`
	SystemOut  string        `xml:"system-out,omitempty"`
}

type junitProp struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

func writeJUnitReport(path string, results []scenarioResult) error {
	if path == "" {
		return nil
	}
	results = append([]scenarioResult(nil), results...)
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	report := junitSuites{}
	suite := junitSuite{Name: "AgentBaker E2E", Timestamp: time.Now().UTC().Format(time.RFC3339)}
	var suiteDuration time.Duration
	for _, result := range results {
		testCase := junitCase{Name: result.Name, Classname: "e2e"}
		var total time.Duration
		for _, attempt := range result.Attempts {
			total += attempt.Duration
		}
		testCase.Time = fmt.Sprintf("%.6f", total.Seconds())
		testCase.Properties = append(testCase.Properties,
			junitProp{Name: "status", Value: string(result.Status)},
			junitProp{Name: "attempts", Value: fmt.Sprint(len(result.Attempts))},
		)
		last := reportedAttempt(result)
		hasAttachment := false
		if result.Status == statusFailed || result.Status == statusFlaky {
			for _, attempt := range result.Attempts {
				if attempt.LogPath == "" {
					continue
				}
				absolute, err := filepath.Abs(attempt.LogPath)
				if err != nil {
					absolute = attempt.LogPath
				}
				testCase.SystemOut += fmt.Sprintf("[[ATTACHMENT|%s]]\n", absolute)
				hasAttachment = true
			}
		}
		switch result.Status {
		case statusFailed:
			testCase.Failure = junitFailureSummary(last.Message, hasAttachment)
		case statusSkipped:
			testCase.Skipped = &junitSkipped{Message: summaryMessage(last.Message)}
		}
		suite.Cases = append(suite.Cases, testCase)
		suiteDuration += total
		switch result.Status {
		case statusFailed:
			suite.Failures++
		case statusSkipped:
			suite.Skipped++
		}
		for _, adoTestCase := range last.ADOTestCases {
			testCase := junitCase{
				Name:      result.Name + "/" + adoTestCase.Name,
				Classname: adoTestCase.ClassName,
				Time:      fmt.Sprintf("%.6f", adoTestCase.Duration.Seconds()),
			}
			if adoTestCase.Message != "" {
				testCase.Failure = junitFailureSummary(adoTestCase.Message, false)
				suite.Failures++
			}
			// Exported case durations are already included in the scenario attempt.
			suite.Cases = append(suite.Cases, testCase)
		}
	}
	suite.Tests = len(suite.Cases)
	suite.Time = fmt.Sprintf("%.6f", suiteDuration.Seconds())
	report.Tests = suite.Tests
	report.Failures = suite.Failures
	report.Skipped = suite.Skipped
	report.Time = suite.Time
	report.Suites = []junitSuite{suite}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("create JUnit directory: %w", err)
	}
	data, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JUnit report: %w", err)
	}
	data = append([]byte(xml.Header), data...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write JUnit report: %w", err)
	}
	return nil
}

func reportedAttempt(result scenarioResult) attemptResult {
	last := result.Attempts[len(result.Attempts)-1]
	if result.Status != statusFailed {
		return last
	}
	for i := len(result.Attempts) - 1; i >= 0; i-- {
		if result.Attempts[i].Status == statusFailed {
			return result.Attempts[i]
		}
	}
	return last
}

func junitFailureSummary(message string, attached bool) *junitFailure {
	summary := summaryMessage(message)
	body := summary
	if attached {
		body += "\n\nFull output is available in the attached scenario log."
	}
	return &junitFailure{Message: summary, Body: body}
}
