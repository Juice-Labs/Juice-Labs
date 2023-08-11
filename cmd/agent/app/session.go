package app

import (
	"errors"
	"sync"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/connection"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type EventListener interface {
	SessionStateChanged(sessionId string, state string)
}

type Session struct {
	mutex sync.Mutex

	id        string
	juicePath string
	version   string
	gpus      *gpu.SelectedGpuSet

	state       string
	connections *orderedmap.OrderedMap[string, *Reference[connection.Connection]]

	eventListener EventListener
}

func (session *Session) Id() string {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	return session.id
}

func NewSession(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, eventListener EventListener) *Session {
	session := Session{
		id:          id,
		juicePath:   juicePath,
		version:     version,
		state:       restapi.SessionActive,
		gpus:        gpus,
		connections: orderedmap.New[string, *Reference[connection.Connection]](),
	}

	return &session
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

	session.state = restapi.SessionClosed
	session.eventListener.SessionStateChanged(session.id, session.state)

	err := errors.Join(errs...)
	return err
}

func (session *Session) Cancel() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.state = restapi.SessionCanceling
	session.eventListener.SessionStateChanged(session.id, session.state)

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
