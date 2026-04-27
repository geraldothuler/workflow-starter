package dbops

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// QueryPostgres executes a SQL query against a PostgreSQL database and returns
// rows as a slice of string-keyed maps (JSON-serializable).
func QueryPostgres(creds *DBCredentials, query string) ([]map[string]any, error) {
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=require",
		creds.Host, creds.Port, creds.Database, creds.User, creds.Password)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		// Try without SSL — some internal RDS instances allow it
		dsn = fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
			creds.Host, creds.Port, creds.Database, creds.User, creds.Password)
		db2, err2 := sql.Open("postgres", dsn)
		if err2 != nil {
			return nil, fmt.Errorf("open (no-ssl): %w", err2)
		}
		defer db2.Close()
		if err2 = db2.Ping(); err2 != nil {
			return nil, fmt.Errorf("ping: %w (ssl), %w (no-ssl)", err, err2)
		}
		db = db2
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	return scanRows(rows)
}

// scanRows converts sql.Rows into []map[string]any.
func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, col := range cols {
			v := vals[i]
			// Convert []byte to string for readability
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[col] = v
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
