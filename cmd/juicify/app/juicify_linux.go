/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/NVIDIA/go-nvml/pkg/dl"
)

func check(name string) error {
	lib := dl.New(name, dl.RTLD_NOW)
	err := lib.Open()
	if err != nil {
		return err
	}

	return lib.Close()
}

func validateHost() error {
	names := []string{
		"libstdc++.so.6",
		"libvulkan.so.1",
	}

	for _, name := range names {
		err := check(name)
		if err != nil {
			return err
		}
	}

	return nil
}

func createCommand(args []string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}

func updateCommand(cmd *exec.Cmd, config Configuration) error {
	// LD_PRELOAD Juice and setup appropriate Vulkan loader quirks

	juiceLibraryPath := filepath.Join(*juicePath, "libjuicejuda.so")

	cmd.Env = append(os.Environ(), fmt.Sprintf("LD_PRELOAD=%s", juiceLibraryPath))

	return nil
}
