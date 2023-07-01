/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"crypto/tls"
	"flag"
	"os"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	address = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
)

type Frontend struct {
	startTime time.Time

	hostname string

	server  *server.Server
	storage storage.Storage
}

func NewFrontend(tlsConfig *tls.Config, storage storage.Storage) (*Frontend, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	frontend := &Frontend{
		startTime: time.Now(),
		hostname:  hostname,
		server:    server.NewServer(*address, tlsConfig),
		storage:   storage,
	}

	frontend.initializeEndpoints()

	return frontend, nil
}

func (frontend *Frontend) TimeSinceStart() time.Duration {
	// TODO: This needs to be done in the database in some way
	return time.Since(frontend.startTime)
}

func (frontend *Frontend) Run(group task.Group) error {
	group.Go("Frontend Server", frontend.server)
	return nil
}

func (frontend *Frontend) GetActiveAgents() ([]restapi.Agent, error) {
	agents, err := frontend.storage.GetActiveAgents()
	if err != nil {
		return nil, err
	}

	apiAgents := make([]restapi.Agent, len(agents))
	for index, agent := range agents {
		apiAgents[index] = agent.Agent
	}

	return apiAgents, nil
}

func (frontend *Frontend) RegisterAgent(agent restapi.Agent) (string, error) {
	storageAgent := storage.Agent{
		Agent:       agent,
		LastUpdated: time.Now().UTC(),
	}

	id, err := frontend.storage.AddAgent(storageAgent)
	if err != nil {
		return id, err
	}

	storageAgent.Id = id

	return id, frontend.storage.UpdateAgentsAndSessions([]storage.Agent{storageAgent}, nil)
}

func (frontend *Frontend) UpdateAgent(agent restapi.Agent) error {
	now := time.Now().UTC()

	storageAgents := []storage.Agent{
		storage.Agent{
			Agent:       agent,
			LastUpdated: now,
		},
	}

	storageSessions := []storage.Session{}
	for _, session := range agent.Sessions {
		storageSessions = append(storageSessions, storage.Session{
			Session:     session,
			AgentId:     agent.Id,
			LastUpdated: now,
		})
	}

	return frontend.storage.UpdateAgentsAndSessions(storageAgents, storageSessions)
}

func (frontend *Frontend) RequestSession(sessionRequirements restapi.SessionRequirements) (restapi.Session, error) {
	storageSession := storage.Session{
		Session: restapi.Session{
			Version: sessionRequirements.Version,
		},
		GpuRequirements: sessionRequirements.Gpus,
		LastUpdated:     time.Now().UTC(),
	}

	id, err := frontend.storage.AddSession(storageSession)
	if err != nil {
		return restapi.Session{}, err
	}

	storageSession.Id = id

	return storageSession.Session, frontend.storage.UpdateAgentsAndSessions(nil, []storage.Session{storageSession})
}

func (frontend *Frontend) GetSession(id string) (restapi.Session, error) {
	session, err := frontend.storage.GetSessionById(id)
	return session.Session, err
}
