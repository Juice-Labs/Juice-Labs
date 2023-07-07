/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Session struct {
	sessionMutex sync.Mutex
	session      restapi.Session

	id        string
	juicePath string

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File

	connections []net.Conn

	gpus *gpu.SelectedGpuSet
}

func New(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet) *Session {
	return &Session{
		session: restapi.Session{
			Id:      id,
			State:   restapi.StateAssigned,
			Version: version,
			Gpus:    gpus.GetGpus(),
		},
		id:        id,
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func Register(apiSession restapi.Session, juicePath string, gpus *gpu.SelectedGpuSet) *Session {
	return &Session{
		session:   apiSession,
		id:        apiSession.Id,
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func (session *Session) Id() string {
	return session.id
}

func (session *Session) Session() restapi.Session {
	session.sessionMutex.Lock()
	defer session.sessionMutex.Unlock()
	return session.session
}

func (session *Session) Close() error {
	var err error
	for _, connection := range session.connections {
		err = errors.Join(err, connection.Close())
	}

	return err
}

func (session *Session) Run(group task.Group) error {
	ch1Read, ch1Write, err := setupIpc()
	if err == nil {
		defer ch1Read.Close()
		defer ch1Write.Close()

		ch2Read, ch2Write, err_ := setupIpc()
		err = err_
		if err == nil {
			defer ch2Read.Close()
			defer ch2Write.Close()

			logLevel, err_ := logger.LogLevelAsString()
			err = err_
			if err_ == nil {
				session.readPipe = ch1Read
				session.writePipe = ch2Write

				session.cmd = exec.CommandContext(group.Ctx(),
					filepath.Join(session.juicePath, "Renderer_Win"),
					"--id", session.id,
					"--log_group", logLevel,
					"--log_file", filepath.Join(session.juicePath, "logs", fmt.Sprint(session.id, ".log")),
					"--ipc_write", fmt.Sprint(ch1Write.Fd()),
					"--ipc_read", fmt.Sprint(ch2Read.Fd()),
					"--pcibus", session.gpus.GetPciBusString())

				inheritFiles(session.cmd, ch1Write, ch2Read)

				err = session.cmd.Start()
				if err == nil {
					session.sessionMutex.Lock()
					session.session.State = restapi.StateActive
					session.sessionMutex.Unlock()

					err = session.cmd.Wait()
				}

				session.readPipe = nil
				session.writePipe = nil
				session.cmd = nil
			}
		}
	}

	if err != nil {
		err = fmt.Errorf("Session: failed to start Renderer_Win with %s", err)
	}

	return err
}

func (session *Session) Signal() error {
	return session.cmd.Cancel()
}

func (session *Session) Connect(c net.Conn) error {
	defer c.Close()

	tcpConn := &net.TCPConn{}
	tlsConn, err := utilities.Cast[*tls.Conn](c)
	if err == nil {
		tcpConn, err = utilities.Cast[*net.TCPConn](tlsConn.NetConn())
	} else {
		tcpConn, err = utilities.Cast[*net.TCPConn](c)
	}

	if err == nil {
		rawConn, err_ := tcpConn.SyscallConn()
		err = err_
		if err == nil {
			err = session.forwardSocket(rawConn)
			if err == nil {
				// Wait for the server to indicate it has created the socket
				data := make([]byte, 1)
				_, err = session.readPipe.Read(data)

				// Close our socket handle
				err = errors.Join(err, c.Close())

				// And finally, inform the server that our side is closed
				_, err_ := session.writePipe.Write(data)
				err = errors.Join(err, err_)
			}
		}
	}

	if err != nil {
		err = errors.Join(err, session.Signal())
	}

	return err
}
