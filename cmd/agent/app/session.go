package app

import (
	"errors"
	"net"
	"strconv"
	"sync"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/connection"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type EventListener interface {
	SessionStateChanged(sessionId string, state string)
}

type Session struct {
	mutex sync.RWMutex

	id         string
	juicePath  string
	version    string
	persistant bool
	gpus       *gpu.SelectedGpuSet

	state       string
	connections *orderedmap.OrderedMap[string, *Reference[connection.Connection]]

	eventListener EventListener
}

func (session *Session) Id() string {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	return session.id
}

func (session *Session) Persistent() bool {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	return session.persistant
}

func (session *Session) State() string {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	return session.state
}

func (session *Session) ActiveConnections() *orderedmap.OrderedMap[string, *Reference[connection.Connection]] {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	// activeConnections := orderedmap.New[string, *Reference[connection.Connection]]()

	// for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
	// 	connection := pair.Value.Object
	// 	if connection.ExitStatus() == restapi.ExitStatusUnknown {
	// 		activeConnections.Set(pair.Key, pair.Value)
	// 	}
	// }

	// return activeConnections

	return session.connections
}

func NewSession(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, eventListener EventListener, persistent bool) *Session {
	session := Session{
		id:            id,
		juicePath:     juicePath,
		version:       version,
		state:         restapi.SessionActive,
		gpus:          gpus,
		persistant:    persistent,
		connections:   orderedmap.New[string, *Reference[connection.Connection]](),
		eventListener: eventListener,
	}

	return &session
}

func (session *Session) Session() restapi.Session {
	session.mutex.RLock()
	defer session.mutex.RUnlock()

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

func (session *Session) Close() error {
	errs := []error{}
	// verify that there are no connections
	if session.ActiveConnections().Len() > 0 {
		logger.Errorf("closing session has active connections, %s", session.Id())
		errs = append(errs, errors.New("closing session has active connections"))
	}
	session.mutex.Lock()

	session.state = restapi.SessionClosed

	session.connections = nil

	session.gpus.Release()
	session.gpus = nil

	session.mutex.Unlock()
	session.eventListener.SessionStateChanged(session.id, session.state)

	err := errors.Join(errs...)
	return err
}

func (session *Session) Cancel() error {
	session.mutex.Lock()
	session.state = restapi.SessionCanceling
	session.mutex.Unlock()

	session.mutex.RLock()
	defer session.mutex.RUnlock()

	errs := []error{}

	for pair := session.connections.Oldest(); pair != nil; pair = pair.Next() {
		connection := pair.Value.Object
		errs = append(errs, connection.Cancel())
	}
	session.eventListener.SessionStateChanged(session.id, session.state)

	return errors.Join(errs...)
}

func (session *Session) AddConnection(connection *connection.Connection) *Reference[connection.Connection] {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	connectionRef := NewReference(connection, func() {
		logger.Tracef("session %s closing connection %s", session.Id(), connection.Id())
		err := connection.Close()
		if err != nil {
			logger.Errorf("session %s experienced a failure during closing, %v", session.Id(), err)
		}

		session.mutex.Lock()
		defer session.mutex.Unlock()

		session.connections.Delete(connection.Id())
	})

	session.connections.Set(connection.Id(), connectionRef)

	return connectionRef
}

func (session *Session) Connect(group task.Group, connectionData restapi.ConnectionData, c net.Conn, agent *Agent) error {
	connectionRef, found := session.GetConnection(connectionData.Id)
	if found {
		logger.Tracef("Connecting to existing connection: %s", connectionData.Id)
		defer connectionRef.Release()
	} else {
		// New Connection - Create it and start RenderWin
		pid, err := strconv.ParseInt(connectionData.Pid, 10, 64)
		if err != nil {
			pid = 0
		}
		connectionRef = session.AddConnection(connection.New(connectionData.Id, session.juicePath, session.version, session.gpus, session.id, pid, connectionData.ProcessName, agent))
		err = connectionRef.Object.Start(group)
		if err == nil {
			group.GoFn("Agent startConnection", func(group task.Group) error {
				err := connectionRef.Object.Wait()
				logger.Tracef("Connection %s exited with code: %v", connectionRef.Object.Id(), connectionRef.Object.ExitStatus())
				if err != nil {
					logger.Errorf("session %s experienced a failure during waiting, %v", session.Id(), err)
				}
				connectionRef.Release()

				if session.ActiveConnections().Len() == 0 {
					if !session.Persistent() {
						err = session.Cancel()
						if err != nil {
							logger.Errorf("session %s experienced a failure canceling, %v", session.Id(), err)
						}
					}

				}
				return err
			})
		} else {
			connectionRef.Release()
		}
		logger.Tracef("Connection %s created for pid: %s, process name: %s", connectionData.Id, connectionData.Pid, connectionData.ProcessName)
		session.eventListener.SessionStateChanged(session.id, session.state)
	}

	err := connectionRef.Object.Connect(c)

	return err
}

func (session *Session) GetConnection(id string) (*Reference[connection.Connection], bool) {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	connectionRef, ok := session.connections.Get(id)
	if !ok {
		return nil, false
	}
	if !connectionRef.Acquire() {
		return nil, false
	}
	return connectionRef, true
}
