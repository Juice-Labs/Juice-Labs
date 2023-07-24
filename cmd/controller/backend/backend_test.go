/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package backend

import (
	"context"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"

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
		Labels: map[string]string{},
		Taints: map[string]string{},
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

func TestGetAvailableAgentsMatching(t *testing.T) {
	run := func(t *testing.T, db storage.Storage) {
		backend := NewBackend(db)

		agentIds := []string{
			registerAgent(t, db, defaultAgent(24*1024*1024*1024)).Id,
			registerAgent(t, db, defaultAgent(4*1024*1024*1024)).Id,
			registerAgent(t, db, defaultAgent(4*1024*1024*1024)).Id,
		}

		sessionIds := []string{
			queueSession(t, db, defaultSessionRequirements(8*1024*1024*1024)),
			queueSession(t, db, defaultSessionRequirements(4*1024*1024*1024)),
			queueSession(t, db, defaultSessionRequirements(2*1024*1024*1024)),
			queueSession(t, db, defaultSessionRequirements(4*1024*1024*1024)),
		}

		err := backend.update(context.Background())
		if err != nil {
			t.Error(err)
		}

		for _, id := range sessionIds {
			session, err := db.GetSessionById(id)
			if err != nil {
				t.Error(err)
			} else if session.State != restapi.SessionAssigned {
				t.Errorf("expected session to be assigned, state = %s", session.State)
			}
		}

		for _, id := range agentIds {
			agent, err := db.GetAgentById(id)
			if err != nil {
				t.Error(err)
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
