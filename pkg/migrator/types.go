package migrator

import "reflect"

// ColumnDiff represents a detected change between struct and DB
type ColumnDiff struct {
	Table      string
	Column     string
	ChangeType ChangeType
	SQLType    string
	OldType    string
	NewType    string
	Nullable   bool
	EnumValues []string // CHECK constraint values
	References *FKRef   // foreign key reference
	Unique     bool     // UNIQUE constraint
}

type ChangeType string

const (
	ChangeAdd    ChangeType = "ADD"
	ChangeModify ChangeType = "MODIFY"
	ChangeDrop   ChangeType = "DROP"
)

// FKRef represents a foreign key reference
type FKRef struct {
	Table    string // referenced table
	Column   string // referenced column (default: id)
	OnDelete string // CASCADE, SET NULL, RESTRICT (default: RESTRICT)
}

// IndexDef represents an index to be created
type IndexDef struct {
	Table   string
	Columns []string
	Unique  bool
	Name    string // auto-generated if empty
}

// StructField parsed from Go struct via reflection
type StructField struct {
	GoName     string
	DBName     string
	GoType     reflect.Type
	SQLType    string
	Nullable   bool
	HasDefault bool
	Default    string
	EnumValues []string // from `enum:"val1,val2"` tag
	References *FKRef   // from `references:"table.column"` tag
	Unique     bool     // from `unique:"true"` tag
	Index      bool     // from `index:"true"` tag
}

// DBColumn read from information_schema
type DBColumn struct {
	Name       string
	DataType   string
	IsNullable bool
	Default    *string
}

// TableSchema represents a table's current state in DB
type TableSchema struct {
	TableName string
	Columns   map[string]DBColumn
	Indexes   map[string]DBIndex
}

// DBIndex represents an existing index in the DB
type DBIndex struct {
	Name    string
	Columns []string
	Unique  bool
}

// ModelSchema represents a Go struct's desired state
type ModelSchema struct {
	TableName string
	Fields    []StructField
}

// GoToSQL type mapping
var GoToSQL = map[string]string{
	"string":    "TEXT",
	"int":       "INTEGER",
	"int32":     "INTEGER",
	"int64":     "BIGINT",
	"float32":   "FLOAT",
	"float64":   "FLOAT",
	"bool":      "BOOLEAN",
	"time.Time": "TIMESTAMPTZ",
	"uuid.UUID": "UUID",
	"[]byte":    "BYTEA",
	"[]string":  "TEXT[]",
	// Pointer types (nullable)
	"*string":    "TEXT",
	"*int":       "INTEGER",
	"*int32":     "INTEGER",
	"*int64":     "BIGINT",
	"*float32":   "FLOAT",
	"*float64":   "FLOAT",
	"*bool":      "BOOLEAN",
	"*time.Time": "TIMESTAMPTZ",
	"*uuid.UUID": "UUID",
}

// SQLTypeCompatible checks if two SQL types are compatible
var SQLTypeCompatible = map[string][]string{
	"TEXT":        {"VARCHAR", "TEXT", "CHARACTER VARYING"},
	"BIGINT":      {"BIGINT", "INT8"},
	"INTEGER":     {"INTEGER", "INT", "INT4"},
	"BOOLEAN":     {"BOOLEAN", "BOOL"},
	"TIMESTAMPTZ": {"TIMESTAMPTZ", "TIMESTAMP WITH TIME ZONE"},
	"UUID":        {"UUID"},
	"FLOAT":       {"FLOAT", "FLOAT8", "DOUBLE PRECISION"},
	"TEXT[]":      {"ARRAY", "TEXT[]"},
	"BYTEA":       {"BYTEA"},
}
