/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	cmdgpu "github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
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
		return nil, err
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
				err = errors.Join(err, fmt.Errorf("tag '%s' must be in the format key=value", tag))
			} else {
				agent.labels[strings.TrimSpace(keyValue[0])] = strings.TrimSpace(keyValue[1])
			}
		}

		if err != nil {
			return nil, fmt.Errorf("Agent.NewAgent: failed to parse --labels with %s", err)
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

	return nil, fmt.Errorf("no session found with id %s", id)
}

func (agent *Agent) addSession(session *Session) *Reference[Session] {
	logger.Tracef("Starting Session %s", session.Id())

	reference := NewReference(session, func() {
		// We decrement this reference inside deleteSession
		sessionId := session.Id()
		logger.Tracef("Closing Session %s", sessionId)
		err := session.Close()
		if err != nil {
			logger.Errorf("session %s experienced a failure during closing, %v", sessionId, err)
		}

		agent.sessionsMutex.Lock()
		defer agent.sessionsMutex.Unlock()

		agent.sessions.Delete(sessionId)
		logger.Tracef("Closed Session %s", sessionId)
	})

	agent.sessionsMutex.Lock()
	agent.sessions.Set(session.Id(), reference)
	agent.sessionsMutex.Unlock()

	agent.SessionStateChanged(session.id, session.state)

	return reference
}

func (agent *Agent) Run(group task.Group) error {
	group.Go("Agent GpuMetricsProvider", agent.GpuMetricsProvider)
	group.Go("Agent Server", agent.Server)
	return nil
}

func (agent *Agent) setSession(group task.Group, id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, persistent bool) error {
	agent.addSession(NewSession(id, juicePath, version, gpus, agent, persistent))
	return nil
}

func (agent *Agent) cancelSession(id string) error {
	sessionRef, err := agent.getSession(id)
	if err != nil {
		return err
	}

	err = sessionRef.Object.Cancel()
	sessionRef.Release()
	if err != nil {
		return err
	}
	return nil
}

func (agent *Agent) deleteSession(id string) {
	agent.sessionsMutex.Lock()
	// Release underlying reference
	reference, found := agent.sessions.Get(id)
	agent.sessionsMutex.Unlock()
	if found {
		reference.Release()
	}
}

func (agent *Agent) connect(group task.Group, connectionData restapi.ConnectionData, sessionId string, c net.Conn) error {
	sessionRef, err := agent.getSession(sessionId)
	if err != nil {
		return err
	}
	defer sessionRef.Release()

	return sessionRef.Object.Connect(group, connectionData, c, agent)
}

func (agent *Agent) requestSession(group task.Group, sessionRequirements restapi.SessionRequirements) (string, error) {
	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return "", fmt.Errorf("Agent.startSession: unable to find a matching set of GPUs")
	}

	id := uuid.NewString()
	return id, agent.setSession(group, id, agent.JuicePath, sessionRequirements.Version, selectedGpus, sessionRequirements.Persistent)
}

func (agent *Agent) registerSession(group task.Group, apiSession restapi.Session) error {
	selectedGpus, err := agent.Gpus.Select(apiSession.Gpus)
	if err != nil {
		return fmt.Errorf("Agent.registerSession: unable to select a matching set of GPUs")
	}

	return agent.setSession(group, apiSession.Id, agent.JuicePath, apiSession.Version, selectedGpus, apiSession.Persistent)
}

func (agent *Agent) ConnectionTerminated(id string, sessionId string, exitStatus string) {
	logger.Tracef("connection %s changed exitStatus to %s", id, exitStatus)
	sessionRef, err := agent.getSession(sessionId)
	if err != nil {
		logger.Errorf("session not found %s with error %s", sessionId, err)
		return
	}
	defer sessionRef.Release()

	session := sessionRef.Object

	if session.ActiveConnections().Len() == 0 {
		if session.State() == restapi.SessionCanceling {
			agent.deleteSession(session.Id())
		}
	}

	agent.NotifySessionUpdates(sessionId)
}

func (agent *Agent) SessionStateChanged(sessionId string, state string) {
	logger.Tracef("session %s changed state to %s", sessionId, state)
	if state == restapi.SessionCanceling {
		sessionRef, err := agent.getSession(sessionId)
		if err != nil {
			logger.Errorf("session not found %s with error %s", sessionId, err)
			return
		}
		defer sessionRef.Release()
		// If session has no connections, go ahead and close it
		if sessionRef.Object.ActiveConnections().Len() == 0 {
			agent.deleteSession(sessionId)
		}
	}

	if state == restapi.SessionClosed {
		// Closed sessions can't be accessed anymore
		agent.NotifySessionClosed(sessionId)
	} else {
		agent.NotifySessionUpdates(sessionId)
	}
}

func (agent *Agent) NotifySessionUpdates(sessionId string) {
	sessionRef, err := agent.getSession(sessionId)
	if err != nil {
		logger.Errorf("session not found %s with error %s", sessionId, err)
		return
	}
	defer sessionRef.Release()

	if agent.sessionUpdates != nil {
		connectionUpdates := make([]connectionUpdate, 0, sessionRef.Object.connections.Len())
		for pair := sessionRef.Object.connections.Oldest(); pair != nil; pair = pair.Next() {
			connection := pair.Value.Object
			connectionUpdates = append(connectionUpdates, connectionUpdate{
				Id:          connection.Id(),
				ExitStatus:  connection.ExitStatus(),
				Pid:         connection.Pid(),
				ProcessName: connection.ProcessName(),
			})
		}
		agent.sessionUpdates <- sessionUpdate{
			Id:          sessionId,
			State:       sessionRef.Object.state,
			Connections: connectionUpdates,
		}
	}
}

func (agent *Agent) NotifySessionClosed(sessionId string) {
	if agent.sessionUpdates != nil {
		agent.sessionUpdates <- sessionUpdate{
			Id:    sessionId,
			State: restapi.SessionClosed,
		}
	}
}

func (agent *Agent) getGpuMetrics() []restapi.GpuMetrics {
	agent.gpuMetricsMutex.Lock()
	defer agent.gpuMetricsMutex.Unlock()

	// Make a copy
	return append(make([]restapi.GpuMetrics, 0, len(agent.gpuMetrics)), agent.gpuMetrics...)
}
