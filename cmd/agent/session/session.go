/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"context"
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
	restapi.Session

	juicePath string

	cmd      *exec.Cmd
	toPipe   *os.File
	fromPipe *os.File

	connections []net.Conn

	gpus gpu.SelectedGpuSet
}

func New(id string, juicePath string, version string, gpus gpu.SelectedGpuSet) *Session {
	return &Session{
		Session: restapi.Session{
			Id:      id,
			State:   restapi.StateActive,
			Version: version,
			Gpus:    gpus.GetGpus(),
		},
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func Register(apisession restapi.Session, juicePath string, gpus gpu.SelectedGpuSet) *Session {
	return &Session{
		Session:   apisession,
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func (session *Session) Start(ctx context.Context) error {
	readPipe, writePipe, err := setupIpc()
	if err == nil {
		logLevel, err_ := logger.LogLevelAsString()
		err = err_
		if err_ == nil {
			session.cmd = exec.CommandContext(ctx,
				filepath.Join(session.juicePath, "Renderer_Win"),
				"--id", session.Id,
				"--log_group", logLevel,
				"--log_file", filepath.Join(session.juicePath, "logs", fmt.Sprint(session.Id, ".log")),
				"--go_ipc", fmt.Sprint(readPipe.Fd()),
				"--pcibus", session.gpus.GetPciBusString())

			inheritFile(session.cmd, readPipe)

			err = session.cmd.Start()
			if err == nil {
				session.toPipe = writePipe
				session.fromPipe = readPipe
			}
		}

		if err != nil {
			err = errors.Join(err, writePipe.Close())
			err = errors.Join(err, readPipe.Close())
		}
	}

	return err
}

func (session *Session) Run(group task.Group) error {
	err := session.cmd.Wait()

	for _, conn := range session.connections {
		err = errors.Join(err, conn.Close())
	}

	err = errors.Join(err, session.toPipe.Close())
	err = errors.Join(err, session.fromPipe.Close())

	session.gpus.Release()

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
		rawConn, err := tcpConn.SyscallConn()
		if err == nil {
			err = session.forwardSocket(rawConn)
			if err == nil {
				session.connections = append(session.connections, c)
			}
		}
	}

	return err
}
