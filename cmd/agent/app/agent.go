/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/cmd/agent/session"
	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	juicePath = flag.String("juice-path", "", "")

	address     = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
	maxSessions = flag.Int("max-sessions", 0, "Maximum number of simultaneous sessions allowed on this Agent")
	tags        = flag.String("tags", "", "Comma separated list of key=value pairs")
	taints      = flag.String("taints", "", "Comma separated list of key=value pairs")
)

type Reference[T any] struct {
	Object      *T
	count       atomic.Int32
	onCountZero func()
}

func NewReference[T any](object *T, onCountZero func()) *Reference[T] {
	reference := &Reference[T]{
		Object:      object,
		count:       atomic.Int32{},
		onCountZero: onCountZero,
	}

	reference.count.Store(1)
	return reference
}

func (reference *Reference[T]) Acquire() bool {
	return reference.count.Add(1) > 1
}

func (reference *Reference[T]) Release() {
	if reference.count.Add(-1) == 0 {
		reference.onCountZero()
		reference.Object = nil
	}
}

type Agent struct {
	Id string

	Hostname  string
	JuicePath string

	Gpus *gpu.GpuSet

	Server *server.Server

	GpuMetricsProvider *cmdgpu.MetricsProvider

	maxSessions int

	tags   map[string]string
	taints map[string]string

	sessionsMutex sync.Mutex
	sessions      *orderedmap.OrderedMap[string, *Reference[session.Session]]

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
		tags:        map[string]string{},
		taints:      map[string]string{},
		sessions:    orderedmap.New[string, *Reference[session.Session]](),
	}

	if *tags != "" {
		var err error
		for _, tag := range strings.Split(*tags, ",") {
			keyValue := strings.Split(tag, "=")
			if len(keyValue) != 2 {
				err = errors.Join(err, fmt.Errorf("tag '%s' must be in the format key=value", tag))
			} else {
				agent.tags[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
		}

		if err != nil {
			return nil, fmt.Errorf("Agent.NewAgent: failed to parse --tags with %s", err)
		}
	}

	if *taints != "" {
		var err error
		for _, taint := range strings.Split(*taints, ",") {
			keyValue := strings.Split(taint, "=")
			if len(keyValue) != 2 {
				err = errors.Join(err, fmt.Errorf("taint '%s' must be in the format key=value", taint))
			} else {
				agent.taints[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
		}

		if err != nil {
			return nil, fmt.Errorf("Agent.NewAgent: failed to parse --taints with %s", err)
		}
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

func (agent *Agent) getState() restapi.Agent {
	return restapi.Agent{
		Id:          agent.Id,
		State:       restapi.AgentActive,
		Hostname:    agent.Hostname,
		Address:     agent.Server.Address(),
		Version:     build.Version,
		MaxSessions: agent.maxSessions,
		Gpus:        agent.Gpus.GetGpus(),
		Tags:        agent.tags,
		Taints:      agent.taints,
		Sessions:    agent.getAllSessions(),
	}
}

func (agent *Agent) getSessionsCount() int {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	return agent.sessions.Len()
}

func (agent *Agent) getAllSessions() []restapi.Session {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	sessions := make([]restapi.Session, 0)

	return sessions
}

func (agent *Agent) getSession(id string) (*Reference[session.Session], error) {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	session, found := agent.sessions.Get(id)
	if found {
		// If Acquire returns false, it is in the middle of being cleaned up
		if session.Acquire() {
			return session, nil
		}
	}

	return nil, fmt.Errorf("no session found with id %s", id)
}

func (agent *Agent) addSession(session *session.Session) *Reference[session.Session] {
	logger.Tracef("Starting Session %s", session.Id())

	reference := NewReference(session, func() {
		agent.sessionsMutex.Lock()
		defer agent.sessionsMutex.Unlock()

		agent.sessions.Delete(session.Id())
	})

	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	agent.sessions.Set(session.Id(), reference)

	return reference
}

func (agent *Agent) Run(group task.Group) error {
	group.Go("Agent GpuMetricsProvider", agent.GpuMetricsProvider)
	group.Go("Agent Server", agent.Server)
	return nil
}

func (agent *Agent) runSession(group task.Group, id string, juicePath string, version string, gpus *gpu.SelectedGpuSet) {
	reference := agent.addSession(session.New(id, juicePath, version, gpus))

	group.GoFn("Agent runSession", func(group task.Group) error {
		err := reference.Object.Run(group)
		reference.Release()
		return err
	})
}

func (agent *Agent) requestSession(group task.Group, sessionRequirements restapi.SessionRequirements) (string, error) {
	if agent.maxSessions > 0 && agent.getSessionsCount()+1 > agent.maxSessions {
		return "", fmt.Errorf("Agent.startSession: unable to add another session")
	}

	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return "", fmt.Errorf("Agent.startSession: unable to find a matching set of GPUs")
	}

	id := uuid.NewString()
	agent.runSession(group, id, agent.JuicePath, sessionRequirements.Version, selectedGpus)

	return id, nil
}

func (agent *Agent) registerSession(group task.Group, apiSession restapi.Session) error {
	selectedGpus, err := agent.Gpus.Select(apiSession.Gpus)
	if err != nil {
		return fmt.Errorf("Agent.registerSession: unable to select a matching set of GPUs")
	}

	agent.runSession(group, apiSession.Id, agent.JuicePath, apiSession.Version, selectedGpus)
	return nil
}
