/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package memdb

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-memdb"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Agent struct {
	restapi.Agent

	SessionIds    []string
	VramAvailable uint64

	LastUpdated int64
}

type Session struct {
	restapi.Session

	AgentId      string
	Requirements restapi.SessionRequirements
	VramRequired uint64

	LastUpdated int64
}

type storageDriver struct {
	ctx context.Context
	db  *memdb.MemDB
}

type Iterator[T any] struct {
	index   int
	objects []T
}

func NewIterator[T any](objects []T) storage.Iterator[T] {
	return &Iterator[T]{
		index:   -1,
		objects: objects,
	}
}

func (iterator *Iterator[T]) Next() bool {
	index := iterator.index + 1
	if index >= len(iterator.objects) {
		return false
	}

	iterator.index = index
	return true
}

func (iterator *Iterator[T]) Value() T {
	return iterator.objects[iterator.index]
}

func OpenStorage(ctx context.Context) (storage.Storage, error) {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"agents": {
				Name: "agents",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Id"},
					},
					"state": {
						Name:    "state",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "State"},
					},
					"last_updated": {
						Name:    "last_updated",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "LastUpdated"},
					},
				},
			},
			"sessions": {
				Name: "sessions",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.UUIDFieldIndex{Field: "Id"},
					},
					"state": {
						Name:    "state",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "State"},
					},
					"last_updated": {
						Name:    "last_updated",
						Unique:  false,
						Indexer: &memdb.IntFieldIndex{Field: "LastUpdated"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	return &storageDriver{
		ctx: ctx,
		db:  db,
	}, nil
}

func (driver *storageDriver) Close() error {
	return nil
}

func (driver *storageDriver) RegisterAgent(apiAgent restapi.Agent) (string, error) {
	agent := Agent{
		Agent:         apiAgent,
		VramAvailable: storage.TotalVram(apiAgent.Gpus),
		LastUpdated:   time.Now().Unix(),
	}

	agent.Id = uuid.NewString()

	txn := driver.db.Txn(true)
	err := txn.Insert("agents", agent)
	if err != nil {
		txn.Abort()
		return "", err
	}

	txn.Commit()
	return agent.Id, nil
}

func (driver *storageDriver) GetAgentById(id string) (restapi.Agent, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First("agents", "id", id)
	if err != nil {
		return restapi.Agent{}, err
	}

	return utilities.Require[restapi.Agent](obj), nil
}

func (driver *storageDriver) UpdateAgent(update storage.AgentUpdate) error {
	now := time.Now().Unix()

	txn := driver.db.Txn(true)

	obj, err := txn.First("agents", "id", update.Id)
	if err != nil {
		txn.Abort()
		return err
	}

	sessionIndex := 0

	agent := utilities.Require[Agent](obj)
	agent.State = update.State
	agent.LastUpdated = now
	for _, sessionUpdate := range update.Sessions {
		if agent.SessionIds[sessionIndex] != sessionUpdate.Id {
			txn.Abort()
			return errors.New("memdb.UpdateAgent: stored Agent sessions do not match Session updates")
		}

		obj, err = txn.First("sessions", "id", sessionUpdate.Id)
		if err != nil {
			txn.Abort()
			return err
		}
		session := utilities.Require[Session](obj)
		session.State = sessionUpdate.State
		session.LastUpdated = now

		if session.State == restapi.SessionClosed {
			agent.SessionIds = append(agent.SessionIds[:sessionIndex], agent.SessionIds[sessionIndex+1:]...)
			agent.Sessions = append(agent.Sessions[:sessionIndex], agent.Sessions[sessionIndex+1:]...)
			agent.VramAvailable += session.VramRequired

			_, err = txn.DeleteAll("sessions", "id", session.Id)
		} else {
			sessionIndex++

			err = txn.Insert("sessions", session)
		}

		if err != nil {
			txn.Abort()
			return err
		}
	}

	err = txn.Insert("agents", agent)
	if err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func (driver *storageDriver) RequestSession(requirements restapi.SessionRequirements) (string, error) {
	session := Session{
		Session: restapi.Session{
			Id: uuid.NewString(),
		},
		Requirements: requirements,
		VramRequired: storage.TotalVramRequired(requirements),
		LastUpdated:  time.Now().Unix(),
	}

	txn := driver.db.Txn(true)

	err := txn.Insert("sessions", session)
	if err != nil {
		txn.Abort()
		return "", err
	}

	txn.Commit()
	return session.Id, nil
}

func (driver *storageDriver) AssignSession(sessionId string, agentId string, gpus []restapi.SessionGpu) error {
	now := time.Now().Unix()

	txn := driver.db.Txn(true)

	obj, err := txn.First("agents", "id", agentId)
	if err != nil {
		return err
	}
	agent := utilities.Require[Agent](obj)

	obj, err = txn.First("sessions", "id", sessionId)
	if err != nil {
		return err
	}
	session := utilities.Require[Session](obj)
	session.AgentId = agentId
	session.Address = agent.Address
	session.Gpus = gpus
	session.LastUpdated = now

	err = txn.Insert("sessions", session)
	if err != nil {
		txn.Abort()
		return err
	}

	agent.Sessions = append(agent.Sessions, session.Session)
	agent.SessionIds = append(agent.SessionIds, sessionId)
	agent.VramAvailable -= session.VramRequired
	agent.LastUpdated = now

	err = txn.Insert("agents", agent)
	if err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func (driver *storageDriver) GetSessionById(id string) (restapi.Session, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First("sessions", "id", id)
	if err != nil {
		return restapi.Session{}, err
	}

	return utilities.Require[restapi.Session](obj), nil
}

func (driver *storageDriver) GetAvailableAgentsMatching(totalAvailableVramMoreThan uint64, tags map[string]string, tolerates map[string]string) (storage.Iterator[restapi.Agent], error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "state", restapi.AgentActive)
	if err != nil {
		return nil, err
	}

	var agents []restapi.Agent
	for obj := iterator.Next(); obj != nil; iterator.Next() {
		agent := utilities.Require[Agent](obj)

		if agent.VramAvailable >= totalAvailableVramMoreThan && storage.IsSubset(agent.Tags, tags) && storage.IsSubset(agent.Taints, tolerates) {
			agents = append(agents, agent.Agent)
		}
	}

	return NewIterator(agents), nil
}

func (driver *storageDriver) GetQueuedSessionsIterator() (storage.Iterator[storage.QueuedSession], error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("sessions", "state", restapi.SessionQueued)
	if err != nil {
		return nil, err
	}

	var sessions []storage.QueuedSession
	for obj := iterator.Next(); obj != nil; iterator.Next() {
		session := utilities.Require[Session](obj)
		sessions = append(sessions, storage.QueuedSession{
			Id:           session.Id,
			Requirements: session.Requirements,
		})
	}

	return NewIterator(sessions), nil
}

func (driver *storageDriver) SetAgentsMissingIfNotUpdatedFor(duration time.Duration) error {
	nowTime := time.Now()
	now := nowTime.Unix()
	since := nowTime.Add(-duration).Unix()

	txn := driver.db.Txn(true)

	iterator, err := txn.LowerBound("agents", "last_updated", since)
	if err != nil {
		txn.Abort()
		return err
	}

	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)
		agent.State = restapi.AgentMissing
		agent.LastUpdated = now

		err = txn.Insert("agents", agent)
		if err != nil {
			txn.Abort()
			return err
		}
	}

	txn.Commit()
	return nil
}

func (driver *storageDriver) RemoveMissingAgentsIfNotUpdatedFor(duration time.Duration) error {
	since := time.Now().Add(-duration).Unix()

	txn := driver.db.Txn(true)

	iterator, err := txn.LowerBound("agents", "last_updated", since)
	if err != nil {
		txn.Abort()
		return err
	}

	agentIds := make([]interface{}, 0)
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)
		agentIds = append(agentIds, agent.Id)
	}

	_, err = txn.DeleteAll("agents", "id", agentIds...)
	if err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}
