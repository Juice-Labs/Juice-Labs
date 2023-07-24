/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type EventListener interface {
	SessionStateChanged(id string, state string)
}

type Session struct {
	mutex sync.Mutex

	id        string
	juicePath string
	version   string

	state      string
	exitStatus string

	gpus *gpu.SelectedGpuSet

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File

	eventListener EventListener
}

func New(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, eventListener EventListener) *Session {
	return &Session{
		id:            id,
		juicePath:     juicePath,
		version:       version,
		state:         restapi.SessionActive,
		exitStatus:    restapi.ExitStatusUnknown,
		gpus:          gpus,
		eventListener: eventListener,
	}
}

func (session *Session) Id() string {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	return session.id
}

func (session *Session) Session() restapi.Session {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	return restapi.Session{
		Id:         session.id,
		State:      session.state,
		ExitStatus: session.exitStatus,
		Version:    session.version,
		Gpus:       session.gpus.GetGpus(),
	}
}

func (session *Session) setExitStatus(exitStatus string) {
	if session.exitStatus == restapi.ExitStatusUnknown {
		session.exitStatus = exitStatus
	}
}

func (session *Session) changeState(newState string) {
	session.eventListener.SessionStateChanged(session.id, newState)
	session.state = newState
}

func (session *Session) Close() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	session.cmd = nil

	err := errors.Join(
		session.readPipe.Close(),
		session.writePipe.Close(),
	)

	session.gpus.Release()
	session.gpus = nil

	session.changeState(restapi.SessionClosed)

	return err
}

func (session *Session) Start(group task.Group) error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	ch1Read, ch1Write, err := setupIpc()
	if err == nil {
		session.readPipe = ch1Read
		defer ch1Write.Close()

		ch2Read, ch2Write, err_ := setupIpc()
		err = err_
		if err == nil {
			session.writePipe = ch2Write
			defer ch2Read.Close()

			logsPath := filepath.Join(session.juicePath, "logs")
			_, err_ = os.Stat(logsPath)
			if err_ != nil && os.IsNotExist(err_) {
				err_ = os.MkdirAll(logsPath, fs.ModeDir|fs.ModePerm)
				if err_ != nil {
					logger.Errorf("unable to create directory %s, %s", logsPath, err_.Error())
				}
			}

			now := time.Now()

			// NOTE: time.Format is really weird. The string below equates to YYYYMMDD-HHMMSS_
			logName := fmt.Sprint(now.Format("20060102-150405_"), session.id, ".log")

			if err == nil {
				session.cmd = exec.CommandContext(group.Ctx(),
					filepath.Join(session.juicePath, "Renderer_Win"),
					append(
						[]string{
							"--id", session.id,
							"--log_file", filepath.Join(logsPath, logName),
							"--ipc_write", fmt.Sprint(ch1Write.Fd()),
							"--ipc_read", fmt.Sprint(ch2Read.Fd()),
							"--pcibus", session.gpus.GetPciBusString(),
						},
						flag.Args()[0:]...,
					)...,
				)

				inheritFiles(session.cmd, ch1Write, ch2Read)

				err = session.cmd.Start()

				session.changeState(restapi.SessionActive)
			}
		}
	}

	if err != nil {
		err = fmt.Errorf("Session: failed to start Renderer_Win with %s", err)
		session.setExitStatus(restapi.ExitStatusFailure)
	}

	return err
}

func (session *Session) Wait() error {
	err := session.cmd.Wait()

	session.mutex.Lock()
	defer session.mutex.Unlock()

	if err != nil {
		logger.Error(fmt.Sprintf("Session: session %s failed with %s", session.id, err))
		session.setExitStatus(restapi.ExitStatusFailure)
	} else {
		session.setExitStatus(restapi.ExitStatusSuccess)
	}

	session.cmd = nil
	return nil
}

func (session *Session) Cancel() error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.cmd != nil {
		session.setExitStatus(restapi.ExitStatusCanceled)
		return session.cmd.Cancel()
	}

	return nil
}

func (session *Session) Connect(c net.Conn) error {
	session.mutex.Lock()
	defer session.mutex.Unlock()

	defer c.Close()

	var err error
	if session.cmd != nil {
		tcpConn := &net.TCPConn{}
		tlsConn, err_ := utilities.Cast[*tls.Conn](c)
		err = err_
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
	}

	return err
}
