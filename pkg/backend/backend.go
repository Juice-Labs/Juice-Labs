/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package backend

import (
	"context"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/backend/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/backend/storage/postgres"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Agent struct {
	storage.Agent

	ActiveSessions []*Session
	GpuSet         gpu.GpuSet
}

type Session struct {
	storage.Session

	AssignedAgent *Agent
	GpuSet        gpu.SelectedGpuSet
}

// This is essentially an in-memory database
type Backend struct {
	ctx context.Context

	storage storage.Storage

	lastUpdated time.Time

	agentsToUpdate   []storage.Agent
	sessionsToUpdate []storage.Session

	agents   *utilities.LinkedList[Agent]
	sessions *utilities.LinkedList[Session]

	// Indexes
	agentsById   map[string]*Agent
	sessionsById map[string]*Session
}

func NewBackend(ctx context.Context) (*Backend, error) {
	storage, err := postgres.NewStorage(ctx)
	if err != nil {
		return nil, err
	}

	return &Backend{
		storage: storage,

		agents:   utilities.NewLinkedList[Agent](),
		sessions: utilities.NewLinkedList[Session](),

		agentsById:   map[string]*Agent{},
		sessionsById: map[string]*Session{},
	}, nil
}

func (backend *Backend) Close() error {
	return backend.storage.Close()
}

func (backend *Backend) RegisterAgent(agent restapi.Agent) (string, error) {
	storageAgent := storage.Agent{
		Agent:       agent,
		LastUpdated: time.Now().UTC(),
	}

	id, err := backend.storage.AddAgent(storageAgent)
	if err != nil {
		return id, err
	}

	storageAgent.Id = id

	return id, backend.storage.UpdateAgentsAndSessions([]storage.Agent{storageAgent}, nil)
}

func (backend *Backend) UpdateAgent(agent restapi.Agent) error {
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

	return backend.storage.UpdateAgentsAndSessions(storageAgents, storageSessions)
}

func (backend *Backend) RequestSession(request restapi.RequestSession) (restapi.Session, error) {
	storageSession := storage.Session{
		Session: restapi.Session{
			Version: request.Version,
		},
		GpuRequirements: request.Gpus,
		LastUpdated:     time.Now().UTC(),
	}

	id, err := backend.storage.AddSession(storageSession)
	if err != nil {
		return restapi.Session{}, err
	}

	storageSession.Id = id

	return storageSession.Session, backend.storage.UpdateAgentsAndSessions(nil, []storage.Session{storageSession})
}

func (backend *Backend) GetSession(id string) (restapi.Session, error) {
	session, err := backend.storage.GetSessionById(id)
	return session.Session, err
}

func (backend *Backend) Update() error {
	now := time.Now().UTC()

	err := backend.updateAgentsAndSessionsSince(backend.lastUpdated)
	if err != nil {
		return err
	}

	inactiveTime := now.Add(-time.Minute * 100)

	agentIterator := backend.agents.Iterator()
	for agentIterator.Next() {
		updated := false

		agent := agentIterator.Value()

		// Has the session been inactive long enough to switch states?
		if agent.State < restapi.StateInactive && agent.LastUpdated.Before(inactiveTime) {
			agent.State = restapi.StateInactive
			updated = true

			// Update all the agents sessions as well
			for index, session := range agent.ActiveSessions {
				agent.Sessions[index].State = restapi.StateInactive
				session.State = restapi.StateInactive

				// Session will be removed when iterating over the sessions
			}
		}

		// If the state is inactive or closed, remove them from in-memory
		switch agent.State {
		case restapi.StateInactive:
			agentIterator = backend.removeAgent(agentIterator)
			updated = true
		}

		if updated {
			backend.agentsToUpdate = append(backend.agentsToUpdate, agent.Agent)
		}
	}

	sessionIterator := backend.sessions.Iterator()
	for sessionIterator.Next() {
		updated := false

		session := sessionIterator.Value()

		// Has the session been inactive long enough to switch states?
		if session.State < restapi.StateInactive && session.LastUpdated.Before(inactiveTime) {
			session.State = restapi.StateInactive
			updated = true
		}

		// If the state is inactive or closed, remove them from in-memory
		switch session.State {
		case restapi.StateQueued:
			if backend.tryAssigningSession(session) {
				updated = true
			}

		case restapi.StateInactive:
		case restapi.StateClosed:
			sessionIterator = backend.removeSession(sessionIterator)
			updated = true
		}

		if updated {
			if session.AssignedAgent != nil {
				backend.agentsToUpdate = append(backend.agentsToUpdate, session.AssignedAgent.Agent)
			}

			backend.sessionsToUpdate = append(backend.sessionsToUpdate, session.Session)
		}
	}

	err = backend.storage.UpdateAgentsAndSessions(backend.agentsToUpdate, backend.sessionsToUpdate)
	if err != nil {
		return err
	}

	backend.lastUpdated = now
	return nil
}

