// Package sqlite wires modernc.org/sqlite (pure-Go) into database/sql for
// NextChapter. The only thing it does beyond `sql.Open` is enforce that
// every connection enables foreign-key checks; SQLite ships them off by
// default and ADR-0005 relies on ON DELETE CASCADE.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	// Register the modernc.org/sqlite driver as "sqlite".
	_ "modernc.org/sqlite"
)

// Open parses a sqlite:// URL and returns a configured *sql.DB. The path
// after the scheme is taken verbatim as the SQLite filename. The special
// path ":memory:" is honoured.
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	path, qs, err := parseURL(databaseURL)
	if err != nil {
		return nil, err
	}

	// modernc supports `_pragma=` DSN parameters that are executed on
	// every connection. We always set foreign_keys=on; callers may layer
	// extra pragmas via the query string.
	values := qs
	pragmas := values["_pragma"]
	if !hasPragma(pragmas, "foreign_keys") {
		pragmas = append(pragmas, "foreign_keys(on)")
	}
	if !hasPragma(pragmas, "busy_timeout") {
		pragmas = append(pragmas, "busy_timeout(5000)")
	}
	if !hasPragma(pragmas, "journal_mode") && path != ":memory:" {
		pragmas = append(pragmas, "journal_mode(WAL)")
	}
	values["_pragma"] = pragmas

	dsn := path
	if encoded := values.Encode(); encoded != "" {
		dsn = path + "?" + encoded
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", path, err)
	}
	// Pure-Go SQLite is single-writer; serialise writes.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: ping %q: %w", path, err)
	}
	return db, nil
}

func parseURL(databaseURL string) (string, url.Values, error) {
	const prefix = "sqlite://"
	if !strings.HasPrefix(databaseURL, prefix) {
		return "", nil, fmt.Errorf("sqlite: URL must start with %q", prefix)
	}
	rest := strings.TrimPrefix(databaseURL, prefix)

	pathPart, queryPart, _ := strings.Cut(rest, "?")
	if pathPart == "" {
		return "", nil, fmt.Errorf("sqlite: empty path in %q", databaseURL)
	}

	var values url.Values
	if queryPart == "" {
		values = url.Values{}
	} else {
		v, err := url.ParseQuery(queryPart)
		if err != nil {
			return "", nil, fmt.Errorf("sqlite: parse query %q: %w", queryPart, err)
		}
		values = v
	}
	return pathPart, values, nil
}

func hasPragma(pragmas []string, name string) bool {
	for _, p := range pragmas {
		if strings.HasPrefix(strings.ToLower(p), name) {
			return true
		}
	}
	return false
}
