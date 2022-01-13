package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var indexClient = &http.Client{
	Timeout: 3 * time.Second,
}

func fetchFromIndexSince(since time.Time) ([]moduleVersion, error) {
	timestamp := since.Format(time.RFC3339)
	url := "https://index.golang.org/index?limit=" + strconv.Itoa(limit) + "&since=" + timestamp

	resp, err := indexClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected http status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var moduleVersions []moduleVersion
	for scanner.Scan() {
		var mv moduleVersion
		line := scanner.Bytes()
		err = json.Unmarshal(line, &mv)
		if err != nil {
			return nil, err
		}
		moduleVersions = append(moduleVersions, mv)
	}
	return moduleVersions, nil
}
