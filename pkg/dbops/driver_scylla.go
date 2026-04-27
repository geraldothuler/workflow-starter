package dbops

import (
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

// QueryScylla executes a CQL query against a Scylla/Cassandra cluster and
// returns rows as a slice of string-keyed maps (JSON-serializable).
func QueryScylla(creds *DBCredentials, query string) ([]map[string]any, error) {
	contactPoints := strings.Split(creds.ContactPoints, ",")
	for i, cp := range contactPoints {
		contactPoints[i] = strings.TrimSpace(cp)
	}

	cluster := gocql.NewCluster(contactPoints...)
	cluster.Keyspace = creds.Keyspace
	cluster.Consistency = gocql.LocalQuorum
	cluster.Timeout = 15 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.NumConns = 2
	// Scylla 3.x requires explicit protocol version — gocql auto-discovery
	// sends an unversioned request before auth, which Scylla rejects.
	cluster.ProtoVersion = 4

	if creds.User != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: creds.User,
			Password: creds.Password,
		}
	}

	if creds.Datacenter != "" {
		cluster.PoolConfig.HostSelectionPolicy = gocql.DCAwareRoundRobinPolicy(creds.Datacenter)
	}

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("scylla connect: %w", err)
	}
	defer session.Close()

	iter := session.Query(query).Iter()
	return scanScyllaIter(iter)
}

func scanScyllaIter(iter *gocql.Iter) ([]map[string]any, error) {
	cols := iter.Columns()
	var result []map[string]any

	for {
		row := make(map[string]any, len(cols))
		if !iter.MapScan(row) {
			break
		}
		// Normalize types for JSON serialization
		for k, v := range row {
			switch t := v.(type) {
			case []byte:
				row[k] = string(t)
			case gocql.UUID:
				row[k] = t.String()
			}
		}
		result = append(result, row)
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scylla iter: %w", err)
	}
	return result, nil
}
