package migrator

import (
	"database/sql"
	"fmt"
)

// Migrator is the main entry point
type Migrator struct {
	db            *sql.DB
	models        []registeredModel
	migrationsDir string
}

type registeredModel struct {
	model     interface{}
	tableName string
}

// New creates a new Migrator instance
func New(db *sql.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
	}
}

// Register adds a model to be tracked
// Usage: migrator.Register(&User{}, "users")
func (m *Migrator) Register(model interface{}, tableName string) *Migrator {
	m.models = append(m.models, registeredModel{model, tableName})
	return m
}

// Diff inspects all registered models against the live DB
// and returns all detected changes
func (m *Migrator) Diff() ([]ColumnDiff, error) {
	allDiffs := []ColumnDiff{}

	for _, rm := range m.models {
		modelSchema, err := ExtractModelSchema(rm.model, rm.tableName)
		if err != nil {
			return nil, fmt.Errorf("extracting schema for %s: %w", rm.tableName, err)
		}

		dbSchema, err := InspectDB(m.db, rm.tableName)
		if err != nil {
			return nil, fmt.Errorf("inspecting DB table %s: %w", rm.tableName, err)
		}

		diffs := Diff(modelSchema, dbSchema)
		allDiffs = append(allDiffs, diffs...)
	}

	return allDiffs, nil
}

// Generate runs the full diff and generates migration files
func (m *Migrator) Generate(name string) (upFile, downFile string, err error) {
	models := []*ModelSchema{}
	dbSchemas := map[string]*TableSchema{}

	for _, rm := range m.models {
		modelSchema, err := ExtractModelSchema(rm.model, rm.tableName)
		if err != nil {
			return "", "", fmt.Errorf("extracting schema for %s: %w", rm.tableName, err)
		}
		models = append(models, modelSchema)

		dbSchema, err := InspectDB(m.db, rm.tableName)
		if err != nil {
			return "", "", fmt.Errorf("inspecting DB table %s: %w", rm.tableName, err)
		}
		dbSchemas[rm.tableName] = dbSchema
	}

	return GenerateMigration(name, models, dbSchemas, m.migrationsDir)
}

// Status prints a summary of all detected changes
func (m *Migrator) Status() error {
	diffs, err := m.Diff()
	if err != nil {
		return err
	}

	if len(diffs) == 0 {
		fmt.Println("✅ No changes detected — DB is in sync with models")
		return nil
	}

	fmt.Printf("⚠️  Detected %d change(s):\n\n", len(diffs))
	for _, d := range diffs {
		switch d.ChangeType {
		case ChangeAdd:
			fmt.Printf("  + ADD    [%s] %s %s\n", d.Table, d.Column, d.SQLType)
		case ChangeModify:
			fmt.Printf("  ~ MODIFY [%s] %s: %s → %s\n", d.Table, d.Column, d.OldType, d.NewType)
		case ChangeDrop:
			fmt.Printf("  - DROP   [%s] %s\n", d.Table, d.Column)
		}
	}
	fmt.Println()
	return nil
}
