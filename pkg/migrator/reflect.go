package migrator

import (
	"fmt"
	"reflect"
	"strings"
)

// Valuer interface — implement on your enum types to auto-discover values
// Example:
//
//	func (KYBStatus) Values() []string {
//	    return []string{"none", "pending", "submitted", "active", "restricted"}
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

		dbName := strings.Split(dbTag, ",")[0]
		if dbName == "" || dbName == "-" {
			continue
		}

		// Skip base fields
		if isBaseField(dbName) {
			continue
		}

		sqlType, nullable := goTypeToSQL(field.Type)
		if sqlType == "" {
			continue
		}

		schema.Fields = append(schema.Fields, StructField{
			GoName:     field.Name,
			DBName:     dbName,
			GoType:     field.Type,
			SQLType:    sqlType,
			Nullable:   nullable,
			EnumValues: readEnumTag(field),
			References: readReferencesTag(field),
			Unique:     field.Tag.Get("unique") == "true",
			Index:      field.Tag.Get("index") == "true",
		})
	}
}

// readEnumTag reads `enum:"val1,val2"` or `enum:"TypeName"` tag
func readEnumTag(field reflect.StructField) []string {
	enumTag := strings.TrimSpace(field.Tag.Get("enum"))
	if enumTag == "" {
		return nil
	}

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

	// Value receiver
	zeroVal := reflect.New(fieldType).Elem()
	if valuer, ok := zeroVal.Interface().(Valuer); ok {
		return valuer.Values()
	}

	// Pointer receiver
	if valuer, ok := reflect.New(fieldType).Interface().(Valuer); ok {
		return valuer.Values()
	}

	return nil
}

// readReferencesTag reads `references:"table"` or `references:"table.column"` or
// `references:"table.column:CASCADE"` tag
//
// Examples:
//
//	references:"users"                → REFERENCES users(id)
//	references:"users.id"             → REFERENCES users(id)
//	references:"users.id:CASCADE"     → REFERENCES users(id) ON DELETE CASCADE
//	references:"users.id:SET NULL"    → REFERENCES users(id) ON DELETE SET NULL
func readReferencesTag(field reflect.StructField) *FKRef {
	tag := strings.TrimSpace(field.Tag.Get("references"))
	if tag == "" {
		return nil
	}

	ref := &FKRef{
		Column:   "id",
		OnDelete: "RESTRICT",
	}

	// Split on colon for ON DELETE action
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) == 2 {
		ref.OnDelete = strings.TrimSpace(strings.ToUpper(parts[1]))
	}

	// Split table.column
	tableParts := strings.SplitN(parts[0], ".", 2)
	ref.Table = strings.TrimSpace(tableParts[0])
	if len(tableParts) == 2 {
		ref.Column = strings.TrimSpace(tableParts[1])
	}

	return ref
}

// isBaseField checks if field is from domain.Base
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
	if t.Kind() == reflect.Ptr {
		sql, _ := goTypeToSQL(t.Elem())
		return sql, true
	}

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
		// Custom string types (KYBStatus, PaymentSource etc.) → TEXT
		if t.Kind() == reflect.String {
			return "TEXT", false
		}
		return "", false
	}
	return sql, false
}

// resolveTypeName gets a clean type name for mapping
func resolveTypeName(t reflect.Type) string {
	pkgPath := t.PkgPath()
	name := t.Name()

	if pkgPath == "" {
		return name
	}
	if strings.Contains(pkgPath, "time") && name == "Time" {
		return "time.Time"
	}
	if strings.Contains(pkgPath, "uuid") && name == "UUID" {
		return "uuid.UUID"
	}
	return name
}
