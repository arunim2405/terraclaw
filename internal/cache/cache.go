// Package cache provides SQLite-based caching for Steampipe resource findings.
// It persists discovered resources and graph edges between terraclaw runs,
// avoiding repeated cloud API calls via Steampipe.
package cache

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver

	"github.com/arunim2405/terraclaw/internal/graph"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// Store wraps a SQLite database for caching scan results.
type Store struct {
	db *sql.DB
}

// ScanInfo describes a cached scan.
type ScanInfo struct {
	ID         int64
	Schema     string
	ScanMode   string
	Tables     []string
	StartedAt  time.Time
	FinishedAt time.Time
	Stats      graph.ScanStats
}

// Open opens (or creates) the SQLite cache database at the given path.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cache db: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate cache schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates or updates the cache schema.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS scans (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			schema      TEXT NOT NULL,
			scan_mode   TEXT NOT NULL,
			tables_json TEXT NOT NULL,
			started_at  TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			stats_json  TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS resources (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_id     INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
			provider    TEXT NOT NULL,
			service     TEXT NOT NULL,
			type        TEXT NOT NULL,
			name        TEXT,
			resource_id TEXT,
			region      TEXT,
			properties  TEXT NOT NULL,
			UNIQUE(scan_id, type, resource_id)
		);

		CREATE INDEX IF NOT EXISTS idx_resources_scan_id ON resources(scan_id);
		CREATE INDEX IF NOT EXISTS idx_resources_type ON resources(type);
		CREATE INDEX IF NOT EXISTS idx_resources_resource_id ON resources(resource_id);

		CREATE TABLE IF NOT EXISTS edges (
			scan_id    INTEGER NOT NULL REFERENCES scans(id) ON DELETE CASCADE,
			source_key TEXT NOT NULL,
			target_key TEXT NOT NULL,
			PRIMARY KEY (scan_id, source_key, target_key)
		);

		CREATE INDEX IF NOT EXISTS idx_edges_scan_id ON edges(scan_id);

		PRAGMA foreign_keys = ON;
	`)
	return err
}

// LatestScan returns the most recent scan for the given schema and scan mode,
// or nil if no cached scan exists.
func (s *Store) LatestScan(schema, scanMode string) (*ScanInfo, error) {
	row := s.db.QueryRow(`
		SELECT id, schema, scan_mode, tables_json, started_at, finished_at, stats_json
		FROM scans
		WHERE schema = ? AND scan_mode = ?
		ORDER BY finished_at DESC
		LIMIT 1
	`, schema, scanMode)

	var info ScanInfo
	var tablesJSON, statsJSON, startedStr, finishedStr string
	err := row.Scan(&info.ID, &info.Schema, &info.ScanMode, &tablesJSON, &startedStr, &finishedStr, &statsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest scan: %w", err)
	}

	if err := json.Unmarshal([]byte(tablesJSON), &info.Tables); err != nil {
		return nil, fmt.Errorf("unmarshal tables: %w", err)
	}
	if err := json.Unmarshal([]byte(statsJSON), &info.Stats); err != nil {
		return nil, fmt.Errorf("unmarshal stats: %w", err)
	}

	info.StartedAt, _ = time.Parse(time.RFC3339, startedStr)
	info.FinishedAt, _ = time.Parse(time.RFC3339, finishedStr)

	return &info, nil
}

// SaveGraph persists a scanned graph and its metadata to the cache.
func (s *Store) SaveGraph(schema, scanMode string, tables []string, g *graph.Graph) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	tablesJSON, _ := json.Marshal(tables)
	statsJSON, _ := json.Marshal(g.Stats)
	now := time.Now().UTC().Format(time.RFC3339)

	res, err := tx.Exec(`
		INSERT INTO scans (schema, scan_mode, tables_json, started_at, finished_at, stats_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, schema, scanMode, string(tablesJSON), now, now, string(statsJSON))
	if err != nil {
		return fmt.Errorf("insert scan: %w", err)
	}

	scanID, _ := res.LastInsertId()

	// Insert resources.
	resStmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO resources (scan_id, provider, service, type, name, resource_id, region, properties)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare resource insert: %w", err)
	}
	defer resStmt.Close()

	for _, node := range g.Nodes {
		r := node.Resource
		propsJSON, _ := json.Marshal(r.Properties)
		if _, err := resStmt.Exec(scanID, r.Provider, r.Service, r.Type, r.Name, r.ID, r.Region, string(propsJSON)); err != nil {
			return fmt.Errorf("insert resource %s: %w", r.ID, err)
		}
	}

	// Insert edges.
	edgeStmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO edges (scan_id, source_key, target_key)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare edge insert: %w", err)
	}
	defer edgeStmt.Close()

	for key, node := range g.Nodes {
		for edgeKey := range node.Edges {
			if _, err := edgeStmt.Exec(scanID, key, edgeKey); err != nil {
				return fmt.Errorf("insert edge %s→%s: %w", key, edgeKey, err)
			}
		}
	}

	return tx.Commit()
}

