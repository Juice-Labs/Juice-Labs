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
	"sync/atomic"

	"github.com/google/uuid"
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/connection"
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

type Session struct {
	mutex sync.Mutex

	id        string
	juicePath string
	version   string
	gpus      *gpu.SelectedGpuSet

	state       string
	connections *orderedmap.OrderedMap[string, *Reference[connection.Connection]]
}

func (session *Session) Id() string {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	return session.id
}

func NewSession(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet) *Session {
	return &Session{
		id:          id,
		juicePath:   juicePath,
		version:     version,
		state:       restapi.SessionActive,
		gpus:        gpus,
		connections: orderedmap.New[string, *Reference[connection.Connection]](),
	}
}

func (session *Session) Session() restapi.Session {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	connections := make([]restapi.Connection, session.connections.Len())
	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connections = append(connections, pair.Value.Object.Connection())
	}

	return restapi.Session{
		Id:          session.id,
		State:       session.state,
		Version:     session.version,
		Gpus:        session.gpus.GetGpus(),
		Connections: connections,
	}
}

func (session *Session) Close() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	errs := []error{}
	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connection := pair.Value.Object
		errs = append(errs, connection.Close())
		pair.Value.Release()
	}

	session.connections = nil

	session.gpus.Release()
	session.gpus = nil

	err := errors.Join(errs...)
	return err
}

func (session *Session) Cancel() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.state = restapi.SessionCanceling
	errs := []error{}

	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connection := pair.Value.Object
		errs = append(errs, connection.Cancel())
	}

	return errors.Join(errs...)
}

func (session *Session) AddConnection(connection *connection.Connection) *Reference[connection.Connection] {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	connectionRef := NewReference(connection, func() {
		err := connection.Close()
		if err != nil {
			logger.Errorf("session %s experienced a failure during closing, %v", session.Id(), err)
		}

		session.mutex.Lock()
		defer session.mutex.Unlock()

		session.connections.Delete(connection.Id())

		// TODO: if session is not persistant, close it here if there are no more connections
	})

	session.connections.Set(connection.Id(), connectionRef)

	return connectionRef
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
		// We delete this reference inside cancelSession

		// TODO: Close the session when the last connection if the session is not persistant

		err := session.Close()
		if err != nil {
			logger.Errorf("session %s experienced a failure during closing, %v", session.Id(), err)
		}

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

func (agent *Agent) setSession(group task.Group, id string, juicePath string, version string, gpus *gpu.SelectedGpuSet) error {
	agent.addSession(NewSession(id, juicePath, version, gpus))

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
	agent.sessionsMutex.Lock()
	defer agent.sessionsMutex.Unlock()
	// Release underlying reference
	reference, found := agent.sessions.Get(id)
	if found {
		reference.Release()
	}
	return nil
}

func (agent *Agent) startConnection(group task.Group, sessionId string, c net.Conn) error {

	sessionRef, err := agent.getSession(sessionId)
	if err != nil {
		return err
	}

	id := uuid.NewString()
	connectionRef := sessionRef.Object.AddConnection(connection.New(id, sessionRef.Object.juicePath, sessionRef.Object.version, sessionRef.Object.gpus, agent))
	if err == nil {
		group.GoFn("Agent startConnection", func(group task.Group) error {
			err := connectionRef.Object.Wait()
			connectionRef.Release()
			return err
		})
	} else {
		connectionRef.Release()
	}

	connectionRef.Object.Connect(c)

	return err
}

func (agent *Agent) requestSession(group task.Group, sessionRequirements restapi.SessionRequirements) (string, error) {
	selectedGpus, err := agent.Gpus.Find(sessionRequirements.Gpus)
	if err != nil {
		return "", fmt.Errorf("Agent.startSession: unable to find a matching set of GPUs")
	}

	id := uuid.NewString()
	return id, agent.setSession(group, id, agent.JuicePath, sessionRequirements.Version, selectedGpus)
}

func (agent *Agent) registerSession(group task.Group, apiSession restapi.Session) error {
	selectedGpus, err := agent.Gpus.Select(apiSession.Gpus)
	if err != nil {
		return fmt.Errorf("Agent.registerSession: unable to select a matching set of GPUs")
	}

	return agent.setSession(group, apiSession.Id, agent.JuicePath, apiSession.Version, selectedGpus)
}

func (agent *Agent) ConnectionTerminated(id string, exitStatus string) {
	if agent.connectionUpdates != nil {
		logger.Tracef("connection %s changed state to %s", id, exitStatus)
		agent.connectionUpdates <- connectionUpdate{
			Id:          id,
			ExistStatus: exitStatus,
		}
	}
}

func (agent *Agent) getGpuMetrics() []restapi.GpuMetrics {
	agent.gpuMetricsMutex.Lock()
	defer agent.gpuMetricsMutex.Unlock()

	// Make a copy
	return append(make([]restapi.GpuMetrics, 0, len(agent.gpuMetrics)), agent.gpuMetrics...)
}
