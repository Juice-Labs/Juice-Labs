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

	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Session struct {
	id        string
	juicePath string
	version   string

	state int

	gpus *gpu.SelectedGpuSet

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File
}

func New(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet) *Session {
	return &Session{
		id:        id,
		juicePath: juicePath,
		version:   version,
		state:     restapi.SessionActive,
		gpus:      gpus,
	}
}

func (session *Session) Id() string {
	return session.id
}

func (session *Session) Session() restapi.Session {
	return restapi.Session{
		Id:      session.id,
		State:   session.state,
		Version: session.version,
		Gpus:    session.gpus.GetGpus(),
	}
}

func (session *Session) Close() error {
	session.cmd = nil

	err := errors.Join(
		session.readPipe.Close(),
		session.writePipe.Close(),
	)

	session.gpus.Release()
	session.gpus = nil

	session.state = restapi.SessionClosed
	return err
}

func (session *Session) Run(group task.Group) error {
	ch1Read, ch1Write, err := setupIpc()
	if err == nil {
		session.readPipe = ch1Read
		defer ch1Write.Close()

		ch2Read, ch2Write, err_ := setupIpc()
		err = err_
		if err == nil {
			session.writePipe = ch2Write
			defer ch2Read.Close()

			logLevel, err_ := logger.LogLevelAsString()
			err = err_
			if err_ == nil {
				session.cmd = exec.CommandContext(group.Ctx(),
					filepath.Join(session.juicePath, "Renderer_Win"),
					"--id", session.id,
					"--log_group", logLevel,
					"--log_file", filepath.Join(session.juicePath, "logs", fmt.Sprint(session.id, ".log")),
					"--ipc_write", fmt.Sprint(ch1Write.Fd()),
					"--ipc_read", fmt.Sprint(ch2Read.Fd()),
					"--pcibus", session.gpus.GetPciBusString())

				inheritFiles(session.cmd, ch1Write, ch2Read)

				err = session.cmd.Run()
			}
		}
	}

	if err != nil {
		err = fmt.Errorf("Session: failed to start Renderer_Win with %s", err)
	}

	return err
}

func (session *Session) Cancel() error {
	session.state = restapi.SessionCanceled
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
		err = errors.Join(err, session.Cancel())
	}

	return err
}
