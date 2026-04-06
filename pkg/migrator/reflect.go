package migrator

import (
	"fmt"
	"reflect"
	"strings"
)

// ExtractModelSchema reads a Go struct and returns its schema
func ExtractModelSchema(model interface{}, tableName string) (*ModelSchema, error) {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("model must be a struct, got %s", t.Kind())
	}

	schema := &ModelSchema{
		TableName: tableName,
		Fields:    []StructField{},
	}

	extractFields(t, schema)
	return schema, nil
}

// extractFields recursively extracts fields including embedded structs
func extractFields(t reflect.Type, schema *ModelSchema) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Handle embedded structs (like domain.Base)
		if field.Anonymous {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Ptr {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				extractFields(embeddedType, schema)
				continue
			}
		}

		// Get db tag
		dbTag := field.Tag.Get("db")
		if dbTag == "" || dbTag == "-" {
			continue
		}

		// Parse db tag (handle options like `db:"name,omitempty"`)
		dbName := strings.Split(dbTag, ",")[0]
		if dbName == "" || dbName == "-" {
			continue
		}

		// Skip base fields managed separately
		if isBaseField(dbName) {
			continue
		}

		sqlType, nullable := goTypeToSQL(field.Type)
		if sqlType == "" {
			continue // unsupported type
		}

		schema.Fields = append(schema.Fields, StructField{
			GoName:   field.Name,
			DBName:   dbName,
			GoType:   field.Type,
			SQLType:  sqlType,
			Nullable: nullable,
		})
	}
}

// isBaseField checks if field is from domain.Base (already in initial schema)
func isBaseField(name string) bool {
	base := map[string]bool{
		"id":         true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
	}
	return base[name]
}

// goTypeToSQL converts a Go reflect.Type to SQL type
func goTypeToSQL(t reflect.Type) (sqlType string, nullable bool) {
	// Handle pointer types (nullable)
	if t.Kind() == reflect.Ptr {
		sql, _ := goTypeToSQL(t.Elem())
		return sql, true
	}

	// Handle slices
	if t.Kind() == reflect.Slice {
		if t.Elem().Kind() == reflect.String {
			return "TEXT[]", false
		}
		if t.Elem().Kind() == reflect.Uint8 {
			return "BYTEA", false
		}
		return "", false
	}

	typeName := resolveTypeName(t)
	sql, ok := GoToSQL[typeName]
	if !ok {
		return "", false
	}
	return sql, false
}

// resolveTypeName gets a clean type name for mapping
func resolveTypeName(t reflect.Type) string {
	pkgPath := t.PkgPath()
	name := t.Name()

	if pkgPath == "" {
		return name // primitive type
	}

	// Handle well-known packages
	if strings.Contains(pkgPath, "time") && name == "Time" {
		return "time.Time"
	}
	if strings.Contains(pkgPath, "uuid") && name == "UUID" {
		return "uuid.UUID"
	}

	return name
}
