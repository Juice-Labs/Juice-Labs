/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package gpu

import (
	"fmt"
	"os/exec"

	pkggpu "github.com/Juice-Labs/Juice-Labs/pkg/gpu"
)

func DetectGpus(rendererWinPath string) (pkggpu.GpuSet, error) {
	var gpus pkggpu.GpuSet

	cmd := exec.Command(rendererWinPath,
		"--log_group", "Fatal",
		"--dump_gpus", "0")
	output, err := cmd.Output()
	if err != nil {
		return gpus, err
	}

	if cmd.ProcessState.ExitCode() == 0 {
		return pkggpu.UnmarshalGpuSet(output)
	}

	return gpus, fmt.Errorf("process Renderer_Win exited with %d", cmd.ProcessState.ExitCode())
}
