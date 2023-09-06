/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"fmt"
	"net"
	"sync"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type Session struct {
	sync.Mutex

	id         string
	juicePath  string
	version    string
	persistant bool
	gpus       *gpu.SelectedGpuSet

	state       string
	connections *orderedmap.OrderedMap[string, *Reference[Connection]]

	eventListener EventListener
}

func (session *Session) Id() string {
	return session.id
}

func (session *Session) State() string {
	session.Lock()
	defer session.Unlock()

	return session.state
}

func (session *Session) HasActiveConnections() bool {
	session.Lock()
	defer session.Unlock()

	return session.connections.Len() > 0
}

func newSession(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, eventListener EventListener, persistent bool) *Session {
	return &Session{
		id:            id,
		juicePath:     juicePath,
		version:       version,
		state:         restapi.SessionActive,
		gpus:          gpus,
		persistant:    persistent,
		connections:   orderedmap.New[string, *Reference[Connection]](),
		eventListener: eventListener,
	}
}

func (session *Session) Session() restapi.Session {
	session.Lock()
	defer session.Unlock()

	connections := make([]restapi.Connection, 0, session.connections.Len())
	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connections = append(connections, pair.Value.Object.Connection())
	}

	return restapi.Session{
		Id:          session.id,
		State:       session.state,
		Version:     session.version,
		Gpus:        session.gpus.GetGpus(),
		Connections: connections,
		Persistent:  session.persistant,
	}
}

func (session *Session) Close() {
	if session.HasActiveConnections() {
		panic(fmt.Sprintf("closing session has active connections, %s", session.Id()))
	}

	session.Lock()

	session.state = restapi.SessionClosed

	session.gpus.Release()
	session.gpus = nil

	session.Unlock()

	session.eventListener.SessionStateChanged(session.Id(), session.State())
}

func (session *Session) Cancel() error {
	session.Lock()

	session.state = restapi.SessionCanceling

	var err error
	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connection := pair.Value.Object
		err = errors.Join(err, connection.Cancel())
	}

	session.Unlock()

	session.eventListener.SessionStateChanged(session.Id(), session.State())

	return err
}

func (session *Session) addConnection(group task.Group, connectionData restapi.ConnectionData) (*Reference[Connection], error) {
	logger.Tracef("session %s creating connection %s for PID %s and process name %s", session.Id(), connectionData.Id, connectionData.Pid, connectionData.ProcessName)

	connection := newConnection(connectionData, session.juicePath, session.gpus.GetPciBusString())

	reference := NewReference(connection, func() {
		connectionId := connection.Id()
		connectionInfo := connection.Connection()

		logger.Tracef("session %s closing connection %s", session.Id(), connectionId)

		err := connection.Close()
		if err != nil {
			logger.Errorf("session %s connection %s failed closing, %v", session.Id(), connectionId, err)
		}

		session.Lock()
		session.connections.Delete(connectionId)
		session.Unlock()

		session.eventListener.ConnectionChanged(session.Id(), connectionInfo)

		if !session.HasActiveConnections() && !session.persistant {
			session.Close()
		}
	})

	session.Lock()
	session.connections.Set(connection.Id(), reference)
	session.Unlock()

	session.eventListener.ConnectionChanged(session.Id(), connection.Connection())

	err := reference.Object.Start(group)
	if err == nil {
		group.GoFn("Connection wait", func(group task.Group) error {
			err := connection.Wait()
			if err != nil {
				err = errors.Newf("session %s connection %s failed waiting", session.Id(), connection.Id()).Wrap(err)
			}

			logger.Tracef("session %s connection %s exited with code %s", session.Id(), connection.Id(), connection.ExitStatus())

			reference.Release()

			return err
		})
	} else {
		reference.Release()

		return nil, errors.Newf("session %s connection %s failed starting", session.Id(), connection.Id()).Wrap(err)
	}

	return reference, nil
}

func (session *Session) Connect(group task.Group, connectionData restapi.ConnectionData, c net.Conn, agent *Agent) error {
	logger.Tracef("Connecting to connection: %s", connectionData.Id)

	session.Lock()
	reference, ok := session.connections.Get(connectionData.Id)
	session.Unlock()

	var err error
	if ok {
		defer reference.Release()
	} else {
		reference, err = session.addConnection(group, connectionData)
	}

	if err != nil {
		return err
	}

	return reference.Object.Connect(c)
}
