/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import (
	"strings"
)

// A `flag.Value` compatible Value that accepts a comma seperated string
// and produces an array of strings
type CommaValue struct {
	Value *[]string
}

func (v CommaValue) String() string {
	if v.Value != nil {
		return strings.Join(*v.Value, ",")
	}
	return ""
}

func (v CommaValue) Set(s string) error {
	*v.Value = strings.Split(s, ",")
	return nil
}
