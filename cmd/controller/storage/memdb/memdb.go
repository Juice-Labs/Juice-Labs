/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package memdb

import (
	"context"
	"sort"
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
						Indexer: &memdb.StringFieldIndex{Field: "State"},
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
						Indexer: &memdb.StringFieldIndex{Field: "State"},
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

func (driver *storageDriver) AggregateData() (storage.AggregatedData, error) {
	txn := driver.db.Snapshot().Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "id")
	if err != nil {
		return storage.AggregatedData{}, err
	}

	data := storage.AggregatedData{
		AgentsByStatus:           map[string]int{},
		SessionsByStatus:         map[string]int{},
		GpusByGpuName:            map[string]int{},
		VramByGpuName:            map[string]uint64{},
		VramUsedByGpuName:        map[string]uint64{},
		VramGBAvailableByGpuName: map[string]storage.Percentile[int]{},
		UtilizationByGpuName:     map[string]float64{},
		PowerDrawByGpuName:       map[string]float64{},
	}

	vramGBAvailable := map[int]int{}
	vramGBAvailableByGpuName := map[string]map[int]int{}

	var utilization uint64
	utilizationByGpuName := map[string]uint64{}

	var powerDraw uint64
	powerDrawByGpuName := map[string]uint64{}

	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)

		data.Agents++
		data.AgentsByStatus[agent.State]++

		data.Sessions += len(agent.Sessions)
		for _, session := range agent.Sessions {
			data.SessionsByStatus[session.State]++
		}

		data.Gpus += len(agent.Gpus)
		for _, gpu := range agent.Gpus {
			data.GpusByGpuName[gpu.Name]++
			data.Vram += gpu.Vram
			data.VramByGpuName[gpu.Name] += gpu.Vram
			data.VramUsed += gpu.Metrics.VramUsed
			data.VramUsedByGpuName[gpu.Name] += gpu.Metrics.VramUsed

			gb := int((gpu.Vram - gpu.Metrics.VramUsed) / (1024 * 1024 * 1024))
			vramGBAvailable[gb]++

			if _, ok := vramGBAvailableByGpuName[gpu.Name]; !ok {
				vramGBAvailableByGpuName[gpu.Name] = map[int]int{}
			}

			vramGBAvailableByGpuName[gpu.Name][gb]++

			utilization += uint64(gpu.Metrics.UtilizationGpu)
			utilizationByGpuName[gpu.Name] += uint64(gpu.Metrics.UtilizationGpu)
			powerDraw += uint64(gpu.Metrics.PowerDraw)
			powerDrawByGpuName[gpu.Name] += uint64(gpu.Metrics.PowerDraw)
		}
	}

	if data.Gpus > 0 {
		calculatePercentiles := func(counts map[int]int, total int) storage.Percentile[int] {
			sortedKeys := []int{}
			for key := range counts {
				sortedKeys = append(sortedKeys, key)
			}
			sort.Ints(sortedKeys)

			indexP90 := int(float64(total) * 0.90)
			indexP75 := int(float64(total) * 0.75)
			indexP50 := int(float64(total) * 0.50)
			indexP25 := int(float64(total) * 0.25)
			indexP10 := int(float64(total) * 0.10)

			percentile := storage.Percentile[int]{
				P100: sortedKeys[len(sortedKeys)-1],
			}

			index := 0
			keysIndex := 0
			key := sortedKeys[keysIndex]
			for keysIndex < len(sortedKeys) && index < indexP10 {
				index += counts[key]
				keysIndex++
				key = sortedKeys[keysIndex]
			}
			percentile.P10 = key

			for keysIndex < len(sortedKeys) && index < indexP25 {
				index += counts[key]
				keysIndex++
				key = sortedKeys[keysIndex]
			}
			percentile.P25 = key

			for keysIndex < len(sortedKeys) && index < indexP50 {
				index += counts[key]
				keysIndex++
				key = sortedKeys[keysIndex]
			}
			percentile.P50 = key

			for keysIndex < len(sortedKeys) && index < indexP75 {
				index += counts[key]
				keysIndex++
				key = sortedKeys[keysIndex]
			}
			percentile.P75 = key

			for keysIndex < len(sortedKeys) && index < indexP90 {
				index += counts[key]
				keysIndex++
				key = sortedKeys[keysIndex]
			}
			percentile.P90 = key

			return percentile
		}

		data.VramGBAvailable = calculatePercentiles(vramGBAvailable, data.Gpus)
		for key, gbAvailable := range vramGBAvailableByGpuName {
			data.VramGBAvailableByGpuName[key] = calculatePercentiles(gbAvailable, data.GpusByGpuName[key])
		}

		data.Utilization = float64(utilization) / float64(data.Gpus)
		for key, value := range utilizationByGpuName {
			data.UtilizationByGpuName[key] = float64(value) / float64(data.Gpus)
		}

		data.PowerDraw = float64(powerDraw) / float64(data.Gpus) / 1000.0
		for key, value := range powerDrawByGpuName {
			data.PowerDrawByGpuName[key] = float64(value) / float64(data.Gpus) / 1000.0
		}
	}

	return data, nil
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

	if obj == nil {
		return restapi.Agent{}, storage.ErrNotFound
	}

	return utilities.Require[Agent](obj).Agent, nil
}

