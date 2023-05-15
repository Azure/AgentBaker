package e2e_test

import (
	"fmt"
	"regexp"
)

const (
	/*
		this regex looks for groups of the following forms, returning KEY and VALUE as submatches
		- KEY=VALUE
		- KEY="VALUE"
		- KEY=
		- KEY="VALUE WITH WHITESPACE".
	*/
	keyValuePairRegexTemplate = `%s: (\"[^\"]*\"|[^\s]*)`
)

func extractKeyValuePair(key string, data string) (string, error) {
	regexString := fmt.Sprintf(keyValuePairRegexTemplate, key)

	r, err := regexp.Compile(regexString)
	if err != nil {
		return "", fmt.Errorf("failed to compile regex: %w", err)
	}

	matches := r.FindAllStringSubmatch(string(data), -1)

	if len(matches) < 1 || len(matches[0]) < 2 {
		return "", fmt.Errorf("expected 1 match with 1 submatch from regex, result %q", matches)
	}

	return matches[0][1], nil
}
