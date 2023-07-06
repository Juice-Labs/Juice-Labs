/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"crypto/tls"
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

	cmd      *exec.Cmd
	toPipe   *os.File
	fromPipe *os.File

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

func (session *Session) Run(group task.Group) error {
	readPipe, writePipe, err := setupIpc()
	if err == nil {
		defer writePipe.Close()
		defer readPipe.Close()

		logLevel, err_ := logger.LogLevelAsString()
		err = err_
		if err_ == nil {
			session.toPipe = writePipe
			session.fromPipe = readPipe

			session.cmd = exec.CommandContext(group.Ctx(),
				filepath.Join(session.juicePath, "Renderer_Win"),
				"--id", session.id,
				"--log_group", logLevel,
				"--log_file", filepath.Join(session.juicePath, "logs", fmt.Sprint(session.id, ".log")),
				"--go_ipc", fmt.Sprint(readPipe.Fd()),
				"--pcibus", session.gpus.GetPciBusString())

			inheritFile(session.cmd, readPipe)

			err = session.cmd.Start()
			if err == nil {
				session.sessionMutex.Lock()
				session.session.State = restapi.StateActive
				session.sessionMutex.Unlock()

				err = session.cmd.Wait()
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
		}
	}

	return err
}
