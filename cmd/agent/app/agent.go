/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/prometheus"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/session"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

var (
	juicePath = flag.String("juice-path", "", "")

	address     = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
	maxSessions = flag.Int("max-sessions", 4, "Maximum number of simultaneous sessions allowed on this Agent")
)

type Agent struct {
	Id string

	Hostname  string
	JuicePath string

	Gpus *gpu.GpuSet

	Server *server.Server

	GpuMetricsProvider *cmdgpu.MetricsProvider

	maxSessions int

	sessionsMutex sync.Mutex
	sessions      *orderedmap.OrderedMap[string, *session.Session]

	taskManager *task.TaskManager

	httpClient *http.Client
}

func NewAgent(ctx context.Context, tlsConfig *tls.Config) (*Agent, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	agent := &Agent{
		Id:          uuid.NewString(),
		JuicePath:   *juicePath,
		Server:      server.NewServer(*address, tlsConfig),
		maxSessions: *maxSessions,
		sessions:    orderedmap.New[string, *session.Session](),
		taskManager: task.NewTaskManager(ctx),
	}

	if agent.JuicePath == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil, err
		}

		agent.JuicePath = filepath.Dir(executable)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	agent.Hostname = hostname

	rendererWinPath := filepath.Join(agent.JuicePath, "Renderer_Win")

	agent.Gpus, err = cmdgpu.DetectGpus(rendererWinPath)
	if err != nil {
		return nil, err
	}

	logger.Info("GPUs")
	for _, gpu := range agent.Gpus.GetGpus() {
		logger.Infof("  %d @ %s: %s %dMB", gpu.Index, gpu.PciBus, gpu.Name, gpu.Vram/(1024*1024))
	}

	agent.initializeEndpoints()
	agent.GpuMetricsProvider = cmdgpu.NewMetricsProvider(agent.Gpus, rendererWinPath)
	agent.GpuMetricsProvider.AddConsumer(prometheus.NewGpuMetricsConsumer())

	return agent, nil
}

func (agent *Agent) Ctx() context.Context {
	return agent.taskManager.Ctx()
}

func (agent *Agent) Cancel() {
	agent.taskManager.Cancel()
}

func (agent *Agent) Go(label string, task task.Task) {
	agent.taskManager.Go(label, task)
}

func (agent *Agent) GoFn(label string, task task.TaskFn) {
	agent.taskManager.GoFn(label, task)
}

func (agent *Agent) Start() {
	agent.Go("Agent GpuMetricsProvider", agent.GpuMetricsProvider)
	agent.Go("Agent Server", agent.Server)
}

func (agent *Agent) Wait() error {
	return agent.taskManager.Wait()
}

func (agent *Agent) getSession(id string) (*session.Session, error) {
	session, found := agent.sessions.Get(id)
	if found {
		return session, nil
	}

	return nil, fmt.Errorf("no session found with id %s", id)
}

func (agent *Agent) getSessions() []restapi.Session {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	sessions := make([]restapi.Session, 0)
	for pair := agent.sessions.Oldest(); pair != nil; pair = pair.Next() {
		sessions = append(sessions, utilities.Require[*session.Session](pair.Value).Session)
	}

	return sessions
}

func (agent *Agent) startSession(sessionRequirements restapi.SessionRequirements) (*session.Session, error) {
	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return nil, err
	}

	session := session.New(uuid.NewString(), agent.JuicePath, sessionRequirements.Version, selectedGpus)

	err = session.Start(agent.taskManager.Ctx())
	if err != nil {
		return nil, err
	}

	agent.GoFn("Agent runSession", func(group task.Group) error {
		err := session.Run(group)

		agent.sessionsMutex.Lock()
		defer agent.sessionsMutex.Unlock()

		agent.sessions.Delete(session.Id)

		return err
	})

	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	agent.sessions.Set(session.Id, session)

	return session, nil
}
