/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package prometheus

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Juice-Labs/Juice-Labs/cmd/agent/gpu"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
)

const (
	namespace = "juice"
	subsystem = "agent"
)

type gpuCollector struct {
	sync.Mutex

	ClockCore       *prometheus.GaugeVec
	ClockMemory     *prometheus.GaugeVec
	UtilizationGpu  *prometheus.GaugeVec
	UtilizationVram *prometheus.GaugeVec
	TemperatureGpu  *prometheus.GaugeVec
	VramUsed        *prometheus.GaugeVec
	Vram            *prometheus.GaugeVec
	PowerDraw       *prometheus.GaugeVec
	PowerLimit      *prometheus.GaugeVec
	FanSpeed        *prometheus.GaugeVec
}

func newGpuCollector() *gpuCollector {
	labels := []string{"index", "name"}

	return &gpuCollector{
		ClockCore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "clock_core",
			},
			labels,
		),
		ClockMemory: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "clock_memory",
			},
			labels,
		),
		UtilizationGpu: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "utilization_gpu",
			},
			labels,
		),
		UtilizationVram: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "utilization_memory",
			},
			labels,
		),
		TemperatureGpu: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "temperature_gpu",
			},
			labels,
		),
		VramUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_used",
			},
			labels,
		),
		Vram: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_total",
			},
			labels,
		),
		PowerDraw: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "power_draw",
			},
			labels,
		),
		PowerLimit: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "power_limit",
			},
			labels,
		),
		FanSpeed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "fan_speed",
			},
			labels,
		),
	}
}

func (c *gpuCollector) Describe(ch chan<- *prometheus.Desc) {
	c.ClockCore.Describe(ch)
	c.ClockMemory.Describe(ch)
	c.UtilizationGpu.Describe(ch)
	c.UtilizationVram.Describe(ch)
	c.TemperatureGpu.Describe(ch)
	c.VramUsed.Describe(ch)
	c.Vram.Describe(ch)
	c.PowerDraw.Describe(ch)
	c.PowerLimit.Describe(ch)
	c.FanSpeed.Describe(ch)
}

func (c *gpuCollector) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	c.ClockCore.Collect(ch)
	c.ClockMemory.Collect(ch)
	c.UtilizationGpu.Collect(ch)
	c.UtilizationVram.Collect(ch)
	c.TemperatureGpu.Collect(ch)
	c.VramUsed.Collect(ch)
	c.Vram.Collect(ch)
	c.PowerDraw.Collect(ch)
	c.PowerLimit.Collect(ch)
	c.FanSpeed.Collect(ch)
}

func NewGpuMetricsConsumer() gpu.MetricsConsumerFn {
	collector := newGpuCollector()
	prometheus.MustRegister(collector)

	return func(metrics []restapi.Gpu) {
		collector.Lock()
		defer collector.Unlock()

		for index, gpu := range metrics {
			collector.ClockCore.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.ClockCore))
			collector.ClockMemory.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.ClockMemory))
			collector.UtilizationGpu.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.UtilizationGpu))
			collector.UtilizationVram.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.UtilizationVram))
			collector.TemperatureGpu.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.TemperatureGpu))
			collector.VramUsed.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.VramUsed))
			collector.Vram.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Vram))
			collector.PowerDraw.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.PowerDraw))
			collector.PowerLimit.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.PowerLimit))
			collector.FanSpeed.WithLabelValues(strconv.Itoa(index), gpu.Name).Set(float64(gpu.Metrics.FanSpeed))
		}
	}
}
