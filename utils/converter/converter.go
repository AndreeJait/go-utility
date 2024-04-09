package converter

import (
	"database/sql"
	"github.com/AndreeJait/go-utility/sqlw/postgres/rows"
	"github.com/jackc/pgx/v5"
	"reflect"
)

//go:generate mockery --name=Converter --filename=mock_Converter.go --inpackage
type Converter interface {
	ConvertRows(rows rows.RowsI, tag string, result interface{}) error
	MapToReflectValue(mapRow map[string]any, tag string, reflectValue reflect.Value) reflect.Value
}

type converter struct {
}

func New() Converter {
	return &converter{}
}

func (m *converter) MapToReflectValue(mapRow map[string]any, tag string, reflectValue reflect.Value) reflect.Value {

	newStruct := reflectValue

	// Get the type of the struct
	structType := newStruct.Type()

	if structType.Kind() != reflect.Struct {
		for _, value := range mapRow {
			newStruct.Set(reflect.ValueOf(value))
		}
		return newStruct
	}

	// Loop through fields of the struct
	for i := 0; i < newStruct.NumField(); i++ {
		field := newStruct.Field(i)
		fieldType := structType.Field(i)

		// Get the tag value
		tagValue := fieldType.Tag.Get(tag)

		// Check if the tag exists in the values map
		if value, ok := mapRow[tagValue]; ok {
			// Set value to the field
			field.Set(reflect.ValueOf(value))
		}

	}
	return newStruct
}

func (m *converter) ConvertRows(rows rows.RowsI, tag string, result interface{}) error {
	switch reflect.TypeOf(result).Elem().Kind() {
	case reflect.Slice, reflect.Array:
		slc := reflect.ValueOf(result).Elem()
		for rows.Next() {
			mapRows, err := pgx.RowToMap(rows)
			if err != nil {
				return err
			}
			newStruct := m.MapToReflectValue(mapRows, tag, reflect.New(slc.Type().Elem()).Elem())
			// Append the new struct to the slice
			slc.Set(reflect.Append(slc, newStruct))
		}
		rows.Close()
		return nil
	case reflect.Map:
		exists := rows.Next()
		defer rows.Close()
		if !exists {
			return sql.ErrNoRows
		}
		mapRows, err := pgx.RowToMap(rows)
		if err != nil {
			return err
		}
		result = &mapRows
		return nil
	case reflect.Struct:
		exists := rows.Next()
		defer rows.Close()
		if !exists {
			return sql.ErrNoRows
		}
		mapRows, err := pgx.RowToMap(rows)
		if err != nil {
			return err
		}
		m.MapToReflectValue(mapRows, tag, reflect.ValueOf(result).Elem())
		return nil
	default:
		exists := rows.Next()
		if !exists {
			return sql.ErrNoRows
		}
		err := rows.Scan(result)
		if err != nil {
			return err
		}
		return nil
	}
}
