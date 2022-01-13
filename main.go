package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

const (
	// Max number of items that will be returned by the index API (max 2000)
	limit = 2000
	// Oldest timestamp supported by index API
	beginningOfTime = "2019-04-10T19:08:52.997264Z"
	// Timestamp in sqlite is stored in this format
	sqliteTimestampFormat = "2006-01-02 15:04:05.999999999-07:00"
	tableDDL              = `
CREATE TABLE IF NOT EXISTS moduleVersion (
  path TEXT not null,
  version TEXT not null,
  timestamp TEXT not null,
  isPreRelease BOOLEAN not null,
  goMod TEXT not null,
  PRIMARY KEY (path, version)
);
CREATE INDEX IF NOT EXISTS timestamp_idx ON moduleVersion (timestamp);
`
	dbFileName = "gomodindex.sqlite"
)

type moduleVersion struct {
	Path         string    `json:"Path"`
	Version      string    `json:"Version"`
	Timestamp    time.Time `json:"Timestamp"`
	isPreRelease bool
	goMod        string
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
	start := time.Now()
	moduleVersionChan := make(chan moduleVersion, limit*3)
	errChan := make(chan error)
	go scrapeAllModules(since, moduleVersionChan, errChan)
	enriched := make(chan moduleVersion, limit)
	numEnrichWorkers := 100
	log.Printf("Starting %d enrich workers", numEnrichWorkers)
	wg := &sync.WaitGroup{}
	wg.Add(numEnrichWorkers)
	for i := 0; i < numEnrichWorkers; i++ {
		go enrichWorker(moduleVersionChan, enriched, wg, errChan)
	}

	// Close chan once all workers finished
	go func() {
		wg.Wait()
		close(enriched)
	}()

	done := make(chan struct{})
	go store(enriched, done, errChan)

	select {
	case err := <-errChan:
		// Terminate on error in scraping or storing routines
		panic(err)
	case <-done:
		// Terminate if all items are stored
		log.Printf("Finished successfully after %s\n", time.Now().Sub(start))
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

func scrapeAllModules(initialSince time.Time, moduleVersionChan chan moduleVersion, errChan chan error) {
	log.Println("Begin scraping data")
	since := initialSince
	for {
		moduleVersions, err := fetchFromIndexSince(since)
		if err != nil {
			errChan <- err
			return
		}
		for _, mv := range moduleVersions {
			moduleVersionChan <- mv
		}
		if len(moduleVersions) < limit {
			// Found latest modules
			break
		}
		// Determine since for next request
		since = moduleVersions[len(moduleVersions)-1].Timestamp
	}

	close(moduleVersionChan)
}

func enrichWorker(moduleVersionChan chan moduleVersion, enriched chan moduleVersion, wg *sync.WaitGroup, errChan chan error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	for item := range moduleVersionChan {
		preRelease := isPreRelease(item.Version)
		item.isPreRelease = preRelease
		if !preRelease {
			goModFile, err := fetchGoModFile(item.Path, item.Version, client)
			if err != nil {
				errChan <- err
				return
			}
			item.goMod = goModFile
		}
		enriched <- item
	}
	wg.Done()
}

func store(moduleVersionChan chan moduleVersion, done chan struct{}, errChan chan error) {
	count := 0
	mustCommit := false
	// By default, every insert is one transaction. Insert everything in one transaction instead
	// because transactions are expensive.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		errChan <- err
		return
	}

	for item := range moduleVersionChan {
		query := "INSERT OR IGNORE INTO moduleVersion VALUES (?, ?, ?, ?, ?)"
		_, err := tx.Exec(query, item.Path, item.Version, item.Timestamp, item.isPreRelease, item.goMod)
		mustCommit = true
		if err != nil {
			errChan <- err
			return
		}
		count += 1
		// Log progress and commit
		if count%10000 == 0 {
			// Commit regularly to prevent data loss
			err := tx.Commit()
			if err != nil {
				log.Printf("Failed to commit transaction: %v", err)
			}
			log.Printf("Stored %d items\n", count)
			mustCommit = false
			tx, err = db.BeginTx(context.Background(), nil)
		}
	}

	// Commit last transaction
	if mustCommit {
		err := tx.Commit()
		if err != nil {
			log.Printf("Failed to commit transaction: %v", err)
		}
	}

	done <- struct{}{}
	close(done)
}
