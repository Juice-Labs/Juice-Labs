package models

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SessionState int

const (
	SessionStateUnknown SessionState = iota
	SessionStateQueued
	SessionStateAssigned
	SessionStateActive
	SessionStateCanceling
	SessionStateClosed
)

var (
	sessionStateMappings = map[string]SessionState{
		"uknown":    SessionStateUnknown,
		"queued":    SessionStateQueued,
		"assigned":  SessionStateAssigned,
		"active":    SessionStateActive,
		"canceling": SessionStateCanceling,
		"closed":    SessionStateClosed,
	}
)

func SessionStateFromString(value string) SessionState {
	status, ok := sessionStateMappings[strings.ToLower(value)]
	if !ok {
		return SessionStateUnknown
	}
	return status
}

func (ss SessionState) String() string {
	switch ss {
	case SessionStateUnknown:
		return "uknown"
	case SessionStateQueued:
		return "queued"
	case SessionStateAssigned:
		return "assigned"
	case SessionStateActive:
		return "active"
	case SessionStateCanceling:
		return "canceling"
	case SessionStateClosed:
		return "closed"
	}
	panic(fmt.Sprintf("invalid SessionState, %d", ss))
}

type Session struct {
	gorm.Model

	UUID         uuid.UUID `gorm:"type:uuid;notnull;unique"`
	AgentID      *uint
	Agent        *Agent
	State        SessionState
	Address      string
	Version      string
	Persistent   bool
	GPUs         datatypes.JSON
	VramRequired uint64
	Requirements datatypes.JSON

	Connections []Connection

	Labels    []KeyValue `gorm:"many2many:session_labels;constraint:OnDelete:CASCADE;"`
	Tolerates []KeyValue `gorm:"many2many:session_tolerates;constraint:OnDelete:CASCADE;"`

	PoolID uuid.UUID `gorm:"type:uuid;"`
	Pool   Pool      `gorm:"constraint:OnDelete:CASCADE;"`
}
