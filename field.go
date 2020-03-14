/*
 * Investor Management Platform
 * Copyright (C) 2020 by crazydan.org
 */

package GoMybatis

import (
    "database/sql"
    "database/sql/driver"
    "errors"
    "fmt"
    "reflect"
	"strings"
	"sync"
)

type ModelStruct struct {
    PrimaryFields []*StructField
    StructFields  []*StructField
    ModelType     reflect.Type

    defaultTableName string
    l                sync.Mutex
}

type StructField struct {
    DBName          string
    Name            string
    Names           []string
    IsPrimaryKey    bool
    IsNormal        bool
    IsIgnored       bool
    IsScanner       bool
    HasDefaultValue bool
    Tag             reflect.StructTag
    TagSettings     map[string]string
    Struct          reflect.StructField
    IsForeignKey    bool

    tagSettingsLock sync.RWMutex
}

// Field model field definition
type Field struct {
    *StructField
    IsBlank bool
    Field   reflect.Value
}

var (
    ErrUnaddressable = errors.New("using unaddressable value")
)

func (field *Field) Match(name string) bool {
	return strings.ToLower(field.Name) == strings.ToLower(name)
}

// Set set a value to the field
func (field *Field) Set(value interface{}) (err error) {
    if !field.Field.IsValid() {
        return errors.New("field value not valid")
    }

    if !field.Field.CanAddr() {
        return ErrUnaddressable
    }

    reflectValue, ok := value.(reflect.Value)
    if !ok {
        reflectValue = reflect.ValueOf(value)
    }

    fieldValue := field.Field
    if reflectValue.IsValid() {
        if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
            fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
        } else {
            if fieldValue.Kind() == reflect.Ptr {
                if fieldValue.IsNil() {
                    fieldValue.Set(reflect.New(field.Struct.Type.Elem()))
                }
                fieldValue = fieldValue.Elem()
            }

            if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
                fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
            } else if scanner, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
                v := reflectValue.Interface()
                if valuer, ok := v.(driver.Valuer); ok {
                    if v, err = valuer.Value(); err == nil {
                        err = scanner.Scan(v)
                    }
                } else {
                    err = scanner.Scan(v)
                }
            } else {
                err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.Name, reflectValue.Type(), fieldValue.Type())
            }
        }
    } else {
        field.Field.Set(reflect.Zero(field.Field.Type()))
    }

    field.IsBlank = isBlank(field.Field)
    return err
}

// TagSettingsSet Sets a tag in the tag settings map
func (sf *StructField) TagSettingsSet(key, val string) {
    sf.tagSettingsLock.Lock()
    defer sf.tagSettingsLock.Unlock()
    sf.TagSettings[key] = val
}

// TagSettingsGet returns a tag from the tag settings
func (sf *StructField) TagSettingsGet(key string) (string, bool) {
    sf.tagSettingsLock.RLock()
    defer sf.tagSettingsLock.RUnlock()
    val, ok := sf.TagSettings[key]
    return val, ok
}

func (sf *StructField) clone() *StructField {
	clone := &StructField{
		DBName:          sf.DBName,
		Name:            sf.Name,
		Names:           sf.Names,
		IsPrimaryKey:    sf.IsPrimaryKey,
		IsNormal:        sf.IsNormal,
		IsIgnored:       sf.IsIgnored,
		IsScanner:       sf.IsScanner,
		HasDefaultValue: sf.HasDefaultValue,
		Tag:             sf.Tag,
		TagSettings:     map[string]string{},
		Struct:          sf.Struct,
		IsForeignKey:    sf.IsForeignKey,
	}

	// copy the struct field tagSettings, they should be read-locked while they are copied
	sf.tagSettingsLock.Lock()
	defer sf.tagSettingsLock.Unlock()
	for key, value := range sf.TagSettings {
		clone.TagSettings[key] = value
	}

	return clone
}
