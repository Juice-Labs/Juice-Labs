/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package storage

import (
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/api"
)

type Agent struct {
	api.Agent

	LastUpdated time.Time
}

type Session struct {
	api.Session

	AgentId         string
	GpuRequirements []api.GpuRequirements `json:"gpuRequirements"`

	LastUpdated time.Time
}

type Storage interface {
	Close() error

	AddAgent(Agent) (string, error)
	AddSession(Session) (string, error)
	UpdateAgentsAndSessions([]Agent, []Session) error

	GetSessionById(id string) (Session, error)
	GetAgentsAndSessionsUpdatedSince(time.Time) ([]Agent, []Session, error)
}
