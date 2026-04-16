package modules

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// Store wraps a SQLite database for persisting user-registered module metadata.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the modules SQLite database at the given path.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		return nil, fmt.Errorf("create modules db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open modules db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate modules schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS user_modules (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			name          TEXT    NOT NULL UNIQUE,
			source        TEXT    NOT NULL,
			description   TEXT    NOT NULL DEFAULT '',
			provider_type TEXT    NOT NULL DEFAULT '',
			metadata_json TEXT    NOT NULL,
			created_at    TEXT    NOT NULL,
			updated_at    TEXT    NOT NULL
		);

		CREATE TABLE IF NOT EXISTS user_module_resources (
			module_id     INTEGER NOT NULL REFERENCES user_modules(id) ON DELETE CASCADE,
			resource_type TEXT    NOT NULL,
			PRIMARY KEY (module_id, resource_type)
		);

		CREATE INDEX IF NOT EXISTS idx_umr_type
			ON user_module_resources(resource_type);
	`)
	return err
}

// moduleRow is the JSON-serializable form stored in metadata_json.
type moduleRow struct {
	ResourceTypes []string       `json:"resource_types"`
	DataSources   []string       `json:"data_sources"`
	Variables     []VariableMeta `json:"variables"`
	Outputs       []OutputMeta   `json:"outputs"`
}

// SaveModule inserts or replaces a module in the database. On conflict by name
// the existing row is updated.
func (s *Store) SaveModule(m *ModuleMetadata) error {
	row := moduleRow{
		ResourceTypes: m.ResourceTypes,
		DataSources:   m.DataSources,
		Variables:     m.Variables,
		Outputs:       m.Outputs,
	}
	metaJSON, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("marshal module metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert the module row.
	res, err := tx.Exec(`
		INSERT INTO user_modules (name, source, description, provider_type, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			source        = excluded.source,
			description   = excluded.description,
			provider_type = excluded.provider_type,
			metadata_json = excluded.metadata_json,
			updated_at    = excluded.updated_at
	`, m.Name, m.Source, m.Description, m.ProviderType, string(metaJSON), now, now)
	if err != nil {
		return fmt.Errorf("upsert module: %w", err)
	}

	// Get the module ID (from insert or existing row).
	moduleID, err := res.LastInsertId()
	if err != nil || moduleID == 0 {
		// ON CONFLICT UPDATE doesn't return LastInsertId reliably — look it up.
		err = tx.QueryRow("SELECT id FROM user_modules WHERE name = ?", m.Name).Scan(&moduleID)
		if err != nil {
			return fmt.Errorf("lookup module id: %w", err)
		}
	}

	// Replace resource type mappings.
	if _, err := tx.Exec("DELETE FROM user_module_resources WHERE module_id = ?", moduleID); err != nil {
		return fmt.Errorf("clear resource types: %w", err)
	}
	for _, rt := range m.ResourceTypes {
		if _, err := tx.Exec("INSERT INTO user_module_resources (module_id, resource_type) VALUES (?, ?)", moduleID, rt); err != nil {
			return fmt.Errorf("insert resource type %s: %w", rt, err)
		}
	}

	return tx.Commit()
}

// GetModule retrieves a single module by name. Returns sql.ErrNoRows if not found.
func (s *Store) GetModule(name string) (*ModuleMetadata, error) {
	var (
		m         ModuleMetadata
		metaJSON  string
		createdAt string
		updatedAt string
	)
	err := s.db.QueryRow(`
		SELECT id, name, source, description, provider_type, metadata_json, created_at, updated_at
		FROM user_modules WHERE name = ?
	`, name).Scan(&m.ID, &m.Name, &m.Source, &m.Description, &m.ProviderType, &metaJSON, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	var row moduleRow
	if err := json.Unmarshal([]byte(metaJSON), &row); err != nil {
		return nil, fmt.Errorf("unmarshal module metadata: %w", err)
	}
	m.ResourceTypes = row.ResourceTypes
	m.DataSources = row.DataSources
	m.Variables = row.Variables
	m.Outputs = row.Outputs
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &m, nil
}

// ListModules returns all registered modules, ordered by name.
func (s *Store) ListModules() ([]ModuleMetadata, error) {
	rows, err := s.db.Query(`
		SELECT id, name, source, description, provider_type, metadata_json, created_at, updated_at
		FROM user_modules ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}
	defer rows.Close()

	var mods []ModuleMetadata
	for rows.Next() {
		var (
			m         ModuleMetadata
			metaJSON  string
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&m.ID, &m.Name, &m.Source, &m.Description, &m.ProviderType, &metaJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		var row moduleRow
		if err := json.Unmarshal([]byte(metaJSON), &row); err != nil {
			return nil, fmt.Errorf("unmarshal module %s: %w", m.Name, err)
		}
		m.ResourceTypes = row.ResourceTypes
		m.DataSources = row.DataSources
		m.Variables = row.Variables
		m.Outputs = row.Outputs
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		mods = append(mods, m)
	}
	return mods, rows.Err()
}

// DeleteModule removes a module by name. Returns nil if no module matched.
func (s *Store) DeleteModule(name string) error {
	_, err := s.db.Exec("DELETE FROM user_modules WHERE name = ?", name)
	return err
}

// FindModulesForResourceTypes returns all modules that manage at least one of
// the given resource types.
func (s *Store) FindModulesForResourceTypes(types []string) ([]ModuleMetadata, error) {
	if len(types) == 0 {
		return nil, nil
	}

	placeholders := ""
	args := make([]interface{}, len(types))
	for i, t := range types {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = t
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT m.id, m.name, m.source, m.description, m.provider_type,
		       m.metadata_json, m.created_at, m.updated_at
		FROM user_modules m
		JOIN user_module_resources mr ON m.id = mr.module_id
		WHERE mr.resource_type IN (%s)
		ORDER BY m.name
	`, placeholders)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find modules for resource types: %w", err)
	}
	defer rows.Close()

	var mods []ModuleMetadata
	for rows.Next() {
		var (
			m         ModuleMetadata
			metaJSON  string
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&m.ID, &m.Name, &m.Source, &m.Description, &m.ProviderType, &metaJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		var row moduleRow
		if err := json.Unmarshal([]byte(metaJSON), &row); err != nil {
			return nil, fmt.Errorf("unmarshal module %s: %w", m.Name, err)
		}
		m.ResourceTypes = row.ResourceTypes
		m.DataSources = row.DataSources
		m.Variables = row.Variables
		m.Outputs = row.Outputs
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		mods = append(mods, m)
	}
	return mods, rows.Err()
}
