/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package backend

import (
	"context"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/memdb"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

func openStorage(t *testing.T) storage.Storage {
	db, err := memdb.OpenStorage(context.Background())
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return db
}

func defaultAgent(maxSessions int, gpuVram uint64) restapi.Agent {
	return restapi.Agent{
		State:       restapi.AgentActive,
		Hostname:    "Test",
		Address:     "127.0.0.1:43210",
		Version:     "Test",
		MaxSessions: maxSessions,
		Gpus: []restapi.Gpu{
			restapi.Gpu{
				Index:       0,
				Name:        "Test",
				VendorId:    0x0001,
				DeviceId:    0x0002,
				SubDeviceId: 0x0003,
				Vram:        gpuVram,
			},
		},
		Tags:   map[string]string{},
		Taints: map[string]string{},
	}
}

func defaultSessionRequirements(gpuVram uint64) restapi.SessionRequirements {
	return restapi.SessionRequirements{
		Version: "Test",
		Gpus: []restapi.GpuRequirements{
			restapi.GpuRequirements{
				VramRequired: gpuVram,
				Tags:         map[string]string{},
				Tolerates:    map[string]string{},
			},
		},
		Tags:      map[string]string{},
		Tolerates: map[string]string{},
	}
}

func createAgent() restapi.Agent {
	agent := restapi.Agent{
		State:       restapi.AgentActive,
		Hostname:    "Test",
		Address:     "127.0.0.1:43210",
		Version:     "Test",
		MaxSessions: rand.Intn(7) + 1,
		Tags:        map[string]string{},
		Taints:      map[string]string{},
	}

	agent.Gpus = make([]restapi.Gpu, rand.Intn(7)+1)
	for index := range agent.Gpus {
		agent.Gpus[index] = restapi.Gpu{
			Index:       index,
			Name:        "Test",
			VendorId:    0x0001,
			DeviceId:    0x0002,
			SubDeviceId: 0x0003,
			Vram:        uint64(rand.Intn(65536+1)) * 1024 * 1024,
		}
	}

	return agent
}

func createSessionRequirements() restapi.SessionRequirements {
	requirements := restapi.SessionRequirements{
		Version:   "Test",
		Tags:      map[string]string{},
		Tolerates: map[string]string{},
	}

	requirements.Gpus = make([]restapi.GpuRequirements, rand.Intn(7)+1)
	for index := range requirements.Gpus {
		requirements.Gpus[index] = restapi.GpuRequirements{
			VramRequired: uint64(rand.Intn(8192+1)) * 1024 * 1024,
			Tags:         map[string]string{},
			Tolerates:    map[string]string{},
		}
	}

	return requirements
}

func compare[T any](t *testing.T, check T, against T, err error) {
	t.Helper()

	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(check, against) {
		t.Error("objects do not match")
	}
}

func checkAgent(t *testing.T, db storage.Storage, check restapi.Agent) {
	t.Helper()
	against, err := db.GetAgentById(check.Id)
	compare(t, check, against, err)
}

func checkSession(t *testing.T, db storage.Storage, check restapi.Session) {
	t.Helper()
	against, err := db.GetSessionById(check.Id)
	compare(t, check, against, err)
}

func checkQueuedSession(t *testing.T, db storage.Storage, check storage.QueuedSession) {
	t.Helper()
	against, err := db.GetQueuedSessionById(check.Id)
	compare(t, check, against, err)
}

func registerAgent(t *testing.T, db storage.Storage, agent restapi.Agent) restapi.Agent {
	id, err := db.RegisterAgent(agent)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	agent.Id = id
	checkAgent(t, db, agent)

	return agent
}

func queueSession(t *testing.T, db storage.Storage, requirements restapi.SessionRequirements) string {
	sessionId, err := db.RequestSession(requirements)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return sessionId
}

func TestAgents(t *testing.T) {
	db := openStorage(t)
	defer db.Close()

	agent := registerAgent(t, db, createAgent())

	time.Sleep(time.Second)

	agent.State = restapi.AgentMissing
	db.SetAgentsMissingIfNotUpdatedFor(0)
	checkAgent(t, db, agent)

	agent.State = restapi.AgentActive
	db.UpdateAgent(storage.AgentUpdate{
		Id:    agent.Id,
		State: restapi.AgentActive,
	})
	checkAgent(t, db, agent)

	time.Sleep(time.Second)

	agent.State = restapi.AgentMissing
	db.SetAgentsMissingIfNotUpdatedFor(0)
	checkAgent(t, db, agent)

	time.Sleep(time.Second)

	db.RemoveMissingAgentsIfNotUpdatedFor(0)
	_, err := db.GetAgentById(agent.Id)
	if err == nil {
		t.Error("expected storage.ErrNotFound, instead did not receive an error")
	} else if err != storage.ErrNotFound {
		t.Errorf("expected storage.ErrNotFound, instead received %s", err)
	}
}

func TestSessions(t *testing.T) {
	db := openStorage(t)
	defer db.Close()

	requirements := createSessionRequirements()
	id := queueSession(t, db, requirements)

	queuedSession := storage.QueuedSession{
		Id:           id,
		Requirements: requirements,
	}

	checkQueuedSession(t, db, queuedSession)

	_, err := db.GetSessionById(id)
	if err == nil {
		t.Error("expected storage.ErrNotSupported, instead did not receive an error")
	} else if err != storage.ErrNotSupported {
		t.Errorf("expected storage.ErrNotSupported, instead received %s", err)
	}
}

func TestAssigningSessions(t *testing.T) {
	db := openStorage(t)
	defer db.Close()

	agent := registerAgent(t, db, defaultAgent(4, 24*1024*1024*1024))

	requirements := defaultSessionRequirements(4 * 1024 * 1024 * 1024)
	sessionId := queueSession(t, db, requirements)

	selectedGpus := []restapi.SessionGpu{
		{
			Gpu:          agent.Gpus[0],
			VramRequired: requirements.Gpus[0].VramRequired,
		},
	}

	err := db.AssignSession(sessionId, agent.Id, selectedGpus)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	session := restapi.Session{
		Id:      sessionId,
		State:   restapi.SessionAssigned,
		Address: agent.Address,
		Version: requirements.Version,
		Gpus:    selectedGpus,
	}
	checkSession(t, db, session)

	agent.Sessions = append(agent.Sessions, session)
	checkAgent(t, db, agent)

	agent.Sessions[0].State = restapi.SessionActive
	session.State = restapi.SessionActive
	db.UpdateAgent(storage.AgentUpdate{
		Id:    agent.Id,
		State: agent.State,
		Sessions: []storage.SessionUpdate{
			{
				Id:    session.Id,
				State: restapi.SessionActive,
			},
		},
	})
	checkAgent(t, db, agent)
	compare(t, session, agent.Sessions[0], nil)
	checkSession(t, db, session)

	agent.Sessions = make([]restapi.Session, 0)
	db.UpdateAgent(storage.AgentUpdate{
		Id:    agent.Id,
		State: agent.State,
		Sessions: []storage.SessionUpdate{
			storage.SessionUpdate{
				Id:    session.Id,
				State: restapi.SessionClosed,
			},
		},
	})
	checkAgent(t, db, agent)
	_, err = db.GetSessionById(session.Id)
	if err == nil {
		t.Error("expected storage.ErrNotFound, instead did not receive an error")
	} else if err != storage.ErrNotFound {
		t.Errorf("expected storage.ErrNotFound, instead received %s", err)
	}
}

func TestGetQueuedSessionsIterator(t *testing.T) {
	db := openStorage(t)
	defer db.Close()

	sessionIds := map[string]restapi.SessionRequirements{}
	for i := 0; i < 4; i++ {
		requirements := defaultSessionRequirements(4 * 1024 * 1024 * 1024)
		sessionIds[queueSession(t, db, requirements)] = requirements
	}

	iterator, err := db.GetQueuedSessionsIterator()
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	for iterator.Next() {
		against := iterator.Value()
		check, present := sessionIds[against.Id]
		if !present {
			t.Error("unexpected session found")
		} else {
			compare(t, check, against.Requirements, nil)
			delete(sessionIds, against.Id)
		}
	}

	if len(sessionIds) != 0 {
		t.Error("sessions not queued")
	}
}

func TestGetAvailableAgentsMatching(t *testing.T) {
	db := openStorage(t)
	defer db.Close()

	backend := NewBackend(db)

	agentIds := []string{
		registerAgent(t, db, defaultAgent(2, 24*1024*1024*1024)).Id,
		registerAgent(t, db, defaultAgent(1, 4*1024*1024*1024)).Id,
		registerAgent(t, db, defaultAgent(1, 4*1024*1024*1024)).Id,
	}

	sessionIds := []string{
		queueSession(t, db, defaultSessionRequirements(8*1024*1024*1024)),
		queueSession(t, db, defaultSessionRequirements(4*1024*1024*1024)),
		queueSession(t, db, defaultSessionRequirements(2*1024*1024*1024)),
		queueSession(t, db, defaultSessionRequirements(4*1024*1024*1024)),
	}

	err := backend.Update(context.Background())
	if err != nil {
		t.Error(err)
	}

	for _, id := range sessionIds {
		session, err := db.GetSessionById(id)
		if err != nil {
			t.Error(err)
		} else if session.State != restapi.SessionAssigned {
			t.Error("expected session to be assigned")
		}
	}

	for _, id := range agentIds {
		agent, err := db.GetAgentById(id)
		if err != nil {
			t.Error(err)
		}

		if len(agent.Sessions) > agent.MaxSessions {
			t.Error("maximum sessions is not adhered to")
		}

		var sessionVram uint64
		for _, session := range agent.Sessions {
			sessionVram += session.Gpus[0].VramRequired

			if session.Address > agent.Address {
				t.Error("session Address is not the agent Address")
			}
		}

		if sessionVram > agent.Gpus[0].Vram {
			t.Error("maximum vram is not adhered to")
		}
	}
}
