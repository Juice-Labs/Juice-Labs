/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/cmd/agent/gpu"
	gpuMetrics "github.com/Juice-Labs/cmd/agent/gpu/metrics"
	"github.com/Juice-Labs/cmd/agent/prometheus"
	"github.com/Juice-Labs/cmd/agent/session"
	"github.com/Juice-Labs/internal/build"
	"github.com/Juice-Labs/pkg/api"
	"github.com/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/pkg/utilities"
)

var (
	printVersion = flag.Bool("version", false, "Prints the version and exits")
	juicePath    = flag.String("juice-path", "", "")

	address     = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
	maxSessions = flag.Int("max-sessions", 4, "Maximum number of simultaneous sessions allowed on this Agent")
)

type Agent struct {
	Id string

	Hostname  string
	JuicePath string

	Gpus gpu.GpuSet

	Server *server.Server

	GpuMetricsProvider *gpuMetrics.Provider

	maxSessions int

	sessionsMutex sync.Mutex
	sessions      *orderedmap.OrderedMap[string, *session.Session]

	taskManager *task.TaskManager
}

func NewAgent(tlsConfig *tls.Config) (*Agent, error) {
	flag.Parse()

	if *printVersion {
		fmt.Fprintln(os.Stdout, build.Version)
		return nil, nil
	}

	err := logger.Configure()
	if err != nil {
		return nil, err
	}

	logger.Info("Juice Agent, v", build.Version)

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	agent := &Agent{
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

	agent.Hostname, err = os.Hostname()
	if err != nil {
		return nil, err
	}

	rendererWinPath := filepath.Join(agent.JuicePath, "Renderer_Win")

	agent.Gpus, err = cmdgpu.DetectGpus(rendererWinPath)
	if err != nil {
		return nil, err
	} else if len(agent.Gpus) == 0 {
		return nil, errors.New("no supported gpus detected")
	}

	logger.Info("GPUs")
	for _, gpu := range agent.Gpus {
		logger.Infof("  %d @ %s: %s %dMB", gpu.Index, gpu.PciBus, gpu.Name, gpu.Vram/(1024*1024))
	}

	agent.initializeEndpoints()
	agent.GpuMetricsProvider = gpuMetrics.NewProvider(agent.Gpus, rendererWinPath)
	agent.GpuMetricsProvider.AddConsumer(prometheus.NewConsumer())

	return agent, nil
}

func (agent *Agent) Sessions() []api.Session {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	sessions := make([]api.Session, 0)
	for pair := agent.sessions.Oldest(); pair != nil; pair = pair.Next() {
		sessions = append(sessions, utilities.Require[*session.Session](pair.Value).Session)
	}

	return sessions
}

func (agent *Agent) MaxSessions() int {
	return agent.maxSessions
}

func (agent *Agent) Ctx() context.Context {
	return agent.taskManager.Ctx()
}

func (agent *Agent) Cancel() {
	agent.taskManager.Cancel()
}

func (agent *Agent) Go(task task.Task) {
	agent.taskManager.Go(task)
}

func (agent *Agent) GoFn(task task.TaskFn) {
	agent.taskManager.GoFn(task)
}

func (agent *Agent) Start() {
	agent.Go(agent.Server)
}

func (agent *Agent) Wait() error {
	return agent.taskManager.Wait()
}

func (agent *Agent) GetSession(id string) (*session.Session, error) {
	session, found := agent.sessions.Get(id)
	if found {
		return session, nil
	}

	return nil, fmt.Errorf("no session found with id %s", id)
}

func (agent *Agent) StartSession(requestSession api.RequestSession) (*session.Session, error) {
	selectedGpus, err := agent.Gpus.Find(requestSession.Gpus)
	if err != nil {
		return nil, err
	}

	return agent.runSession(session.New(uuid.NewString(), agent.JuicePath, requestSession.Version, selectedGpus))
}

func (agent *Agent) RegisterSession(sessionToRegister api.Session) error {
	selectedGpus, err := agent.Gpus.Select(sessionToRegister.Gpus)
	if err != nil {
		return err
	}

	_, err = agent.runSession(session.New(sessionToRegister.Id, agent.JuicePath, sessionToRegister.Version, selectedGpus))
	return err
}

func (agent *Agent) runSession(session *session.Session) (*session.Session, error) {
	agent.GoFn(func(group task.Group) error {
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