func (driver *storageDriver) UpdateAgent(update restapi.AgentUpdate) error {
	now := time.Now().Unix()

	txn := driver.db.Txn(true)

	obj, err := txn.First("agents", "id", update.Id)
	if err != nil {
		txn.Abort()
		return err
	}

	agent := utilities.Require[Agent](obj)
	if update.State != "" {
		agent.State = update.State
	}

	agent.LastUpdated = now

	if agent.State != restapi.AgentClosed {
		sessionIds := make([]string, 0, len(agent.SessionIds))
		sessions := make([]restapi.Session, 0, len(agent.Sessions))

		for index, sessionId := range agent.SessionIds {
			// TODO: Handle closing sessions
			// This should be handled by the controller or agent

			// sessionUpdate, present := update.Sessions[sessionId]
			// if present {
			// 	// First, update the session information within the agent structure
			// 	agent.Sessions[index].State = sessionUpdate.State

			// 	// Next, update the session object itself
			// 	obj, err = txn.First("sessions", "id", sessionId)
			// 	if err != nil {
			// 		txn.Abort()
			// 		return err
			// 	}
			// 	session := utilities.Require[Session](obj)
			// 	session.State = sessionUpdate.State
			// 	session.LastUpdated = now

			// 	if session.State == restapi.SessionClosed {
			// 		agent.VramAvailable += session.VramRequired
			// 	} else {
			// 		sessionIds = append(sessionIds, sessionId)
			// 		sessions = append(sessions, session.Session)
			// 	}

			// 	err = txn.Insert("sessions", session)
			// 	if err != nil {
			// 		txn.Abort()
			// 		return err
			// 	}
			// } else {
			sessionIds = append(sessionIds, sessionId)
			sessions = append(sessions, agent.Sessions[index])
			// }
		}

		for index, gpuMetrics := range update.Gpus {
			agent.Gpus[index].Metrics = gpuMetrics
		}

		agent.SessionIds = sessionIds
		agent.Sessions = sessions

		err = txn.Insert("agents", agent)
		if err != nil {
			txn.Abort()
			return err
		}
	} else {
		sessionIds := make([]interface{}, len(agent.SessionIds))
		for index, id := range agent.SessionIds {
			sessionIds[index] = id
		}

		_, err = txn.DeleteAll("sessions", "id", sessionIds...)
		if err != nil {
			txn.Abort()
			return err
		}

		_, err = txn.DeleteAll("agents", "id", agent.Id)
		if err != nil {
			txn.Abort()
			return err
		}
	}

	txn.Commit()
	return nil
}

