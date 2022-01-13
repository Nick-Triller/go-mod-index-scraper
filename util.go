package main

import "regexp"

// Pre-release regex pattern, see https://go.dev/ref/mod#glos-pre-release-version
const preReleasePattern = `^v\d+\.\d+.\d+-`

var preReleaseRegex *regexp.Regexp

func isPreRelease(version string) bool {
	// Compile regex on first invocation
	if preReleaseRegex == nil {
		regex, err := regexp.Compile(preReleasePattern)
		if err != nil {
			panic(err)
		}
		preReleaseRegex = regex
	}
	return preReleaseRegex.MatchString(version)
}
