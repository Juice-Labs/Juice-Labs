/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package connection

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
	ConnectionTerminated(id string, sessionId string, existStatus string)
}

type Connection struct {
	mutex     sync.Mutex
	juicePath string

	id         string
	sessionId  string
	exitStatus string

	gpus *gpu.SelectedGpuSet

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File

	eventListener EventListener
}

func New(id string, juicePath string, version string, gpus *gpu.SelectedGpuSet, sessionId string, eventListener EventListener) *Connection {
	return &Connection{
		id:            id,
		sessionId:     sessionId,
		juicePath:     juicePath,
		exitStatus:    restapi.ExitStatusUnknown,
		gpus:          gpus,
		eventListener: eventListener,
	}
}

func (connection *Connection) Id() string {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	return connection.id
}
func (connection *Connection) ExitStatus() string {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	return connection.exitStatus
}

func (connection *Connection) Connection() restapi.Connection {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	return restapi.Connection{
		Id:         connection.id,
		ExitStatus: connection.exitStatus,
	}
}

func (connection *Connection) setExitStatus(exitStatus string) {
	connection.mutex.Lock()
	if connection.exitStatus == restapi.ExitStatusUnknown {
		connection.exitStatus = exitStatus
	}
	connection.mutex.Unlock()

	connection.eventListener.ConnectionTerminated(connection.id, connection.sessionId, exitStatus)
}

func (connection *Connection) Close() error {
	connection.mutex.Lock()

	connection.cmd = nil

	err := errors.Join(
		connection.readPipe.Close(),
		connection.writePipe.Close(),
	)

	connection.gpus.Release()
	connection.gpus = nil

	connection.mutex.Unlock()

	return err
}

func (connection *Connection) Start(group task.Group) error {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	ch1Read, ch1Write, err := setupIpc()
	if err == nil {
		connection.readPipe = ch1Read
		defer ch1Write.Close()

		ch2Read, ch2Write, err_ := setupIpc()
		err = err_
		if err == nil {
			connection.writePipe = ch2Write
			defer ch2Read.Close()

			logsPath := filepath.Join(connection.juicePath, "logs")
			_, err_ = os.Stat(logsPath)
			if err_ != nil && os.IsNotExist(err_) {
				err_ = os.MkdirAll(logsPath, fs.ModeDir|fs.ModePerm)
				if err_ != nil {
					logger.Errorf("unable to create directory %s, %s", logsPath, err_.Error())
				}
			}

			now := time.Now()

			// NOTE: time.Format is really weird. The string below equates to YYYYMMDD-HHMMSS_
			logName := fmt.Sprint(now.Format("20060102-150405_"), connection.id, ".log")

			if err == nil {
				connection.cmd = exec.CommandContext(group.Ctx(),
					filepath.Join(connection.juicePath, "Renderer_Win"),
					append(
						[]string{
							"--id", connection.id,
							"--log_file", filepath.Join(logsPath, logName),
							"--ipc_write", fmt.Sprint(ch1Write.Fd()),
							"--ipc_read", fmt.Sprint(ch2Read.Fd()),
							"--pcibus", connection.gpus.GetPciBusString(),
						},
						flag.Args()[0:]...,
					)...,
				)

				inheritFiles(connection.cmd, ch1Write, ch2Read)

				err = connection.cmd.Start()

			}
		}
	}

	if err != nil {
		err = fmt.Errorf("Connection: failed to start Renderer_Win with %s", err)
		connection.mutex.Unlock()
		connection.setExitStatus(restapi.ExitStatusFailure)
		return err
	}

	return err
}

func (connection *Connection) Wait() error {
	err := connection.cmd.Wait()

	connection.mutex.Lock()
	connection.cmd = nil

	if err != nil {
		logger.Error(fmt.Sprintf("Connection: connection %s failed with %s", connection.id, err))
		connection.mutex.Unlock()
		connection.setExitStatus(restapi.ExitStatusFailure)
	} else {

		connection.mutex.Unlock()
		connection.setExitStatus(restapi.ExitStatusSuccess)
	}

	return nil
}

func (connection *Connection) Cancel() error {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	if connection.cmd != nil {
		connection.mutex.Unlock()
		connection.setExitStatus(restapi.ExitStatusCanceled)
		return connection.cmd.Cancel()
	}

	return nil
}

func (connection *Connection) Connect(c net.Conn) error {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()

	defer c.Close()

	var err error
	if connection.cmd != nil {
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
				err = connection.forwardSocket(rawConn)
				if err == nil {
					// Wait for the server to indicate it has created the socket
					data := make([]byte, 1)
					_, err = connection.readPipe.Read(data)

					// Close our socket handle
					err = errors.Join(err, c.Close())

					// And finally, inform the server that our side is closed
					_, err_ := connection.writePipe.Write(data)
					err = errors.Join(err, err_)
				}
			}
		}

		if err != nil {
			err = errors.Join(err, connection.Cancel())
		}
	}

	return err
}
