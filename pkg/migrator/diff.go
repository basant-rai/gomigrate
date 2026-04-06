package migrator

import "strings"

// Diff compares a ModelSchema (Go struct) against a TableSchema (live DB)
// and returns a list of changes needed
func Diff(model *ModelSchema, db *TableSchema) []ColumnDiff {
	diffs := []ColumnDiff{}

	// Case 1: Table doesn't exist at all → CREATE TABLE (handled in generator)
	if len(db.Columns) == 0 {
		return diffs // generator handles full table creation
	}

	// Case 2: Compare each struct field against DB columns
	for _, field := range model.Fields {
		dbCol, exists := db.Columns[field.DBName]

		if !exists {
			// Column missing in DB → ADD COLUMN
			diffs = append(diffs, ColumnDiff{
				Table:      model.TableName,
				Column:     field.DBName,
				ChangeType: ChangeAdd,
				SQLType:    field.SQLType,
				Nullable:   field.Nullable,
			})
			continue
		}

		// Column exists → check type compatibility
		if !typesCompatible(field.SQLType, dbCol.DataType) {
			diffs = append(diffs, ColumnDiff{
				Table:      model.TableName,
				Column:     field.DBName,
				ChangeType: ChangeModify,
				OldType:    dbCol.DataType,
				NewType:    field.SQLType,
				Nullable:   field.Nullable,
			})
		}
	}

	return diffs
}

// typesCompatible checks if a Go SQL type matches a DB type
func typesCompatible(goSQL, dbType string) bool {
	goSQL = strings.ToUpper(goSQL)
	dbType = strings.ToUpper(dbType)

	if goSQL == dbType {
		return true
	}

	// Check compatibility map
	compatibles, ok := SQLTypeCompatible[goSQL]
	if !ok {
		return false
	}

	for _, c := range compatibles {
		if c == dbType {
			return true
		}
	}
	return false
}
