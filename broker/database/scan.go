// Copyright 2026 Sam Schreiber
//
// This file is part of nito.
//
// nito is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// nito is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with nito. If not, see <https://www.gnu.org/licenses/>.

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
