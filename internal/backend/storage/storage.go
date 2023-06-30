/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package storage

import (
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

type Agent struct {
	restapi.Agent

	LastUpdated time.Time
}

type Session struct {
	restapi.Session

	AgentId         string
	GpuRequirements []restapi.GpuRequirements `json:"gpuRequirements"`

	LastUpdated time.Time
}

type Storage interface {
	Close() error

	AddAgent(Agent) (string, error)
	AddSession(Session) (string, error)
	GetActiveAgents() ([]Agent, error)
	UpdateAgentsAndSessions([]Agent, []Session) error

	GetSessionById(id string) (Session, error)
	GetAgentsAndSessionsUpdatedSince(time.Time) ([]Agent, []Session, error)
}
