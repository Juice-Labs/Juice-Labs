/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"os/exec"
	"path/filepath"
	"unsafe"

	"github.com/kolesnikovae/go-winjob"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	pkgWinjob "github.com/Juice-Labs/Juice-Labs/pkg/winjob"
)

// #include "version_windows.h"
import "C"

func getVersion() (string, error) {
	libPath := filepath.Join(*juicePath, "juiceclient.dll")
	libPathBytes := make([]byte, len(libPath)+1)
	copy(libPathBytes, libPath)

	var err int32
	version := C.GetJuiceVersion((*C.char)(unsafe.Pointer(&libPathBytes[0])), (*C.int)(unsafe.Pointer(&err)))
	if err != 0 {
		return "", errors.Newf("GetLastError => %d", err)
	}

	return C.GoString(version), nil
}

func validateHost() error {
	return nil
}

func createCommand(args []string) *exec.Cmd {
	return exec.Command(filepath.Join(*juicePath, "launch.exe"), args...)
}

func runCommand(group task.Group, cmd *exec.Cmd, config Configuration) error {
	// Windows jobs intermittently generate "SetInformationJobOject: The
	// parameter is incorrect" errors when trying to execute launch.exe to
	// spawn and inject Juice into an application.  Also juicify doesn't exit
	// when the launched process exits when using Windows jobs.
	//
	// Commented out for now to preserve existing behavior from the C++
	// version of juicify.  See https://github.com/Juice-Labs/juice/issues/1756
	// and https://github.com/Juice-Labs/juice/issues/1765.

	job, err := pkgWinjob.CreateAnonymous(winjob.WithKillOnJobClose())
	if err != nil {
		return err
	}
	defer job.Close()

	notificationChannel := make(chan winjob.Notification, 1)
	subscription, err := winjob.Notify(notificationChannel, job)
	if err != nil {
		return err
	}
	defer subscription.Close()

	err = winjob.StartInJobObject(cmd, job)
	if err != nil {
		return err
	}

	for {
		select {
		case <-group.Ctx().Done():
			return nil

		case n := <-notificationChannel:
			if n.Type == winjob.NotificationActiveProcessZero {
				return nil
			}
		}
	}
}
