# Go module proxy index scraper

Scrapes all golang module versions indexed in the Google module proxy and
stores the results in a sqlite database.
The resulting database has a size of ~2 GB as of 01.01.2022.

The scraper can be stopped and started safely.
It will continue scraping where it was stopped.

## Implementation

index.golang.org is an index which serves a feed of new module versions that become
available by proxy.golang.org.
The list is sorted in chronological order. There are two optional parameters:
- 'since': the oldest allowable timestamp (RFC3339 format) for module versions in the returned list. Default is the
  beginning of time, e.g. https://index.golang.org/index?since=2019-04-10T19:08:52.997264Z
- 'limit': the maximum length of the returned list. Default = 2000, Max = 2000,
  e.g. https://index.golang.org/index?limit=10

Program logic:
1. Create database and tables if they don't exist.
2. Determine `since` value from most recent entry in sqlite DB or default to beginning of time.
3. Start goroutine that scrapes index continuously until the feed is fully consumed.
   Results are written to a channel. Result channel is closed once feed is fully consumed.
4. Start goroutine that stores the results into a sqlite DB continuously.
5. Exit program once scraping finished and all results were stored.

## References

- [https://go.dev/ref/mod](https://go.dev/ref/mod)
- [https://index.golang.org/](https://index.golang.org/)
