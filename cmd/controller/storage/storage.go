/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package storage

import (
	"errors"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

type AggregatedData struct {
	Agents               int
	AgentsByStatus       []int
	Sessions             int
	SessionsByStatus     []int
	Gpus                 int
	GpusByGpuName        map[string]int
	Vram                 uint64
	VramByGpuName        map[string]uint64
	VramUsed             uint64
	VramUsedByGpuName    map[string]uint64
	Utilization          float64
	UtilizationByGpuName map[string]float64
	PowerDraw            float64
	PowerDrawByGpuName   map[string]float64
}

type QueuedSession struct {
	Id           string
	Requirements restapi.SessionRequirements
}

type Iterator[T any] interface {
	Next() bool
	Value() T
}

type Storage interface {
	Close() error

	AggregateData() (AggregatedData, error)

	RegisterAgent(agent restapi.Agent) (string, error)
	GetAgentById(id string) (restapi.Agent, error)
	UpdateAgent(update restapi.AgentUpdate) error

	RequestSession(requirements restapi.SessionRequirements) (string, error)
	AssignSession(sessionId string, agentId string, gpus []restapi.SessionGpu) error
	GetSessionById(id string) (restapi.Session, error)
	GetQueuedSessionById(id string) (QueuedSession, error) // For Testing

	GetAgents() (Iterator[restapi.Agent], error)
	GetAvailableAgentsMatching(totalAvailableVramAtLeast uint64, tags map[string]string, tolerates map[string]string) (Iterator[restapi.Agent], error)
	GetQueuedSessionsIterator() (Iterator[QueuedSession], error)

	SetAgentsMissingIfNotUpdatedFor(duration time.Duration) error
	RemoveMissingAgentsIfNotUpdatedFor(duration time.Duration) error
}

var (
	ErrNotFound = errors.New("object not found")
)

func IsSubset(set, subset map[string]string) bool {
	for key, value := range subset {
		checkValue, present := set[key]
		if !present || value != checkValue {
			return false
		}
	}

	return true
}

func TotalVram(gpus []restapi.Gpu) uint64 {
	var vram uint64
	for _, gpu := range gpus {
		vram += gpu.Vram
	}

	return vram
}

func TotalVramRequired(requirements restapi.SessionRequirements) uint64 {
	var vramRequired uint64
	for _, gpu := range requirements.Gpus {
		vramRequired += gpu.VramRequired
	}

	return vramRequired
}
