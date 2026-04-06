package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// InspectDB reads the current schema from a live Postgres database
func InspectDB(db *sql.DB, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
		Columns:   make(map[string]DBColumn),
	}

	// Check if table exists
	var exists bool
	err := db.QueryRowContext(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		)`, tableName).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("checking table existence: %w", err)
	}

	if !exists {
		return schema, nil // empty schema = table doesn't exist yet
	}

	// Read all columns
	rows, err := db.QueryContext(context.Background(), `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`,
		tableName,
	)
	if err != nil {
		return nil, fmt.Errorf("reading columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var col DBColumn
		var isNullable string
		var colDefault sql.NullString

		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &colDefault); err != nil {
			return nil, fmt.Errorf("scanning column: %w", err)
		}

		col.IsNullable = isNullable == "YES"
		col.DataType = normalizeDBType(col.DataType)

		if colDefault.Valid {
			col.Default = &colDefault.String
		}

		schema.Columns[col.Name] = col
	}

	return schema, rows.Err()
}

// normalizeDBType normalizes Postgres type names for comparison
func normalizeDBType(pgType string) string {
	pgType = strings.ToUpper(strings.TrimSpace(pgType))

	switch pgType {
	case "CHARACTER VARYING", "VARCHAR":
		return "TEXT"
	case "TIMESTAMP WITH TIME ZONE":
		return "TIMESTAMPTZ"
	case "INT8", "BIGSERIAL":
		return "BIGINT"
	case "INT", "INT4", "SERIAL":
		return "INTEGER"
	case "BOOL":
		return "BOOLEAN"
	case "DOUBLE PRECISION", "FLOAT8":
		return "FLOAT"
	case "ARRAY":
		return "TEXT[]"
	default:
		return pgType
	}
}
