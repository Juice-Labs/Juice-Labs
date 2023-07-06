/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/session"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
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

	api restapi.Client
}

func NewAgent(tlsConfig *tls.Config) (*Agent, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	server, err := server.NewServer(*address, tlsConfig)
	if err != nil {
		return nil, err
	}

	agent := &Agent{
		Id:          uuid.NewString(),
		JuicePath:   *juicePath,
		Server:      server,
		maxSessions: *maxSessions,
		sessions:    orderedmap.New[string, *session.Session](),
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

	agent.GpuMetricsProvider = cmdgpu.NewMetricsProvider(agent.Gpus, rendererWinPath)

	agent.initializeEndpoints()

	return agent, nil
}

func (agent *Agent) Run(group task.Group) error {
	group.Go("Agent GpuMetricsProvider", agent.GpuMetricsProvider)
	group.Go("Agent Server", agent.Server)
	return nil
}

func (agent *Agent) getSession(id string) (*session.Session, error) {
	session, found := agent.sessions.Get(id)
	if found {
		return session, nil
	}

	return nil, fmt.Errorf("no session found with id %s", id)
}

func (agent *Agent) startSession(group task.Group, sessionRequirements restapi.SessionRequirements) (string, error) {
	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return "", fmt.Errorf("Agent.startSession: unable to find a matching set of GPUs")
	}

	id := uuid.NewString()
	session := session.New(id, agent.JuicePath, sessionRequirements.Version, selectedGpus)

	group.GoFn("Agent runSession", func(group task.Group) error {
		err := session.Run(group)

		agent.sessionsMutex.Lock()
		defer agent.sessionsMutex.Unlock()

		agent.sessions.Delete(id)
		logger.Debugf("Removing Session %s", id)

		selectedGpus.Release()

		return err
	})

	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	agent.sessions.Set(id, session)
	logger.Debugf("Starting Session %s", id)

	return id, nil
}
