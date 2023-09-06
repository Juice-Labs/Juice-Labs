/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Connection struct {
	sync.Mutex

	connectionData restapi.ConnectionData

	juicePath  string
	pciBus     string
	exitStatus string

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File
}

func newConnection(connectionData restapi.ConnectionData, juicePath string, pciBus string) *Connection {
	return &Connection{
		connectionData: connectionData,
		juicePath:      juicePath,
		pciBus:         pciBus,
		exitStatus:     restapi.ExitStatusUnknown,
	}
}

func (connection *Connection) Id() string {
	return connection.connectionData.Id
}

func (connection *Connection) ExitStatus() string {
	connection.Lock()
	defer connection.Unlock()

	return connection.exitStatus
}

func (connection *Connection) Connection() restapi.Connection {
	return restapi.Connection{
		ConnectionData: connection.connectionData,
		ExitStatus:     connection.ExitStatus(),
	}
}

func (connection *Connection) setExitStatus(exitStatus string) {
	connection.Lock()

	if connection.exitStatus != restapi.ExitStatusUnknown {
		panic("connection exit status set multiple times")
	}

	connection.exitStatus = exitStatus
	connection.Unlock()
}

func (connection *Connection) Close() error {
	connection.Lock()

	if connection.cmd != nil {
		return connection.cmd.Cancel()
	}

	err := errors.Join(
		connection.readPipe.Close(),
		connection.writePipe.Close(),
	)

	connection.Unlock()

	return err
}

func (connection *Connection) Start(group task.Group) error {
	connection.Lock()

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
			_, err = os.Stat(logsPath)
			if err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(logsPath, fs.ModeDir|fs.ModePerm)
			}

			if err == nil {
				now := time.Now()

				// NOTE: time.Format is really weird. The string below equates to YYYYMMDD-HHMMSS_
				logName := fmt.Sprintf("%s_%s_%s.log", connection.connectionData.ProcessName, now.Format("20060102-150405_"), connection.connectionData.Id)

				if err == nil {
					connection.cmd = exec.CommandContext(group.Ctx(),
						filepath.Join(connection.juicePath, "Renderer_Win"),
						append(
							[]string{
								"--id", connection.connectionData.Id,
								"--log_file", filepath.Join(logsPath, logName),
								"--ipc_write", fmt.Sprint(ch1Write.Fd()),
								"--ipc_read", fmt.Sprint(ch2Read.Fd()),
								"--pcibus", connection.pciBus,
							},
							flag.Args()[0:]...,
						)...,
					)

					inheritFiles(connection.cmd, ch1Write, ch2Read)

					err = connection.cmd.Start()
				}
			}
		}
	}

	connection.Unlock()

	if err != nil {
		err = errors.New("failed to start Renderer_Win").Wrap(err)

		connection.setExitStatus(restapi.ExitStatusFailure)
	}

	return err
}

func (connection *Connection) Wait() error {
	err := connection.cmd.Wait()
	if err != nil {
		connection.setExitStatus(restapi.ExitStatusFailure)

		err = errors.Newf("connection %s failed", connection.connectionData.Id).Wrap(err)
	} else {
		connection.setExitStatus(restapi.ExitStatusSuccess)
	}

	return err
}

func (connection *Connection) Cancel() error {
	connection.Lock()
	defer connection.Unlock()

	if connection.exitStatus == restapi.ExitStatusUnknown {
		connection.setExitStatus(restapi.ExitStatusCanceled)

		if connection.cmd != nil {
			return connection.cmd.Cancel()
		}
	}

	return nil
}

func (connection *Connection) Connect(c net.Conn) error {
	connection.Lock()
	defer connection.Unlock()

	defer c.Close()

	var err error
	if connection.cmd != nil {
		tcpConn := &net.TCPConn{}

		var tlsConn *tls.Conn
		tlsConn, err := utilities.Cast[*tls.Conn](c)
		if err == nil {
			tcpConn, err = utilities.Cast[*net.TCPConn](tlsConn.NetConn())
		} else {
			tcpConn, err = utilities.Cast[*net.TCPConn](c)
		}

		if err == nil {
			var rawConn syscall.RawConn
			rawConn, err := tcpConn.SyscallConn()
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

	err = errors.Join(err, c.Close())

	if err != nil {
		err = errors.New("failed to connect to Renderer_Win").Wrap(err)
	}

	return err
}
