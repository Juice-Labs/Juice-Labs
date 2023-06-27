/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"os/exec"
	"path/filepath"
)

func validateHost() error {
	return nil
}

func createCommand(args []string) *exec.Cmd {
	return exec.Command(filepath.Join(*juicePath, "launch.exe"), args...)
}

func updateCommand(cmd *exec.Cmd, config Configuration) error {
	return nil
}
