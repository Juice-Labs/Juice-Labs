/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package utilities

import (
	"reflect"

	"github.com/Xdevlab/Run/pkg/errors"
	"github.com/Xdevlab/Run/pkg/logger"
)

var (
	ErrInvalidCast = errors.New("utilities: invalid cast")
)

func Cast[T any](value any) (T, error) {
	converted, ok := value.(T)
	if !ok {
		return converted, ErrInvalidCast.Wrap(errors.Newf("invalid cast from type '%s' to type '%s'", reflect.TypeOf(value), reflect.TypeOf(converted)))
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
