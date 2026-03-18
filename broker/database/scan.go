package database

import (
	"reflect"

	"github.com/jackc/pgx/v5"
)

// ScanRow maps a single pgx.Row into T by scanning into each exported field in
// declaration order. The SELECT column list must match the struct field order.
func ScanRow[T any](row pgx.Row) (T, error) {
	var result T
	dest := fieldPtrs(&result)
	return result, row.Scan(dest...)
}

// ScanRows maps all rows into a slice of T, scanning each exported field in
// declaration order. The SELECT column list must match the struct field order.
func ScanRows[T any](rows pgx.Rows) ([]T, error) {
	defer rows.Close()
	var results []T
	for rows.Next() {
		var result T
		if err := rows.Scan(fieldPtrs(&result)...); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func fieldPtrs(v any) []any {
	rv := reflect.ValueOf(v).Elem()
	ptrs := make([]any, rv.NumField())
	for i := range ptrs {
		ptrs[i] = rv.Field(i).Addr().Interface()
	}
	return ptrs
}
