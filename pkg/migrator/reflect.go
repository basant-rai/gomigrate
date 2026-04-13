package migrator

import (
	"fmt"
	"reflect"
	"strings"
)

// Valuer interface — implement this on your enum types to auto-discover values
// Example:
//
//	func (Status) Values() []string {
//	    return []string{"pending", "submitted", "active", "restricted"}
//	}
type Valuer interface {
	Values() []string
}

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

		// Read enum tag — supports two formats:
		//   enum:"none,pending,submitted"         → explicit values
		//   enum:"KYBStatus"                       → reads Values() from type
		enumValues := readEnumTag(field)

		schema.Fields = append(schema.Fields, StructField{
			GoName:     field.Name,
			DBName:     dbName,
			GoType:     field.Type,
			SQLType:    sqlType,
			Nullable:   nullable,
			EnumValues: enumValues,
		})
	}
}

// readEnumTag reads the `enum` struct tag and resolves values
func readEnumTag(field reflect.StructField) []string {
	enumTag := field.Tag.Get("enum")
	if enumTag == "" {
		return nil
	}

	enumTag = strings.TrimSpace(enumTag)

	// Format 1: explicit values — enum:"none,pending,submitted"
	if strings.Contains(enumTag, ",") {
		values := strings.Split(enumTag, ",")
		for i, v := range values {
			values[i] = strings.TrimSpace(v)
		}
		return values
	}

	// Format 2: type name — enum:"KYBStatus"
	// Try to call Values() on a zero value of the field type
	fieldType := field.Type
	if fieldType.Kind() == reflect.Ptr {
		fieldType = fieldType.Elem()
	}

	// Create zero value and check if it implements Valuer
	zeroVal := reflect.New(fieldType).Elem()
	if valuer, ok := zeroVal.Interface().(Valuer); ok {
		return valuer.Values()
	}

	// Also try pointer receiver
	zeroPtr := reflect.New(fieldType)
	if valuer, ok := zeroPtr.Interface().(Valuer); ok {
		return valuer.Values()
	}

	return nil
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