// LoadGraph reconstructs a graph.Graph from a cached scan.
func (s *Store) LoadGraph(scanID int64) (*graph.Graph, error) {
	g := graph.New()

	// Load resources.
	rows, err := s.db.Query(`
		SELECT provider, service, type, name, resource_id, region, properties
		FROM resources
		WHERE scan_id = ?
	`, scanID)
	if err != nil {
		return nil, fmt.Errorf("query resources: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r steampipe.Resource
		var propsJSON string
		var name, resourceID, region sql.NullString

		if err := rows.Scan(&r.Provider, &r.Service, &r.Type, &name, &resourceID, &region, &propsJSON); err != nil {
			return nil, fmt.Errorf("scan resource row: %w", err)
		}

		r.Name = name.String
		r.ID = resourceID.String
		r.Region = region.String

		if err := json.Unmarshal([]byte(propsJSON), &r.Properties); err != nil {
			return nil, fmt.Errorf("unmarshal properties: %w", err)
		}

		g.AddNode(r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load edges.
	edgeRows, err := s.db.Query(`
		SELECT source_key, target_key
		FROM edges
		WHERE scan_id = ?
	`, scanID)
	if err != nil {
		return nil, fmt.Errorf("query edges: %w", err)
	}
	defer edgeRows.Close()

	edgeCount := 0
	for edgeRows.Next() {
		var src, dst string
		if err := edgeRows.Scan(&src, &dst); err != nil {
			return nil, fmt.Errorf("scan edge row: %w", err)
		}
		g.AddEdge(src, dst)
		edgeCount++
	}
	if err := edgeRows.Err(); err != nil {
		return nil, err
	}

	g.Stats.ResourceCount = len(g.Nodes)
	g.Stats.EdgeCount = edgeCount

	return g, nil
}

// DeleteScan removes a scan and its associated resources and edges.
func (s *Store) DeleteScan(scanID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete in order: edges → resources → scan (foreign keys).
	if _, err := tx.Exec("DELETE FROM edges WHERE scan_id = ?", scanID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM resources WHERE scan_id = ?", scanID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM scans WHERE id = ?", scanID); err != nil {
		return err
	}

	return tx.Commit()
}

// Purge removes all scans older than the given duration.
func (s *Store) Purge(maxAge time.Duration) error {
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	_, err := s.db.Exec("DELETE FROM scans WHERE finished_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("purge old scans: %w", err)
	}
	// Clean orphaned resources/edges.
	_, _ = s.db.Exec("DELETE FROM resources WHERE scan_id NOT IN (SELECT id FROM scans)")
	_, _ = s.db.Exec("DELETE FROM edges WHERE scan_id NOT IN (SELECT id FROM scans)")
	return nil
}
