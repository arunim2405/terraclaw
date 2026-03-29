// Package steampipe provides a client for querying cloud resources via Steampipe.
package steampipe

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Resource represents a discovered cloud resource.
type Resource struct {
	Provider   string
	Service    string
	Type       string
	Name       string
	ID         string
	Region     string
	Properties map[string]string
}

// String returns a human-readable representation of the resource.
func (r Resource) String() string {
	return fmt.Sprintf("[%s] %s/%s: %s (%s)", r.Region, r.Provider, r.Type, r.Name, r.ID)
}

// Client wraps a Steampipe (PostgreSQL) database connection.
type Client struct {
	db *sql.DB
}

// New creates a new Steampipe client using the given connection string.
func New(connStr string) (*Client, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("open steampipe connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping steampipe: %w", err)
	}
	return &Client{db: db}, nil
}

// Close closes the underlying database connection.
func (c *Client) Close() error {
	return c.db.Close()
}

// ListSchemas returns the list of available plugin schemas (cloud providers).
func (c *Client) ListSchemas() ([]string, error) {
	rows, err := c.db.Query(`
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog','information_schema','steampipe_internal','public')
		ORDER BY schema_name
	`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, name)
	}
	return schemas, rows.Err()
}

// ListTables returns the tables (resource types) available in a given schema.
func (c *Client) ListTables(schema string) ([]string, error) {
	rows, err := c.db.Query(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = $1
		ORDER BY table_name
	`, schema)
	if err != nil {
		return nil, fmt.Errorf("list tables for schema %q: %w", schema, err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// FetchResources queries a specific resource table and returns discovered resources.
func (c *Client) FetchResources(schema, table string) ([]Resource, error) {
	// Try to get common columns; fall back gracefully if they don't exist.
	query := fmt.Sprintf(`SELECT * FROM %s.%s LIMIT 200`, quoteIdent(schema), quoteIdent(table))
	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("fetch resources from %s.%s: %w", schema, table, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var resources []Resource
	for rows.Next() {
		// Scan all columns as raw bytes.
		rawValues := make([]interface{}, len(cols))
		scanArgs := make([]interface{}, len(cols))
		for i := range rawValues {
			scanArgs[i] = &rawValues[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}

		props := make(map[string]string, len(cols))
		for i, col := range cols {
			if rawValues[i] != nil {
				props[col] = fmt.Sprintf("%s", rawValues[i])
			}
		}

		r := Resource{
			Provider:   schema,
			Service:    schema,
			Type:       table,
			Properties: props,
		}
		r.Name = pickFirst(props, "name", "title", "id", "arn")
		r.ID = pickFirst(props, "id", "arn", "name", "title")
		r.Region = pickFirst(props, "region", "location", "zone")

		resources = append(resources, r)
	}
	return resources, rows.Err()
}

// quoteIdent safely quotes a PostgreSQL identifier.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// pickFirst returns the value of the first key found in the map, or empty string.
func pickFirst(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// FetchResourcesByARNs looks up specific resources by their ARNs across the
// appropriate Steampipe tables. It uses the ARN service mapping to determine
// which tables to query, avoiding a full scan.
func (c *Client) FetchResourcesByARNs(schema string, arns []string) ([]Resource, error) {
	var allResources []Resource

	for _, arn := range arns {
		tables := TableNamesForARN(arn)
		if len(tables) == 0 {
			return nil, fmt.Errorf("cannot determine Steampipe table for ARN: %s", arn)
		}

		found := false
		for _, table := range tables {
			query := fmt.Sprintf(
				`SELECT * FROM %s.%s WHERE arn = $1 LIMIT 1`,
				quoteIdent(schema), quoteIdent(table),
			)
			rows, err := c.db.Query(query, arn)
			if err != nil {
				// Table might not exist for this plugin installation; skip it.
				continue
			}

			cols, err := rows.Columns()
			if err != nil {
				rows.Close()
				continue
			}

			for rows.Next() {
				rawValues := make([]interface{}, len(cols))
				scanArgs := make([]interface{}, len(cols))
				for i := range rawValues {
					scanArgs[i] = &rawValues[i]
				}
				if err := rows.Scan(scanArgs...); err != nil {
					rows.Close()
					continue
				}

				props := make(map[string]string, len(cols))
				for i, col := range cols {
					if rawValues[i] != nil {
						props[col] = fmt.Sprintf("%s", rawValues[i])
					}
				}

				r := Resource{
					Provider:   schema,
					Service:    schema,
					Type:       table,
					Properties: props,
				}
				r.Name = pickFirst(props, "name", "title", "id", "arn")
				r.ID = pickFirst(props, "id", "arn", "name", "title")
				r.Region = pickFirst(props, "region", "location", "zone")

				allResources = append(allResources, r)
				found = true
			}
			rows.Close()

			if found {
				break // Found the resource in this table, no need to check others.
			}
		}

		if !found {
			return nil, fmt.Errorf("resource not found for ARN: %s", arn)
		}
	}

	return allResources, nil
}
