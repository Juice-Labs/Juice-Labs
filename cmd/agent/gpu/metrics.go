/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package gpu

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"

	"github.com/Juice-Labs/Juice-Labs/pkg/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	disableGpuMetrics  = flag.Bool("disable-gpu-metrics", false, "")
	gpuMetricsInterval = flag.Uint("gpu-metrics-interval-ms", 1000, "")
)

type MetricsConsumerFn = func([]restapi.Gpu)

type MetricsProvider struct {
	consumers []MetricsConsumerFn

	gpus            gpu.GpuSet
	rendererWinPath string
}

func NewMetricsProvider(gpus gpu.GpuSet, rendererWinPath string) *MetricsProvider {
	return &MetricsProvider{
		gpus:            gpus,
		rendererWinPath: rendererWinPath,
	}
}

func (provider *MetricsProvider) AddConsumer(consumer MetricsConsumerFn) {
	provider.consumers = append(provider.consumers, consumer)
}

func (provider *MetricsProvider) Run(group task.Group) error {
	if !*disableGpuMetrics && len(provider.consumers) > 0 {
		cmd := exec.CommandContext(group.Ctx(), provider.rendererWinPath,
			"--log_group", "Fatal",
			"--dump_gpus", fmt.Sprint(*gpuMetricsInterval),
			"--pcibus", provider.gpus.GetPciBusString())

		stdoutReader, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}

		err = cmd.Start()
		if err != nil {
			return err
		}

		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			var metrics []restapi.Gpu
			err := json.Unmarshal(scanner.Bytes(), &metrics)
			if err == nil {
				for _, consumer := range provider.consumers {
					consumer(metrics)
				}
			} else {
				logger.Warning(err)
			}
		}

		return cmd.Wait()
	}

	return nil
}
