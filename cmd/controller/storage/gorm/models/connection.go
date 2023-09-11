package models

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type ExitStatus int

const (
	ExitStatusUnknown ExitStatus = iota
	ExitStatusSuccess
	ExitStatusFailure
	ExitStatusCanceled
)

var (
	exitStatusMappings = map[string]ExitStatus{
		"unknown":  ExitStatusUnknown,
		"success":  ExitStatusSuccess,
		"failure":  ExitStatusFailure,
		"canceled": ExitStatusCanceled,
	}
)

func (es ExitStatus) String() string {
	switch es {
	case ExitStatusUnknown:
		return "unknown"
	case ExitStatusSuccess:
		return "success"
	case ExitStatusFailure:
		return "failure"
	case ExitStatusCanceled:
		return "canceled"
	}
	panic(fmt.Sprintf("invalid ExitStatus, %d", es))
}

func ExitStatusFromString(value string) ExitStatus {
	status, ok := exitStatusMappings[strings.ToLower(value)]
	if !ok {
		return ExitStatusUnknown
	}
	return status
}

type Connection struct {
	gorm.Model

	UUID        uuid.UUID `gorm:"type:uuid;notnull;unique"`
	SessionID   uint      `gorm:"constraint:OnDelete:CASCADE;"`
	Session     Session
	ExitStatus  ExitStatus
	Pid         uint64
	ProcessName string
}
