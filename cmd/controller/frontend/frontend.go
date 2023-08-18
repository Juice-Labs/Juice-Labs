/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"flag"
	"os"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	overrideHostname = flag.String("override-hostname", "", "")
)

type Frontend struct {
	startTime time.Time

	hostname string

	storage storage.Storage
}

func NewFrontend(server *server.Server, storage storage.Storage) (*Frontend, error) {
	hostname := *overrideHostname
	if hostname == "" {
		hostname_, err := os.Hostname()
		if err != nil {
			return nil, err
		}

		hostname = hostname_
	}

	frontend := &Frontend{
		startTime: time.Now(),
		hostname:  hostname,
		storage:   storage,
	}

	frontend.initializeEndpoints(server)

	return frontend, nil
}

func (frontend *Frontend) Run(group task.Group) error {
	return nil
}

func (frontend *Frontend) registerAgent(agent restapi.Agent) (string, error) {
	agent.State = restapi.AgentActive
	return frontend.storage.RegisterAgent(agent)
}

func (frontend *Frontend) getAgents() ([]restapi.Agent, error) {
	iterator, err := frontend.storage.GetAgents()
	if err != nil {
		return nil, err
	}

	agents := make([]restapi.Agent, 0)
	for iterator.Next() {
		agents = append(agents, iterator.Value())
	}

	return agents, nil
}

func (frontend *Frontend) getAgentById(id string) (restapi.Agent, error) {
	return frontend.storage.GetAgentById(id)
}

func (frontend *Frontend) updateAgent(update restapi.AgentUpdate) error {
	return frontend.storage.UpdateAgent(update)
}

func (frontend *Frontend) requestSession(sessionRequirements restapi.SessionRequirements) (string, error) {
	return frontend.storage.RequestSession(sessionRequirements)
}

func (frontend *Frontend) getSessionById(id string) (restapi.Session, error) {
	return frontend.storage.GetSessionById(id)
}

func (frontend *Frontend) cancelSession(id string) error {
	return frontend.storage.CancelSession(id)
}
