/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"os/exec"
	"path/filepath"

	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/kolesnikovae/go-winjob"
)

func validateHost() error {
	return nil
}

func createCommand(args []string) *exec.Cmd {
	return exec.Command(filepath.Join(*juicePath, "launch.exe"), args...)
}

func runCommand(group task.Group, cmd *exec.Cmd, config Configuration) error {
	job, err := winjob.Create("Juicify", winjob.WithKillOnJobClose())
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
