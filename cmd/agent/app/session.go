/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"context"
	"fmt"
	"net"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Session struct {
	Id         string
	Version    string
	Persistent bool

	juicePath string
	gpus      *gpu.SelectedGpuSet

	connections *utilities.ConcurrentMap[string, *Connection]
	taskManager *task.TaskManager

	eventListener EventListener
}

func newSession(ctx context.Context, id string, version string, persistent bool, juicePath string, gpus *gpu.SelectedGpuSet, eventListener EventListener) *Session {
	return &Session{
		Id:            id,
		Version:       version,
		Persistent:    persistent,
		juicePath:     juicePath,
		gpus:          gpus,
		connections:   utilities.NewConcurrentMap[string, *Connection](),
		taskManager:   task.NewTaskManager(ctx),
		eventListener: eventListener,
	}
}

func (session *Session) Session() restapi.Session {
	connections := make([]restapi.Connection, 0, session.connections.Len())
	session.connections.Foreach(func(key string, value *Connection) bool {
		connections = append(connections, restapi.Connection{
			ConnectionData: value.ConnectionData,
		})
		return true
	})

	return restapi.Session{
		Id:          session.Id,
		State:       restapi.SessionActive,
		Version:     session.Version,
		Gpus:        session.gpus.GetGpus(),
		Connections: connections,
		Persistent:  session.Persistent,
	}
}

func (session *Session) Run(group task.Group) error {
	group.GoFn(fmt.Sprintf("session %s close", session.Id), func(g task.Group) error {
		<-session.taskManager.Ctx().Done()

		err := session.taskManager.Wait()

		session.gpus.Release()

		session.eventListener.SessionClosed(session.Id)

		return err
	})

	return nil
}

func (session *Session) Cancel() {
	session.taskManager.Cancel()
}

func (session *Session) addConnection(connectionData restapi.ConnectionData) (*Connection, error) {
	logger.Tracef("session %s creating connection %s for PID %s and process name %s", session.Id, connectionData.Id, connectionData.Pid, connectionData.ProcessName)

	exitCodeCh := make(chan int)

	connection := newConnection(connectionData, session.juicePath, session.gpus.GetPciBusString())
	err := connection.Start(session.taskManager, exitCodeCh)
	if err != nil {
		return nil, err
	}

	session.connections.Set(connection.Id, connection)

	session.taskManager.GoFn(fmt.Sprintf("session %s connection %s", session.Id, connection.Id), func(g task.Group) error {
		exitCode, ok := <-exitCodeCh
		if !ok {
			panic("channel has been closed")
		}
		close(exitCodeCh)

		session.connections.Delete(connection.Id)
		if !session.Persistent && session.connections.Empty() {
			session.Cancel()
		}

		session.eventListener.ConnectionClosed(session.Id, connection.ConnectionData, exitCode)

		return nil
	})

	session.eventListener.ConnectionCreated(session.Id, connection.ConnectionData)

	return connection, nil
}

func (session *Session) Connect(connectionData restapi.ConnectionData, c net.Conn) error {
	logger.Tracef("Connecting to connection: %s", connectionData.Id)

	connection, found := session.connections.Get(connectionData.Id)
	if !found {
		var err error
		connection, err = session.addConnection(connectionData)
		if err != nil {
			return errors.Newf("session %s connection %s failed to connect", session.Id, connectionData.Id).Wrap(err)
		}
	}

	return connection.Connect(c)
}
