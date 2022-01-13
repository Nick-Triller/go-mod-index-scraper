package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"unicode"
)

// $module and $version elements are case-encoded by replacing every uppercase letter with an
// exclamation mark followed by the corresponding lower-case letter
func caseEncode(str string) string {
	var sb strings.Builder
	for _, char := range str {
		if unicode.IsUpper(char) && unicode.IsLetter(char) {
			sb.WriteRune('!')
			sb.WriteRune(unicode.ToLower(char))
		} else {
			sb.WriteRune(char)
		}
	}
	return sb.String()
}

func fetchGoModFile(path, version string, client *http.Client) (string, error) {
	proxyBaseUrl := "https://proxy.golang.org"
	// $base/$module/@v/$version.mod
	url := fmt.Sprintf("%s/%s/@v/%s.mod", proxyBaseUrl, caseEncode(path), caseEncode(version))

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		// Error is not expected
		panic(err)
	}
	// Non-standard header supported by google module proxy,
	// see https://go.dev/ref/mod#goproxy-protocol
	req.Header.Add("Disable-Module-Fetch", "true")
	req.Header.Set("User-Agent", "Go_Mod_Analytics/1.0 nicktriller@gmail.com")
	resp, err := client.Do(req)
	if err != nil {
		// TODO handle error, e.g. retry
		panic(err)
	}
	defer resp.Body.Close()
	// Check status code
	if resp.StatusCode == http.StatusGone {
		return "gone", nil
	} else if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch go.mod file, "+
			"unexpected status code %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), nil
}
