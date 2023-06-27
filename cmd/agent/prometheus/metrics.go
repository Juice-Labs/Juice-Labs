/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package prometheus

import (
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	gpuMetrics "github.com/Juice-Labs/cmd/agent/gpu/metrics"
)

const (
	namespace = "juice"
	subsystem = "agent"
)

type gpuCollector struct {
	sync.Mutex

	when              prometheus.Gauge
	gpuUtilization    *prometheus.GaugeVec
	memoryUtilization *prometheus.GaugeVec
	memoryUsed        *prometheus.GaugeVec
	memoryTotal       *prometheus.GaugeVec
	powerUsage        *prometheus.GaugeVec
	powerLimit        *prometheus.GaugeVec
	fanSpeed          *prometheus.GaugeVec
	gpuTemperature    *prometheus.GaugeVec
	memoryTemperature *prometheus.GaugeVec
	coreClock         *prometheus.GaugeVec
	memoryClock       *prometheus.GaugeVec
}

func newGpuCollector() *gpuCollector {
	labels := []string{"index", "name"}

	return &gpuCollector{
		when: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "utc",
			},
		),
		gpuUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "gpu_utilization",
			},
			labels,
		),
		memoryUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_utilization",
			},
			labels,
		),
		memoryUsed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_used",
			},
			labels,
		),
		memoryTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_total",
			},
			labels,
		),
		powerUsage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "power_usage",
			},
			labels,
		),
		powerLimit: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "power_limit",
			},
			labels,
		),
		fanSpeed: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "fan_speed",
			},
			labels,
		),
		gpuTemperature: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "gpu_temperature",
			},
			labels,
		),
		memoryTemperature: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_temperature",
			},
			labels,
		),
		coreClock: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "gpu_clock",
			},
			labels,
		),
		memoryClock: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Subsystem: subsystem,
				Name:      "memory_clock",
			},
			labels,
		),
	}
}

func (c *gpuCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.when.Desc()
	c.gpuUtilization.Describe(ch)
	c.memoryUtilization.Describe(ch)
	c.memoryUsed.Describe(ch)
	c.memoryTotal.Describe(ch)
	c.powerUsage.Describe(ch)
	c.powerLimit.Describe(ch)
	c.fanSpeed.Describe(ch)
	c.gpuTemperature.Describe(ch)
	c.memoryTemperature.Describe(ch)
	c.coreClock.Describe(ch)
	c.memoryClock.Describe(ch)
}

func (c *gpuCollector) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	ch <- c.when
	c.gpuUtilization.Collect(ch)
	c.memoryUtilization.Collect(ch)
	c.memoryUsed.Collect(ch)
	c.memoryTotal.Collect(ch)
	c.powerUsage.Collect(ch)
	c.powerLimit.Collect(ch)
	c.fanSpeed.Collect(ch)
	c.gpuTemperature.Collect(ch)
	c.memoryTemperature.Collect(ch)
	c.coreClock.Collect(ch)
	c.memoryClock.Collect(ch)
}

func NewConsumer() gpuMetrics.ConsumerFn {
	collector := newGpuCollector()
	prometheus.MustRegister(collector)

	return func(metrics []gpuMetrics.Metrics) {
		collector.Lock()
		defer collector.Unlock()

		// UtcWhen should all be the same
		collector.when.Set(float64(metrics[0].UtcWhen))
		for index, metric := range metrics {
			collector.gpuUtilization.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.GpuUtilization))
			collector.memoryUtilization.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.MemoryUtilization))
			collector.memoryUsed.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.MemoryUsed))
			collector.memoryTotal.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.MemoryTotal))
			collector.powerUsage.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.PowerUsage))
			collector.powerLimit.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.PowerLimit))
			collector.fanSpeed.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.FanSpeed))
			collector.gpuTemperature.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.TemperatureGpu))
			collector.memoryTemperature.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.TemperatureMemory))
			collector.coreClock.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.ClockCore))
			collector.memoryClock.WithLabelValues(strconv.Itoa(index), metric.Name).Set(float64(metric.ClockMemory))
		}
	}
}
