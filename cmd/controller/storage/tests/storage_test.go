/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package tests

import (
	"context"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/memdb"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/postgres"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

func openMemdb(t *testing.T) storage.Storage {
	db, err := memdb.OpenStorage(context.Background())
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return db
}

func openPostgres(t *testing.T) storage.Storage {
	db, err := postgres.OpenStorage(context.Background(), "user=postgres password=password dbname=postgres sslmode=disable")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return db
}

func defaultAgent(gpuVram uint64) restapi.Agent {
	return restapi.Agent{
		State:    restapi.AgentActive,
		Hostname: "Test",
		Address:  "127.0.0.1:43210",
		Version:  "Test",
		Gpus: []restapi.Gpu{
			{
				Index:       0,
				Name:        "Test",
				VendorId:    0x0001,
				DeviceId:    0x0002,
				SubDeviceId: 0x0003,
				Vram:        gpuVram,
			},
		},
		Labels: map[string]string{
			"Key1": "Value1",
			"Key2": "Value2",
		},
		Taints:   map[string]string{},
		Sessions: make([]restapi.Session, 0),
	}
}

func defaultSessionRequirements(gpuVram uint64) restapi.SessionRequirements {
	return restapi.SessionRequirements{
		Version: "Test",
		Gpus: []restapi.GpuRequirements{
			{
				VramRequired: gpuVram,
			},
		},
		MatchLabels: map[string]string{},
		Tolerates:   map[string]string{},
	}
}

func createAgent() restapi.Agent {
	agent := restapi.Agent{
		State:    restapi.AgentActive,
		Hostname: "Test",
		Address:  "127.0.0.1:43210",
		Version:  "Test",
		Labels:   map[string]string{},
		Taints:   map[string]string{},
		Sessions: make([]restapi.Session, 0),
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
		Version:     "Test",
		MatchLabels: map[string]string{},
		Tolerates:   map[string]string{},
	}

	requirements.Gpus = make([]restapi.GpuRequirements, rand.Intn(7)+1)
	for index := range requirements.Gpus {
		requirements.Gpus[index] = restapi.GpuRequirements{
			VramRequired: uint64(rand.Intn(8192+1)) * 1024 * 1024,
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
		var checkStr, againstStr string

		bytes, err := json.Marshal(check)
		if err == nil {
			checkStr = string(bytes)
		} else {
			checkStr = err.Error()
		}

		bytes, err = json.Marshal(against)
		if err == nil {
			againstStr = string(bytes)
		} else {
			againstStr = err.Error()
		}

		t.Errorf("objects do not match\ncheck:\n%s\n====\nagainst:\n%s", checkStr, againstStr)
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
	run := func(t *testing.T, db storage.Storage) {
		agent := registerAgent(t, db, defaultAgent(24*1024*1024*1024))

		time.Sleep(time.Second)

		agent.State = restapi.AgentMissing
		db.SetAgentsMissingIfNotUpdatedFor(0)
		checkAgent(t, db, agent)

		agent.State = restapi.AgentActive
		db.UpdateAgent(restapi.AgentUpdate{
			Id: agent.Id,
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

	t.Run("memdb", func(t *testing.T) {
		db := openMemdb(t)
		defer db.Close()
		run(t, db)
	})

	t.Run("postgresql", func(t *testing.T) {
		db := openPostgres(t)
		defer db.Close()
		run(t, db)
	})
}

func TestSessions(t *testing.T) {
	run := func(t *testing.T, db storage.Storage) {
		requirements := createSessionRequirements()
		id := queueSession(t, db, requirements)

		queuedSession := storage.QueuedSession{
			Id:           id,
			Requirements: requirements,
		}

		checkQueuedSession(t, db, queuedSession)
	}

	t.Run("memdb", func(t *testing.T) {
		db := openMemdb(t)
		defer db.Close()
		run(t, db)
	})

	t.Run("postgresql", func(t *testing.T) {
		db := openPostgres(t)
		defer db.Close()
		run(t, db)
	})
}

func TestAssigningSessions(t *testing.T) {
	run := func(t *testing.T, db storage.Storage) {
		agent := registerAgent(t, db, defaultAgent(24*1024*1024*1024))

		requirements := defaultSessionRequirements(4 * 1024 * 1024 * 1024)
		sessionId := queueSession(t, db, requirements)

		selectedGpus := []restapi.SessionGpu{
			{
				Index:        agent.Gpus[0].Index,
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
		db.UpdateAgent(restapi.AgentUpdate{
			Id: agent.Id,
			Sessions: map[string]restapi.SessionUpdate{
				session.Id: {
					State: restapi.SessionActive,
				},
			},
		})
		checkAgent(t, db, agent)
		compare(t, session, agent.Sessions[0], nil)
		checkSession(t, db, session)

		agent.Sessions = make([]restapi.Session, 0)
		db.UpdateAgent(restapi.AgentUpdate{
			Id: agent.Id,
			Sessions: map[string]restapi.SessionUpdate{
				session.Id: {
					State: restapi.SessionClosed,
				},
			},
		})
		checkAgent(t, db, agent)

		session.State = restapi.SessionClosed
		checkSession(t, db, session)
	}

	t.Run("memdb", func(t *testing.T) {
		db := openMemdb(t)
		defer db.Close()
		run(t, db)
	})

	t.Run("postgresql", func(t *testing.T) {
		db := openPostgres(t)
		defer db.Close()
		run(t, db)
	})
}

func TestGetQueuedSessionsIterator(t *testing.T) {
	run := func(t *testing.T, db storage.Storage) {
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

	t.Run("memdb", func(t *testing.T) {
		db := openMemdb(t)
		defer db.Close()
		run(t, db)
	})

	t.Run("postgresql", func(t *testing.T) {
		db := openPostgres(t)
		defer db.Close()
		run(t, db)
	})
}
