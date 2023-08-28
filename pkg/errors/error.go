/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package errors

import (
	"errors"
	"fmt"
	"sync/atomic"
)

var (
	errorId = atomic.Uint64{}

	ErrRuntime = New("errors: runtime error")
)

type Error struct {
	id      uint64
	Message string
	Cause   error
}

func (err *Error) Wrap(cause error) *Error {
	return &Error{
		id:      err.id,
		Message: err.Message,
		Cause:   cause,
	}
}

func (err *Error) Error() string {
	if err.Cause != nil {
		return fmt.Sprintf("%s caused by %s", err.Message, err.Cause.Error())
	}

	return err.Message
}

func (err *Error) Unwrap() error {
	return err.Cause
}

func (err *Error) Is(target error) bool {
	if castTarget, ok := target.(*Error); ok {
		if err.id == castTarget.id {
			return true
		}
	}

	return errors.Is(err.Cause, target)
}

func New(a ...any) *Error {
	return &Error{
		id:      errorId.Add(1) - 1,
		Message: fmt.Sprint(a...),
		Cause:   nil,
	}
}

func Newf(format string, a ...any) *Error {
	return New(fmt.Sprintf(format, a...))
}

func Join(errs ...error) error {
	return errors.Join(errs...)
}

func Unwrap(err error) error {
	return errors.Unwrap(err)
}

func Is(err error, target error) bool {
	return errors.Is(err, target)
}
