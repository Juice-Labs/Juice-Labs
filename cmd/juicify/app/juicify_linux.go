/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package app

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"unsafe"

	"github.com/NVIDIA/go-nvml/pkg/dl"

	"github.com/Juice-Labs/Juice-Labs/pkg/errors"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

// #cgo LDFLAGS: -ldl
// #include "version_linux.h"
import "C"

func getVersion() (string, error) {
	libPath := filepath.Join(*juicePath, "libjuiceclient.so")
	libPathBytes := make([]byte, len(libPath)+1)
	copy(libPathBytes, libPath)

	var err *C.char
	version := C.GetJuiceVersion((*C.char)(unsafe.Pointer(&libPathBytes[0])), (**C.char)(unsafe.Pointer(&err)))
	if err != nil {
		return "", errors.Newf("dlerror => %s", C.GoString(err))
	}

	return C.GoString(version), nil
}

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

	var err error
	for _, name := range names {
		err = errors.Join(err, check(name))
	}

	return err
}

func createCommand(args []string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...)
}

func runCommand(group task.Group, cmd *exec.Cmd, config Configuration) error {
	// LD_PRELOAD Juice and setup appropriate Vulkan loader quirks

	juiceLibraryPath := filepath.Join(*juicePath, "libjuicejuda.so")

	cmd.Env = append(cmd.Env, fmt.Sprintf("LD_PRELOAD=%s", juiceLibraryPath))

	return cmd.Run()
}
