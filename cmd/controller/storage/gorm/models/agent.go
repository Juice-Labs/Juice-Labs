package models

import (
	"fmt"
	"strings"

	uuid "github.com/satori/go.uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AgentState int

const (
	AgentStateUnknown AgentState = iota
	AgentStateActive
	AgentStateDisabled
	AgentStateMissing
	AgentStateClosed
)

var (
	agentStateMappings = map[string]AgentState{
		"unknown":  AgentStateUnknown,
		"active":   AgentStateActive,
		"disabled": AgentStateDisabled,
		"missing":  AgentStateMissing,
		"closed":   AgentStateClosed,
	}
)

func AgentStateFromString(value string) AgentState {
	status, ok := agentStateMappings[strings.ToLower(value)]
	if !ok {
		return AgentStateUnknown
	}
	return status
}

func (as AgentState) String() string {
	switch as {
	case AgentStateUnknown:
		return "unknown"
	case AgentStateActive:
		return "active"
	case AgentStateDisabled:
		return "disabled"
	case AgentStateMissing:
		return "missing"
	case AgentStateClosed:
		return "closed"
	}
	panic(fmt.Sprintf("invalid AgentState, %d", as))
}

type KeyValue struct {
	ID    uint
	Key   string `gorm:"notnull"`
	Value string `gorm:"notnull"`
}

type Agent struct {
	gorm.Model

	State         AgentState
	UUID          uuid.UUID `gorm:"type:uuid;notnull;unique"`
	Hostname      string
	Address       string
	Version       string
	Gpus          datatypes.JSON
	VramAvailable uint64

	Labels   []KeyValue `gorm:"many2many:agent_labels;constraint:OnDelete:CASCADE;"`
	Taints   []KeyValue `gorm:"many2many:agent_taints;constraint:OnDelete:CASCADE;"`
	Sessions []Session
}
