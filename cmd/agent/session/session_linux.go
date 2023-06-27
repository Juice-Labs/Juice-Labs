/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/Juice-Labs/pkg/logger"
)

func setupIpc() (*os.File, *os.File, error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM, 0)
	if err != nil {
		logger.Error(err)
		return nil, nil, err
	}
	readPipe := os.NewFile(uintptr(fds[0]), "read")
	writePipe := os.NewFile(uintptr(fds[1]), "write")

	return readPipe, writePipe, nil
}

func inheritFile(cmd *exec.Cmd, f *os.File) {
	cmd.ExtraFiles = append(cmd.ExtraFiles, f)
}

func (session *Session) forwardSocket(rawConn syscall.RawConn) error {
	var rights []byte
	err := rawConn.Control(func(fd uintptr) {
		rights = unix.UnixRights(int(fd))
	})
	if err != nil {
		return err
	}

	_, err = unix.SendmsgN(int(session.serverPipe.Fd()), nil, rights, nil, 0)
	return err
}
