/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/frontend"
	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage/gorm"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

func TestAgents(t *testing.T) {
	logger.Configure()

	group := task.NewTaskManager(context.Background())

	server, err := server.NewServer("0.0.0.0:8080", nil)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	storage, err := gorm.OpenStorage(group.Ctx(), "sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	frontend, err := frontend.NewFrontend(server, storage)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	group.Go("Server", server)
	group.Go("Frontend", frontend)

	api := restapi.Client{
		Client:  &http.Client{},
		Address: "localhost:8080",
	}

	id, err := api.RegisterAgentWithContext(group.Ctx(), restapi.Agent{
		State:    restapi.AgentActive,
		Hostname: "test",
		Address:  "test",
		Version:  "test",
		Gpus: []restapi.Gpu{
			{
				Index: 0,
				Name:  "test",
				Vram:  1000,
			},
		},
	})
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	group.GoFn("Agent Websocket", func(g task.Group) error {
		return api.ConnectAgentWithWebsocket(group.Ctx(), id, createProcessMessages(t))
	})

	ticker := time.NewTimer(3 * time.Second)
	<-ticker.C
	ticker.Stop()

	session, err := api.RequestSessionWithContext(group.Ctx(), restapi.SessionRequirements{})

	ticker = time.NewTimer(3 * time.Second)
	<-ticker.C
	ticker.Stop()

	msg, err := api.ConnectWithContext(group.Ctx(), session)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if msg != "Test Message" {
		t.Error("Response is invalid")
		t.FailNow()
	}

	group.Cancel()
	group.Wait()
}

func createProcessMessages(t *testing.T) restapi.MessageHandler {
	return func(message []byte) (*restapi.MessageResponse, error) {
		t.Log(string(message))
		return &restapi.MessageResponse{
			Topic:   "response",
			Message: json.RawMessage(message),
		}, nil
	}
}
