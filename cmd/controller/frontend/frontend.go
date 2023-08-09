/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	overrideHostname = flag.String("override-hostname", "", "")
	webhook          = flag.String("webhook-url", "", "")
)

type Frontend struct {
	startTime time.Time

	hostname      string
	webhookClient restapi.Client

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

	if *webhook != "" {
		frontend.webhookClient = restapi.Client{
			Client:  &http.Client{},
			Address: *webhook,
		}
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

func (frontend *Frontend) updateAgent(ctx context.Context, update restapi.AgentUpdate) error {
	err := frontend.storage.UpdateAgent(update)
	if err == nil && len(update.Sessions) > 0 {
		for sessionId, session := range update.Sessions {
			response, err_ := frontend.webhookClient.Post(ctx, fmt.Sprintf("/v1/session/update?agentId=%s&sessionId=%s&newState=%s", update.Id, sessionId, session.State))
			err = err_
			if response != nil {
				err = errors.Join(err, response.Body.Close())
			}
		}
	}

	return err
}

func (frontend *Frontend) requestSession(sessionRequirements restapi.SessionRequirements) (string, error) {
	return frontend.storage.RequestSession(sessionRequirements)
}

func (frontend *Frontend) getSessionById(id string) (restapi.Session, error) {
	return frontend.storage.GetSessionById(id)
}
