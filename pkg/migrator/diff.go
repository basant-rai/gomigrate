package migrator

import (
	"fmt"
	"strings"
)

// Diff compares a ModelSchema against a live TableSchema and returns changes
func Diff(model *ModelSchema, db *TableSchema) []ColumnDiff {
	diffs := []ColumnDiff{}

	if len(db.Columns) == 0 {
		return diffs
	}

	for _, field := range model.Fields {
		dbCol, exists := db.Columns[field.DBName]

		if !exists {
			diffs = append(diffs, ColumnDiff{
				Table:      model.TableName,
				Column:     field.DBName,
				ChangeType: ChangeAdd,
				SQLType:    field.SQLType,
				Nullable:   field.Nullable,
				EnumValues: field.EnumValues,
				References: field.References,
				Unique:     field.Unique,
			})
			continue
		}

		// Type mismatch
		if !typesCompatible(field.SQLType, dbCol.DataType) {
			diffs = append(diffs, ColumnDiff{
				Table:      model.TableName,
				Column:     field.DBName,
				ChangeType: ChangeModify,
				OldType:    dbCol.DataType,
				NewType:    field.SQLType,
				Nullable:   field.Nullable,
				EnumValues: field.EnumValues,
				References: field.References,
			})
		}
	}

	return diffs
}

// DiffIndexes returns indexes that exist in model but not in DB
func DiffIndexes(model *ModelSchema, db *TableSchema) []IndexDef {
	indexes := []IndexDef{}

	for _, field := range model.Fields {
		// Index tag
		if field.Index {
			idxName := fmt.Sprintf("idx_%s_%s", model.TableName, field.DBName)
			if _, exists := db.Indexes[idxName]; !exists {
				indexes = append(indexes, IndexDef{
					Table:   model.TableName,
					Columns: []string{field.DBName},
					Unique:  false,
					Name:    idxName,
				})
			}
		}

		// Unique tag → unique index
		if field.Unique {
			idxName := fmt.Sprintf("idx_%s_%s_unique", model.TableName, field.DBName)
			if _, exists := db.Indexes[idxName]; !exists {
				indexes = append(indexes, IndexDef{
					Table:   model.TableName,
					Columns: []string{field.DBName},
					Unique:  true,
					Name:    idxName,
				})
			}
		}

		// FK → auto index on foreign key column
		if field.References != nil {
			idxName := fmt.Sprintf("idx_%s_%s", model.TableName, field.DBName)
			if _, exists := db.Indexes[idxName]; !exists {
				indexes = append(indexes, IndexDef{
					Table:   model.TableName,
					Columns: []string{field.DBName},
					Unique:  false,
					Name:    idxName,
				})
			}
		}
	}

	return indexes
}

// typesCompatible checks if a Go SQL type matches a DB type
func typesCompatible(goSQL, dbType string) bool {
	goSQL = strings.ToUpper(goSQL)
	dbType = strings.ToUpper(dbType)

	if goSQL == dbType {
		return true
	}

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
