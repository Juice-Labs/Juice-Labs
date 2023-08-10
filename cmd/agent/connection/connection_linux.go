/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package connection

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func setupIpc() (*os.File, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, err
	}
	readPipe := os.NewFile(uintptr(fds[0]), "read")
	writePipe := os.NewFile(uintptr(fds[1]), "write")

	return readPipe, writePipe, nil
}

func inheritFiles(cmd *exec.Cmd, files ...*os.File) {
	for _, f := range files {
		cmd.ExtraFiles = append(cmd.ExtraFiles, f)
	}
}

func (connection *Connection) forwardSocket(rawConn syscall.RawConn) error {
	var rights []byte
	err := rawConn.Control(func(fd uintptr) {
		rights = unix.UnixRights(int(fd))
	})
	if err != nil {
		return err
	}

	_, err = unix.SendmsgN(int(connection.writePipe.Fd()), nil, rights, nil, 0)
	return err
}
