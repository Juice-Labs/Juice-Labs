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
	"syscall"
	"time"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/Juice-Labs/Juice-Labs/pkg/utilities"
)

type Connection struct {
	restapi.ConnectionData

	juicePath string
	pciBus    string

	cmd       *exec.Cmd
	readPipe  *os.File
	writePipe *os.File
}

func newConnection(connectionData restapi.ConnectionData, juicePath string, pciBus string) *Connection {
	return &Connection{
		ConnectionData: connectionData,
		juicePath:      juicePath,
		pciBus:         pciBus,
	}
}

func (connection *Connection) Start(group task.Group, exitCode chan int) error {
	if connection.cmd != nil {
		panic("must call Start() exactly once")
	}

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
				logName := fmt.Sprintf("%s_%s_%s.log", connection.ProcessName, now.Format("20060102-150405"), connection.Id)

				if err == nil {
					connection.cmd = exec.CommandContext(group.Ctx(),
						filepath.Join(connection.juicePath, "Renderer_Win"),
						append(
							[]string{
								"--id", connection.Id,
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
					if err == nil {
						group.GoFn(fmt.Sprintf("connection %s", connection.Id), func(g task.Group) error {
							err := errors.Join(
								connection.cmd.Wait(),
								connection.readPipe.Close(),
								connection.writePipe.Close(),
							)
							if err != nil {
								err = errors.Newf("connection %s failed to wait", connection.Id).Wrap(err)
							}

							exitCode <- connection.cmd.ProcessState.ExitCode()

							return err
						})
					}
				}
			}
		}
	}

	if err != nil {
		err = errors.Join(err,
			connection.readPipe.Close(),
			connection.writePipe.Close())

		err = errors.Newf("connection %s failed to start", connection.Id).Wrap(err)
	}

	return err
}

func (connection *Connection) Connect(c net.Conn) error {
	if connection.cmd == nil {
		panic("must call Start() successfully first")
	}

	tcpConn := &net.TCPConn{}
	tlsConn, err := utilities.Cast[*tls.Conn](c)
	if err == nil {
		tcpConn, err = utilities.Cast[*net.TCPConn](tlsConn.NetConn())
	} else {
		tcpConn, err = utilities.Cast[*net.TCPConn](c)
	}

	if err == nil {
		var rawConn syscall.RawConn
		rawConn, err = tcpConn.SyscallConn()
		if err == nil {
			err = connection.forwardSocket(rawConn)
			if err == nil {
				// Wait for the server to indicate it is ready
				data := make([]byte, 1)
				_, err = connection.readPipe.Read(data)

				// And finally, inform the server to continue
				_, err_ := connection.writePipe.Write(data)
				err = errors.Join(err, err_)
			}
		}
	}

	err = errors.Join(err, c.Close())
	if err != nil {
		err = errors.Newf("connection %s failed to connect", connection.Id).Wrap(err)
	}

	return err
}
