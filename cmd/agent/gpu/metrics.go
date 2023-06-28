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
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type Metrics struct {
	Name              string `json:"name"`
	UtcWhen           uint64 `json:"utcWhen"`
	GpuUtilization    uint32 `json:"gpuUtilization"`
	MemoryUtilization uint32 `json:"memoryUtilization"`
	MemoryUsed        uint64 `json:"memoryUsed"`
	MemoryTotal       uint64 `json:"memoryTotal"`
	PowerUsage        uint32 `json:"powerUsage"`
	PowerLimit        uint32 `json:"powerLimit"`
	FanSpeed          uint32 `json:"fanSpeed"`
	TemperatureGpu    uint32 `json:"temperatureGpu"`
	TemperatureMemory uint32 `json:"temperatureMemory"`
	ClockCore         uint32 `json:"clockCore"`
	ClockMemory       uint32 `json:"clockMemory"`
}

var (
	disableGpuMetrics  = flag.Bool("disable-gpu-metrics", false, "")
	gpuMetricsInterval = flag.Uint("gpu-metrics-interval-ms", 1000, "")
)

type MetricsConsumerFn = func([]Metrics)

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
			"--gpu_watcher", fmt.Sprint(*gpuMetricsInterval),
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
			var metrics []Metrics
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
