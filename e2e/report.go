package e2e

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
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

func (e *executor) writeReports() error {
	if e.opts.junitFile == "" {
		return nil
	}
	e.resultsMu.Lock()
	results := append([]scenarioResult(nil), e.results...)
	e.resultsMu.Unlock()
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
		last := result.Attempts[len(result.Attempts)-1]
		switch result.Status {
		case statusFailed:
			testCase.Failure = &junitFailure{Message: concise(last.Message), Body: concise(last.Message)}
		case statusSkipped:
			testCase.Skipped = &junitSkipped{Message: concise(last.Message)}
		}
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
			}
		}
		suite.Cases = append(suite.Cases, testCase)
		suiteDuration += total
		switch result.Status {
		case statusFailed:
			suite.Failures++
		case statusSkipped:
			suite.Skipped++
		}
		for _, check := range last.Checks {
			checkCase := junitCase{
				Name:      result.Name + "/" + check.Name,
				Classname: "e2e.cse",
				Time:      fmt.Sprintf("%.6f", check.Duration.Seconds()),
			}
			if check.Message != "" {
				checkCase.Failure = &junitFailure{Message: concise(check.Message), Body: concise(check.Message)}
				suite.Failures++
			}
			// Child durations are already included in the parent attempt.
			suite.Cases = append(suite.Cases, checkCase)
		}
	}
	suite.Tests = len(suite.Cases)
	suite.Time = fmt.Sprintf("%.6f", suiteDuration.Seconds())
	report.Tests = suite.Tests
	report.Failures = suite.Failures
	report.Skipped = suite.Skipped
	report.Time = suite.Time
	report.Suites = []junitSuite{suite}

	if err := os.MkdirAll(filepath.Dir(e.opts.junitFile), 0o755); err != nil && filepath.Dir(e.opts.junitFile) != "." {
		return fmt.Errorf("create JUnit directory: %w", err)
	}
	data, err := xml.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JUnit report: %w", err)
	}
	data = append([]byte(xml.Header), data...)
	if err := os.WriteFile(e.opts.junitFile, data, 0o600); err != nil {
		return fmt.Errorf("write JUnit report: %w", err)
	}
	return nil
}

func concise(message string) string {
	const max = 4096
	message = strings.TrimSpace(message)
	if len(message) <= max {
		return message
	}
	start := len(message) - max
	for start < len(message) && !utf8.RuneStart(message[start]) {
		start++
	}
	return "... beginning truncated; see attached scenario log\n" + message[start:]
}
