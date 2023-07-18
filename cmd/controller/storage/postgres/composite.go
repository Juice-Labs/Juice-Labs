/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package postgres

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

var (
	errNotPtr = errors.New("destination not a pointer")
	errNilPtr = errors.New("destination pointer is nil")
)

type Composite []byte

func (composite Composite) Scan(dest ...any) error {
	columns, err := parseComposite(composite)
	if err != nil {
		return err
	}

	if len(columns) != len(dest) {
		return fmt.Errorf("Composite: expected %d destination arguments in Scan, not %d", len(columns), len(dest))
	}

	for i, column := range columns {
		err := convertColumn(dest[i], column)
		if err != nil {
			return fmt.Errorf("Composite: Scan error on column index %d, %w", i, err)
		}
	}

	return nil
}

// The following was pulled largely from github.com/lib/pq/array.go:parseArray
func parseComposite(src []byte) ([][]byte, error) {
	if len(src) < 1 || src[0] != '(' {
		return nil, fmt.Errorf("Composite: unable to parse composite, expected %q at offset %d", '(', 0)
	}

	i := 1
	elems := make([][]byte, 0)

Element:
	for i < len(src) {
		switch src[i] {
		case '"':
			elem := []byte{}
			escape := false
		QuoteSearch:
			for i++; i < len(src); i++ {
				if escape {
					elem = append(elem, src[i])
					escape = false
				} else {
					switch src[i] {
					default:
						elem = append(elem, src[i])
					case '\\':
						escape = true
					case '"':
						i++

						// Double double-quotes is an escape
						if i < len(src) && src[i] == '"' {
							elem = append(elem, src[i])
						} else {
							elems = append(elems, elem)
							break QuoteSearch
						}
					}
				}
			}

		default:
			for start := i; i < len(src); i++ {
				if src[i] == ',' || src[i] == ')' {
					elem := src[start:i]
					if len(elem) == 0 || bytes.Equal(elem, []byte("NULL")) {
						elem = nil
					}
					elems = append(elems, elem)
					break
				}
			}
		}

		switch src[i] {
		case ',':
			i++
		case ')':
			break Element
		}
	}

	if i >= len(src) {
		return nil, errors.New("Composite: unable to parse array; unexpected end")
	}

	if src[i] != ')' {
		return nil, fmt.Errorf("Composite: unable to parse array, unexpected %q at offset %d", src[i], i)
	}

	return elems, nil
}

// The following was pulled largely from database/sql/convert.go:convertAssignRows
func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

func convertColumn(dest any, src []byte) error {
	if dest == nil {
		return errNilPtr
	}

	switch d := dest.(type) {
	case *string:
		*d = string(src)
		return nil
	case *[]byte:
		*d = bytes.Clone(src)
		return nil
	case *bool:
		value, err := strconv.ParseBool(string(src))
		if err != nil {
			return err
		}
		*d = value
		return nil
	}

	if scanner, ok := dest.(sql.Scanner); ok {
		return scanner.Scan(src)
	}

	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Pointer {
		return errNotPtr
	}
	if dpv.IsNil() {
		return errNilPtr
	}

	// The following conversions use a string value as an intermediate representation
	// to convert between various numeric types.
	//
	// This also allows scanning into user defined types such as "type Int int64".
	// For symmetry, also check for string destination types.
	dv := reflect.Indirect(dpv)
	switch dv.Kind() {
	case reflect.Pointer:
		if src == nil {
			dv.Set(reflect.Zero(dv.Type()))
			return nil
		}
		dv.Set(reflect.New(dv.Type().Elem()))
		return convertColumn(dv.Interface(), src)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := string(src)
		i64, err := strconv.ParseInt(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting '%q' to a %s: %v", s, dv.Kind(), err)
		}
		dv.SetInt(i64)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := string(src)
		u64, err := strconv.ParseUint(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting '%q' to a %s: %v", s, dv.Kind(), err)
		}
		dv.SetUint(u64)
		return nil
	case reflect.Float32, reflect.Float64:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		s := string(src)
		f64, err := strconv.ParseFloat(s, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting '%q' to a %s: %v", s, dv.Kind(), err)
		}
		dv.SetFloat(f64)
		return nil
	case reflect.String:
		if src == nil {
			return fmt.Errorf("converting NULL to %s is unsupported", dv.Kind())
		}
		dv.SetString(string(src))
		return nil
	}

	return fmt.Errorf("unsupported Scan, storing %T into type %T", src, dest)
}
