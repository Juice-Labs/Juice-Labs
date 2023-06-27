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

	"github.com/Juice-Labs/Juice-Labs/pkg/api"
	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Session struct {
	api.Session

	juicePath string

	cmd     *exec.Cmd
	cmdPipe *os.File

	connections []net.Conn

	gpus gpu.SelectedGpuSet
}

func New(id string, juicePath string, version string, gpus gpu.SelectedGpuSet) *Session {
	return &Session{
		Session: api.Session{
			Id:      id,
			State:   api.StateActive,
			Version: version,
			Gpus:    gpus.GetGpus(),
		},
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func Register(session api.Session, juicePath string, gpus gpu.SelectedGpuSet) *Session {
	return &Session{
		Session:   session,
		juicePath: juicePath,
		gpus:      gpus,
	}
}

func (session *Session) Run(group task.Group) error {
	readPipe, writePipe, err := setupIpc()
	if err == nil {
		defer readPipe.Close()

		logLevel, err := logger.LogLevelAsString()
		if err == nil {
			session.cmd = exec.CommandContext(group.Ctx(),
				filepath.Join(session.juicePath, "Renderer_Win"),
				"--id", session.Id,
				"--log_group", logLevel,
				"--log_file", filepath.Join(session.juicePath, "logs", fmt.Sprint(session.Id, ".log")),
				"--go_ipc", fmt.Sprint(readPipe.Fd()),
				"--pcibus", session.gpus.GetPciBusString())

			inheritFile(session.cmd, readPipe)

			session.cmdPipe = writePipe

			err = session.cmd.Start()
			if err == nil {
				err = session.cmd.Wait()

				for _, conn := range session.connections {
					err = errors.Join(err, conn.Close())
				}

				session.gpus.Release()
			}
		}
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
