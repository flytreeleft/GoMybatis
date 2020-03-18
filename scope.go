/*
 * Investor Management Platform
 * Copyright (C) 2020 by crazydan.org
 */

package GoMybatis

import (
	"database/sql"
	"go/ast"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Scope contain current operation's information when you perform any operation on the database
type Scope struct {
	Value           interface{}
	SQL             string
	SQLVars         []interface{}
	instanceID      string
	primaryKeyField *Field
	skipLeft        bool
	fields          *[]*Field
	selectAttrs     *[]string
}

// New create a new Scope without search information
func (scope *Scope) New(value interface{}) *Scope {
	return &Scope{Value: value}
}

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	return indirect(reflect.ValueOf(scope.Value))
}

// SkipLeft skip remaining callbacks
func (scope *Scope) SkipLeft() {
	scope.skipLeft = true
}

// Fields get value's fields
func (scope *Scope) Fields() []*Field {
	if scope.fields == nil {
		var (
			fields             []*Field
			indirectScopeValue = scope.IndirectValue()
			isStruct           = indirectScopeValue.Kind() == reflect.Struct
		)

		for _, structField := range scope.GetModelStruct().StructFields {
			if isStruct {
				fieldValue := indirectScopeValue
				for _, name := range structField.Names {
					if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
						fieldValue.Set(reflect.New(fieldValue.Type().Elem()))
					}
					fieldValue = reflect.Indirect(fieldValue).FieldByName(name)
				}
				fields = append(fields, &Field{StructField: structField, Field: fieldValue, IsBlank: isBlank(fieldValue)})
			} else {
				fields = append(fields, &Field{StructField: structField, IsBlank: true})
			}
		}
		scope.fields = &fields
	}

	return *scope.fields
}

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields []*Field, resultMap map[string]*ResultProperty) error {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       []*Field
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*Field{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		offset := 0
		if idx, ok := selectedColumnsMap[column]; ok {
			offset = idx + 1
			selectFields = selectFields[offset:]
		}

		property := resultMap[column]
		if property == nil {
			continue
		}

		for fieldIndex, field := range selectFields {
			if field.Match(property.Property) {
				if field.Field.Kind() == reflect.Ptr {
					values[index] = field.Field.Addr().Interface()
					resetFields[index] = field
				} else {
					reflectValue := reflect.New(reflect.PtrTo(field.Struct.Type))
					reflectValue.Elem().Set(field.Field.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}

				selectedColumnsMap[column] = offset + fieldIndex

				if field.IsNormal {
					break
				}
			}
		}
	}

	if err := rows.Scan(values...); err != nil {
		return err
	}

	resetFieldNames := map[string]bool{}
	for index, field := range resetFields {
		v := reflect.ValueOf(values[index]).Elem().Elem()
		if err := field.Set(v); err != nil {
			return err
		} else if !v.IsValid() {
			continue
		}

		fieldNames := strings.Split(field.StructField.Name, ".")
		for i := 1; i < len(fieldNames); i++ {
			name := strings.Join(fieldNames[0:i], ".")
			resetFieldNames[name] = true
		}
		resetFieldNames[field.StructField.Name] = true
	}
	// clean the unused fields
	for _, field := range fields {
		if resetFieldNames[field.StructField.Name] != true {
			// ignore the clean error
			field.Set(reflect.Zero(field.Field.Type()))
		}
	}

	return nil
}


// global cache of model metadata
var modelStructsMap sync.Map
// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := reflect.ValueOf(scope.Value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Scope value need to be a struct
	if reflectType.Kind() != reflect.Struct {
		return &modelStruct
	}

	// Get Cached model struct
	isSingularTable := false

	hashKey := struct {
		singularTable bool
		reflectType   reflect.Type
	}{isSingularTable, reflectType}

	if value, ok := modelStructsMap.Load(hashKey); ok && value != nil {
		return value.(*ModelStruct)
	}
	// Avoid the cycle reference
	modelStructsMap.Store(hashKey, &modelStruct)

	modelStruct.ModelType = reflectType

	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		if fieldStruct := reflectType.Field(i); ast.IsExported(fieldStruct.Name) {
			field := &StructField{
				Struct:      fieldStruct,
				Name:        fieldStruct.Name,
				Names:       []string{fieldStruct.Name},
				Tag:         fieldStruct.Tag,
				TagSettings: parseTagSetting(fieldStruct.Tag),
			}

			// is ignored field
			if _, ok := field.TagSettingsGet("-"); ok {
				field.IsIgnored = true
			} else {
				indirectType := fieldStruct.Type
				for indirectType.Kind() == reflect.Ptr {
					indirectType = indirectType.Elem()
				}

				fieldValue := reflect.New(indirectType).Interface()
				if _, isScanner := fieldValue.(sql.Scanner); isScanner {
					// is scanner
					field.IsScanner, field.IsNormal = true, true
					if indirectType.Kind() == reflect.Struct {
						for i := 0; i < indirectType.NumField(); i++ {
							for key, value := range parseTagSetting(indirectType.Field(i).Tag) {
								if _, ok := field.TagSettingsGet(key); !ok {
									field.TagSettingsSet(key, value)
								}
							}
						}
					}
				} else if _, isTime := fieldValue.(*time.Time); isTime {
					// is time
					field.IsNormal = true
				} else if fieldStruct.Anonymous {
					// is embedded struct
					for _, subField := range scope.New(fieldValue).GetModelStruct().StructFields {
						subField = subField.clone()
						subField.Names = append([]string{fieldStruct.Name}, subField.Names...)

						if subField.IsPrimaryKey {
							if _, ok := subField.TagSettingsGet("PRIMARY_KEY"); ok {
								modelStruct.PrimaryFields = append(modelStruct.PrimaryFields, subField)
							} else {
								subField.IsPrimaryKey = false
							}
						}

						modelStruct.StructFields = append(modelStruct.StructFields, subField)
					}
					continue
				} else {
					switch indirectType.Kind() {
					case reflect.Slice:
						// var elemType = field.Struct.Type
						// for elemType.Kind() == reflect.Slice || elemType.Kind() == reflect.Ptr {
						// 	elemType = elemType.Elem()
						// }
						//
						// if elemType.Kind() == reflect.Struct {
						// 	for _, subField := range scope.New(fieldValue).GetModelStruct().StructFields {
						// 		subField = subField.clone()
						// 		subField.Name = fieldStruct.Name+"."+subField.Name
						// 		subField.Names = append([]string{fieldStruct.Name}, subField.Names...)
						//
						// 		modelStruct.StructFields = append(modelStruct.StructFields, subField)
						// 	}
						// } else {
						// 	field.IsNormal = true
						// }
					case reflect.Struct:
						for _, subField := range scope.New(fieldValue).GetModelStruct().StructFields {
							subField = subField.clone()
							subField.Name = fieldStruct.Name+"."+subField.Name
							subField.Names = append([]string{fieldStruct.Name}, subField.Names...)

							modelStruct.StructFields = append(modelStruct.StructFields, subField)
						}
					default:
						field.IsNormal = true
					}
				}
			}

			modelStruct.StructFields = append(modelStruct.StructFields, field)
		}
	}

	return &modelStruct
}

// GetStructFields get model's field structs
func (scope *Scope) GetStructFields() (fields []*StructField) {
	return scope.GetModelStruct().StructFields
}

func parseTagSetting(tags reflect.StructTag) map[string]string {
	setting := map[string]string{}
	for _, str := range []string{tags.Get("sql"), tags.Get("gorm")} {
		if str == "" {
			continue
		}
		tags := strings.Split(str, ";")
		for _, value := range tags {
			v := strings.Split(value, ":")
			k := strings.TrimSpace(strings.ToUpper(v[0]))
			if len(v) >= 2 {
				setting[k] = strings.Join(v[1:], ":")
			} else {
				setting[k] = k
			}
		}
	}
	return setting
}
