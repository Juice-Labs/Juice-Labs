/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package session

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
)

func inheritFiles(cmd *exec.Cmd, files ...*os.File) {
	for _, f := range files {
		cmd.ExtraFiles = append(cmd.ExtraFiles, f)
	}
}

func (session *Session) forwardSocket(rawConn syscall.RawConn) error {
	var rights []byte
	err := rawConn.Control(func(fd uintptr) {
		rights = unix.UnixRights(int(fd))
	})
	if err != nil {
		return err
	}

	_, err = unix.SendmsgN(int(session.writePipe.Fd()), nil, rights, nil, 0)
	return err
}