func (driver *storageDriver) RequestSession(requirements restapi.SessionRequirements) (string, error) {
	session := Session{
		Session: restapi.Session{
			Id:      uuid.NewString(),
			Version: requirements.Version,
			State:   restapi.SessionQueued,
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
		txn.Abort()
		return err
	}
	session := utilities.Require[Session](obj)
	session.State = restapi.SessionAssigned
	// session.ExitStatus = restapi.ExitStatusUnknown
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

func (driver *storageDriver) CancelSession(sessionId string) error {
	txn := driver.db.Txn(true)

	obj, err := txn.First("sessions", "id", sessionId)
	if err != nil {
		txn.Abort()
		return err
	}
	session := utilities.Require[Session](obj)
	if session.AgentId == "" {
		session.State = restapi.SessionClosed
		// session.ExitStatus = restapi.ExitStatusCanceled
	} else {
		session.State = restapi.SessionCanceling
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

	if obj == nil {
		return restapi.Session{}, storage.ErrNotFound
	}

	return utilities.Require[Session](obj).Session, nil
}

func (driver *storageDriver) GetQueuedSessionById(id string) (storage.QueuedSession, error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	obj, err := txn.First("sessions", "id", id)
	if err != nil {
		return storage.QueuedSession{}, err
	}

	if obj == nil {
		return storage.QueuedSession{}, storage.ErrNotFound
	}

	session := utilities.Require[Session](obj)

	return storage.QueuedSession{
		Id:           session.Id,
		Requirements: session.Requirements,
	}, nil
}

func (driver *storageDriver) GetAgents() (storage.Iterator[restapi.Agent], error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "id")
	if err != nil {
		return nil, err
	}

	var agents []restapi.Agent
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)
		if agent.State == restapi.AgentActive {
			agents = append(agents, agent.Agent)
		}
	}

	return storage.NewDefaultIterator(agents), nil
}

func (driver *storageDriver) GetAvailableAgentsMatching(totalAvailableVramAtLeast uint64) (storage.Iterator[restapi.Agent], error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("agents", "state", restapi.AgentActive)
	if err != nil {
		return nil, err
	}

	var agents []restapi.Agent
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)

		if agent.VramAvailable >= totalAvailableVramAtLeast {
			agents = append(agents, agent.Agent)
		}
	}

	return storage.NewDefaultIterator(agents), nil
}

func (driver *storageDriver) GetQueuedSessionsIterator() (storage.Iterator[storage.QueuedSession], error) {
	txn := driver.db.Txn(false)
	defer txn.Abort()

	iterator, err := txn.Get("sessions", "state", restapi.SessionQueued)
	if err != nil {
		return nil, err
	}

	var sessions []storage.QueuedSession
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		session := utilities.Require[Session](obj)
		sessions = append(sessions, storage.QueuedSession{
			Id:           session.Id,
			Requirements: session.Requirements,
		})
	}

	return storage.NewDefaultIterator(sessions), nil
}

func (driver *storageDriver) SetAgentsMissingIfNotUpdatedFor(duration time.Duration) error {
	nowTime := time.Now()
	now := nowTime.Unix()
	since := nowTime.Add(-duration).Unix()

	txn := driver.db.Txn(true)

	iterator, err := txn.ReverseLowerBound("agents", "last_updated", since)
	if err != nil {
		txn.Abort()
		return err
	}

	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)
		if agent.State == restapi.AgentActive {
			agent.State = restapi.AgentMissing
			agent.LastUpdated = now

			err = txn.Insert("agents", agent)
			if err != nil {
				txn.Abort()
				return err
			}
		}
	}

	txn.Commit()
	return nil
}

func (driver *storageDriver) RemoveMissingAgentsIfNotUpdatedFor(duration time.Duration) error {
	since := time.Now().Add(-duration).Unix()

	txn := driver.db.Txn(true)

	iterator, err := txn.ReverseLowerBound("agents", "last_updated", since)
	if err != nil {
		txn.Abort()
		return err
	}

	agentIds := make([]interface{}, 0)
	for obj := iterator.Next(); obj != nil; obj = iterator.Next() {
		agent := utilities.Require[Agent](obj)
		if agent.State == restapi.AgentMissing {
			agentIds = append(agentIds, agent.Id)
		}
	}

	if len(agentIds) > 0 {
		_, err = txn.DeleteAll("agents", "id", agentIds...)
		if err != nil {
			txn.Abort()
			return err
		}

		txn.Commit()
	} else {
		txn.Abort()
	}

	return nil
}
