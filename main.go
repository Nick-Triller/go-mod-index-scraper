package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

const (
	// Max number of items that will be returned by the index API
	limit = 2000
	// Oldest timestamp supported by index API
	beginningOfTime = "2019-04-10T19:08:52.997264Z"
	// Timestamp in sqlite is stored in this format
	sqliteTimestampFormat = "2006-01-02 15:04:05.999999999-07:00"
	// Wait for some time after each scrape request
	scrapeDelay = 0 * time.Millisecond
	tableDDL    = `
CREATE TABLE IF NOT EXISTS moduleVersion (
  path TEXT not null,
  version TEXT not null,
  timestamp TEXT not null,
  isPreRelease BOOLEAN not null,
  PRIMARY KEY (path, version)
);
CREATE INDEX IF NOT EXISTS timestamp_idx ON moduleVersion (timestamp);
`
	dbFileName = "gomodindex.sqlite"
)

type moduleVersion struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Timestamp time.Time `json:"Timestamp"`
}

func main() {
	err := initDatabase()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	since := initialSince()
	log.Printf("Initial since value is %s\n", since)

	// Start scraping and storing routines
	batches := make(chan []moduleVersion, 10)
	errChan := make(chan error)
	go scrapeAllModules(since, batches, errChan)
	done := make(chan struct{})
	go store(batches, done, errChan)

	select {
	case err := <-errChan:
		// Terminate on error in scraping or storing routines
		panic(err)
	case <-done:
		// Terminate if all items are stored
		log.Println("Finished successfully")
		break
	}
}

func initDatabase() error {
	log.Println("Opening database")
	sqliteDB, err := sql.Open("sqlite3", dbFileName)
	if err != nil {
		return err
	}
	// Set global db variable
	db = sqliteDB
	// Create table
	sqlStmt := tableDDL
	log.Println("Ensure table exists")
	_, err = db.Exec(sqlStmt)
	return err
}

func initialSince() time.Time {
	// Read latest timestamp from DB if available
	res, err := db.Query("select timestamp from moduleVersion order by timestamp desc limit 1")
	if err != nil {
		panic(err)
	}
	var mostRecentTimestamp string
	if res.Next() {
		err := res.Scan(&mostRecentTimestamp)
		if err != nil {
			panic(err)
		}
		ts, err := time.Parse(sqliteTimestampFormat, mostRecentTimestamp)
		if err != nil {
			panic(err)
		}
		return ts
	}
	// Default to oldest available timestamp
	since, err := time.Parse(time.RFC3339Nano, beginningOfTime)
	if err != nil {
		panic(err)
	}
	return since
}

func scrapeAllModules(initialSince time.Time, batches chan []moduleVersion, errChan chan error) {
	log.Println("Begin scraping data")
	since := initialSince
	for {
		moduleVersions, err := fetchFromIndexSince(since)
		if err != nil {
			errChan <- err
			return
		}
		batches <- moduleVersions
		if len(moduleVersions) < limit {
			// Found latest modules
			break
		}
		// Determine since for next request
		since = moduleVersions[len(moduleVersions)-1].Timestamp
		// Throttle scraping
		time.Sleep(scrapeDelay)
	}

	close(batches)
}

func store(batches chan []moduleVersion, done chan struct{}, errChan chan error) {
	count := 0
	for batch := range batches {
		// Insert all items from batch at once
		valueStrings := make([]string, 0, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*4)
		for _, item := range batch {
			valueStrings = append(valueStrings, "(?, ?, ?, ?)")
			valueArgs = append(valueArgs, item.Path)
			valueArgs = append(valueArgs, item.Version)
			valueArgs = append(valueArgs, item.Timestamp)
			valueArgs = append(valueArgs, isPreRelease(item.Version))
		}
		query := fmt.Sprintf("INSERT OR IGNORE INTO moduleVersion VALUES %s", strings.Join(valueStrings, ","))
		_, err := db.Exec(query, valueArgs...)
		if err != nil {
			errChan <- err
			return
		}
		count += 1
		// Log progress
		if count%25 == 0 {
			log.Printf("Stored %d items\n", count*limit)
		}
	}

	done <- struct{}{}
	close(done)
}