func (backend *Backend) updateAgentsAndSessionsSince(since time.Time) error {
	agents, sessions, err := backend.storage.GetAgentsAndSessionsUpdatedSince(since)
	if err != nil {
		return err
	}

	// Update all the agents
	for _, agent := range agents {
		agentById, found := backend.agentsById[agent.Id]
		if !found {
			node := backend.agents.Append(Agent{
				Agent: agent,
			})

			agentById = &node.Data
			backend.agentsById[agent.Id] = agentById
		} else {
			agentById.Agent = agent
			agentById.ActiveSessions = nil
		}
	}

	// Update all the sessions
	for _, session := range sessions {
		sessionById, found := backend.sessionsById[session.Id]
		if !found {
			node := backend.sessions.Append(Session{
				Session: session,
			})

			sessionById = &node.Data
			backend.sessionsById[session.Id] = sessionById
		} else {
			sessionById.Session = session
			sessionById.AgentId = ""
			sessionById.AssignedAgent = nil
		}
	}

	// Go back through the agents and update the pointers
	for _, agent := range agents {
		agentById, found := backend.agentsById[agent.Id]
		if !found {
			logger.Panic("agent should have been found")
		} else {
			agentById.GpuSet = gpu.NewGpuSet(agentById.Gpus)

			for _, session := range agentById.Sessions {
				sessionById, found := backend.sessionsById[session.Id]
				if !found {
					logger.Panic("session should have been found")
				}

				selectedGpus, err := agentById.GpuSet.Select(sessionById.Gpus)
				if err != nil {
					logger.Panic(err)
				}

				sessionById.AgentId = agent.Id
				sessionById.AssignedAgent = agentById
				sessionById.GpuSet = selectedGpus
				agentById.ActiveSessions = append(agentById.ActiveSessions, sessionById)
			}
		}
	}

	return nil
}

func (backend *Backend) tryAssigningSession(session *Session) bool {
	agentIterator := backend.agents.Iterator()
	for agentIterator.Next() {
		agent := agentIterator.Value()

		// Check to make sure this agent can run another session
		if (len(agent.Sessions) + 1) <= agent.MaxSessions {
			// Check all the requirements
			selectedGpus, err := agent.GpuSet.Find(session.GpuRequirements)
			if err == nil {
				session.State = restapi.StateAssigned
				session.AgentId = agent.Id
				session.Address = agent.Address
				session.AssignedAgent = agent
				session.GpuSet = selectedGpus
				session.Gpus = selectedGpus.GetGpus()

				agent.Sessions = append(agent.Sessions, session.Session.Session)
				agent.ActiveSessions = append(agent.ActiveSessions, session)

				return true
			}
		}
	}

	return false
}

func (backend *Backend) removeAgent(iterator utilities.NodeIterator[Agent]) utilities.NodeIterator[Agent] {
	id := iterator.Value().Id

	delete(backend.agentsById, id)

	return backend.agents.Remove(iterator)
}

func (backend *Backend) removeSession(iterator utilities.NodeIterator[Session]) utilities.NodeIterator[Session] {
	id := iterator.Value().Id

	delete(backend.sessionsById, id)

	return backend.sessions.Remove(iterator)
}

func (backend *Backend) removeSessionNode(node *utilities.Node[Session]) {
	delete(backend.sessionsById, node.Data.Id)

	backend.sessions.RemoveNode(node)
}
