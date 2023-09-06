/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"flag"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	juicePath = flag.String("juice-path", "", "")

	address = flag.String("address", "0.0.0.0:43210", "The IP address and port to use for listening for client connections")
	labels  = flag.String("labels", "", "Comma separated list of key=value pairs")
	taints  = flag.String("taints", "", "Comma separated list of key=value pairs")
)

type EventListener interface {
	SessionStateChanged(id string, state string)
	ConnectionChanged(sessionId string, connection restapi.Connection)
}

type Agent struct {
	Id string

	Hostname  string
	JuicePath string

	Gpus               *gpu.GpuSet
	GpuMetricsProvider *cmdgpu.MetricsProvider

	Server *server.Server

	labels map[string]string
	taints map[string]string

	sessionsMutex sync.Mutex
	sessions      *orderedmap.OrderedMap[string, *Reference[Session]]

	controllerData
}

func NewAgent(tlsConfig *tls.Config) (*Agent, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	server, err := server.NewServer(*address, tlsConfig)
	if err != nil {
		return nil, errors.New("failed to create server").Wrap(err)
	}

	agent := &Agent{
		Id:        uuid.NewString(),
		JuicePath: *juicePath,
		Server:    server,
		labels:    map[string]string{},
		taints:    map[string]string{},
		sessions:  orderedmap.New[string, *Reference[Session]](),
	}

	if *labels != "" {
		var err error
		for _, tag := range strings.Split(*labels, ",") {
			keyValue := strings.Split(tag, "=")
			if len(keyValue) != 2 {
				err = errors.Join(err, errors.Newf("tag '%s' must be in the format key=value", tag))
			} else {
				agent.labels[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
		}

		if err != nil {
			return nil, errors.New("failed to parse --labels").Wrap(err)
		}
	}

	if *taints != "" {
		var err error
		for _, taint := range strings.Split(*taints, ",") {
			keyValue := strings.Split(taint, "=")
			if len(keyValue) != 2 {
				err = errors.Join(err, errors.Newf("taint '%s' must be in the format key=value", taint))
			} else {
				agent.taints[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
		}

		if err != nil {
			return nil, errors.New("failed to parse --taints").Wrap(err)
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
		return nil, errors.New("failed to retrieve system hostname").Wrap(err)
	}

	agent.Hostname = hostname

	rendererWinPath := filepath.Join(agent.JuicePath, "Renderer_Win")

	agent.Gpus, err = cmdgpu.DetectGpus(rendererWinPath)
	if err != nil {
		return nil, errors.New("failed to detect GPUs").Wrap(err)
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
	logger.Infof("Starting agent on %s", *address)

	group.Go("Agent GpuMetricsProvider", agent.GpuMetricsProvider)
	group.Go("Agent Server", agent.Server)
	return nil
}

func (agent *Agent) getSession(id string) (*Reference[Session], error) {
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()

	reference, found := agent.sessions.Get(id)
	if found {
		// If Acquire returns false, it is in the middle of being cleaned up
		if reference.Acquire() {
			return reference, nil
		}
	}

	return nil, errors.Newf("no session found with id %s", id)
}

func (agent *Agent) addSession(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, persistent bool) error {
	logger.Tracef("Starting Session %s", id)

	session := newSession(id, juicePath, version, gpus, agent, persistent)

	reference := NewReference(session, func() {
		// We decrement this reference inside deleteSession
		logger.Tracef("closing Session %s", session.id)

		session.Close()

		agent.sessionsMutex.Lock()
		agent.sessions.Delete(session.id)
		agent.sessionsMutex.Unlock()

		logger.Tracef("closed Session %s", session.id)
	})

	agent.sessionsMutex.Lock()
	agent.sessions.Set(session.Id(), reference)
	agent.sessionsMutex.Unlock()

	agent.SessionStateChanged(session.Id(), session.State())

	return nil
}

func (agent *Agent) cancelSession(id string) error {
	sessionRef, err := agent.getSession(id)
	if err == nil {
		err = sessionRef.Object.Cancel()
		sessionRef.Release()
	}
	return err
}

func (agent *Agent) deleteSession(id string) {
	agent.sessionsMutex.Lock()
	reference, found := agent.sessions.Get(id)
	agent.sessionsMutex.Unlock()

	// Release underlying reference
	if found {
		reference.Release()
	}
}

func (agent *Agent) connect(group task.Group, connectionData restapi.ConnectionData, sessionId string, c net.Conn) error {
	sessionRef, err := agent.getSession(sessionId)
	if err == nil {
		err = sessionRef.Object.Connect(group, connectionData, c, agent)
		sessionRef.Release()
	}
	return err
}

func (agent *Agent) requestSession(group task.Group, sessionRequirements restapi.SessionRequirements) (string, error) {
	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return "", errors.New("unable to find a matching set of GPUs").Wrap(err)
	}

	id := uuid.NewString()
	return id, agent.addSession(id, agent.JuicePath, sessionRequirements.Version, selectedGpus, sessionRequirements.Persistent)
}

func (agent *Agent) registerSession(group task.Group, apiSession restapi.Session) error {
	selectedGpus, err := agent.Gpus.Select(apiSession.Gpus)
	if err != nil {
		return errors.New("unable to select a matching set of GPUs").Wrap(err)
	}

	return agent.addSession(apiSession.Id, agent.JuicePath, apiSession.Version, selectedGpus, apiSession.Persistent)
}

func (agent *Agent) getGpuMetrics() []restapi.GpuMetrics {
	agent.gpuMetricsMutex.Lock()
	defer agent.gpuMetricsMutex.Unlock()

	// Make a copy
	return append(make([]restapi.GpuMetrics, 0, len(agent.gpuMetrics)), agent.gpuMetrics...)
}

func (agent *Agent) SessionStateChanged(id string, state string) {
	logger.Tracef("session %s changed state to %s", id, state)

	if state == restapi.SessionClosed {
		agent.deleteSession(id)
	}

	if agent.sessionUpdates != nil {
		agent.sessionUpdates <- sessionUpdate{
			Id:    id,
			State: state,
		}
	}
}

func (agent *Agent) ConnectionChanged(sessionId string, connection restapi.Connection) {
	logger.Tracef("session %s closed connection %s", sessionId, connection.Id)

	if agent.connectionUpdates != nil {
		agent.connectionUpdates <- connectionUpdate{
			SessionId:  sessionId,
			Connection: connection,
		}
	}
}
