/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import (
	"fmt"
	"reflect"

	"github.com/Juice-Labs/Juice-Labs//pkg/logger"
)

func Cast[T any](value any) (T, error) {
	var converted T
	if converted, ok := value.(T); !ok {
		return converted, fmt.Errorf("invalid cast type '%s' to type '%s'", reflect.TypeOf(value), reflect.TypeOf(converted))
	}

	return converted, nil
}

func Require[T any](value any) T {
	result, err := Cast[T](value)
	if err != nil {
		logger.Panic(err)
	}

	return result
}
