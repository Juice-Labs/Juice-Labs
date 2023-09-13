/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package prometheus

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

type Frontend struct {
	sync.Mutex

	storage storage.Storage

	agents                   prometheus.Gauge
	agentsByStatus           *prometheus.GaugeVec
	sessions                 prometheus.Gauge
	sessionsByStatus         *prometheus.GaugeVec
	gpus                     prometheus.Gauge
	gpusByGpuName            *prometheus.GaugeVec
	vram                     prometheus.Gauge
	vramByGpuName            *prometheus.GaugeVec
	vramUsed                 prometheus.Gauge
	vramUsedByGpuName        *prometheus.GaugeVec
	vramGBAvailable          *prometheus.GaugeVec
	vramGBAvailableByGpuName *prometheus.GaugeVec
	utilization              prometheus.Gauge
	utilizationByGpuName     *prometheus.GaugeVec
	powerDraw                prometheus.Gauge
	powerDrawByGpuName       *prometheus.GaugeVec
}

func getGaugeOpts(name string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "Juice",
		Subsystem: "Controller",
		Name:      name,
	}
}

func NewFrontend(server *server.Server, storage storage.Storage) *Frontend {
	frontend := &Frontend{
		storage: storage,

		agents:                   prometheus.NewGauge(getGaugeOpts("agents")),
		agentsByStatus:           prometheus.NewGaugeVec(getGaugeOpts("agentsByStatus"), []string{"status"}),
		sessions:                 prometheus.NewGauge(getGaugeOpts("sessions")),
		sessionsByStatus:         prometheus.NewGaugeVec(getGaugeOpts("sessionsByStatus"), []string{"status"}),
		gpus:                     prometheus.NewGauge(getGaugeOpts("gpus")),
		gpusByGpuName:            prometheus.NewGaugeVec(getGaugeOpts("gpusByGpuName"), []string{"gpu"}),
		vram:                     prometheus.NewGauge(getGaugeOpts("vram")),
		vramByGpuName:            prometheus.NewGaugeVec(getGaugeOpts("vramByGpuName"), []string{"gpu"}),
		vramUsed:                 prometheus.NewGauge(getGaugeOpts("vramUsed")),
		vramUsedByGpuName:        prometheus.NewGaugeVec(getGaugeOpts("vramUsedByGpuName"), []string{"gpu"}),
		vramGBAvailable:          prometheus.NewGaugeVec(getGaugeOpts("vramGBAvailable"), []string{"percentile"}),
		vramGBAvailableByGpuName: prometheus.NewGaugeVec(getGaugeOpts("vramGBAvailableByGpuName"), []string{"gpu", "percentile"}),
		utilization:              prometheus.NewGauge(getGaugeOpts("utilization")),
		utilizationByGpuName:     prometheus.NewGaugeVec(getGaugeOpts("utilizationByGpuName"), []string{"gpu"}),
		powerDraw:                prometheus.NewGauge(getGaugeOpts("powerDrawWatts")),
		powerDrawByGpuName:       prometheus.NewGaugeVec(getGaugeOpts("powerDrawWattsByGpuName"), []string{"gpu"}),
	}
	prometheus.MustRegister(frontend)

	server.AddEndpointHandler("GET", "/metrics", promhttp.Handler(), true)

	return frontend
}

func (frontend *Frontend) Run(group task.Group) error {
	err := frontend.update()
	if err == nil {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for err == nil {
			select {
			case <-group.Ctx().Done():
				return err

			case <-ticker.C:
				err = frontend.update()
			}
		}
	}

	return err
}

func (c *Frontend) update() error {
	data, err := c.storage.AggregateData()
	if err != nil {
		return err
	}

	c.Lock()
	defer c.Unlock()

	c.agents.Set(float64(data.Agents))
	c.agentsByStatus.Reset()
	for key, value := range data.AgentsByStatus {
		c.agentsByStatus.WithLabelValues(key).Set(float64(value))
	}

	c.sessions.Set(float64(data.Sessions))
	c.sessionsByStatus.Reset()
	for key, value := range data.SessionsByStatus {
		c.sessionsByStatus.WithLabelValues(key).Set(float64(value))
	}

	c.gpus.Set(float64(data.Gpus))
	c.gpusByGpuName.Reset()
	for key, value := range data.GpusByGpuName {
		c.gpusByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.vram.Set(float64(data.Vram))
	c.vramByGpuName.Reset()
	for key, value := range data.VramByGpuName {
		c.vramByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.vramUsed.Set(float64(data.VramUsed))
	c.vramUsedByGpuName.Reset()
	for key, value := range data.VramUsedByGpuName {
		c.vramUsedByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.vramGBAvailable.WithLabelValues("100").Set(float64(data.VramGBAvailable.P100))
	c.vramGBAvailable.WithLabelValues("90").Set(float64(data.VramGBAvailable.P90))
	c.vramGBAvailable.WithLabelValues("75").Set(float64(data.VramGBAvailable.P75))
	c.vramGBAvailable.WithLabelValues("50").Set(float64(data.VramGBAvailable.P50))
	c.vramGBAvailable.WithLabelValues("25").Set(float64(data.VramGBAvailable.P25))
	c.vramGBAvailable.WithLabelValues("10").Set(float64(data.VramGBAvailable.P10))
	for key, value := range data.VramGBAvailableByGpuName {
		c.vramGBAvailableByGpuName.WithLabelValues(key, "100").Set(float64(value.P100))
		c.vramGBAvailableByGpuName.WithLabelValues(key, "90").Set(float64(value.P90))
		c.vramGBAvailableByGpuName.WithLabelValues(key, "75").Set(float64(value.P75))
		c.vramGBAvailableByGpuName.WithLabelValues(key, "50").Set(float64(value.P50))
		c.vramGBAvailableByGpuName.WithLabelValues(key, "25").Set(float64(value.P25))
		c.vramGBAvailableByGpuName.WithLabelValues(key, "10").Set(float64(value.P10))
	}

	c.utilization.Set(float64(data.Utilization))
	c.utilizationByGpuName.Reset()
	for key, value := range data.UtilizationByGpuName {
		c.utilizationByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.powerDraw.Set(float64(data.PowerDraw))
	c.powerDrawByGpuName.Reset()
	for key, value := range data.PowerDrawByGpuName {
		c.powerDrawByGpuName.WithLabelValues(key).Set(float64(value))
	}

	return err
}

func (c *Frontend) Describe(ch chan<- *prometheus.Desc) {
	c.agents.Describe(ch)
	c.agentsByStatus.Describe(ch)
	c.sessions.Describe(ch)
	c.sessionsByStatus.Describe(ch)
	c.gpus.Describe(ch)
	c.gpusByGpuName.Describe(ch)
	c.vram.Describe(ch)
	c.vramByGpuName.Describe(ch)
	c.vramUsed.Describe(ch)
	c.vramUsedByGpuName.Describe(ch)
	c.vramGBAvailable.Describe(ch)
	c.vramGBAvailableByGpuName.Describe(ch)
	c.utilization.Describe(ch)
	c.utilizationByGpuName.Describe(ch)
	c.powerDraw.Describe(ch)
	c.powerDrawByGpuName.Describe(ch)
}

func (c *Frontend) Collect(ch chan<- prometheus.Metric) {
	c.Lock()
	defer c.Unlock()

	c.agents.Collect(ch)
	c.agentsByStatus.Collect(ch)
	c.sessions.Collect(ch)
	c.sessionsByStatus.Collect(ch)
	c.gpus.Collect(ch)
	c.gpusByGpuName.Collect(ch)
	c.vram.Collect(ch)
	c.vramByGpuName.Collect(ch)
	c.vramUsed.Collect(ch)
	c.vramUsedByGpuName.Collect(ch)
	c.vramGBAvailable.Collect(ch)
	c.vramGBAvailableByGpuName.Collect(ch)
	c.utilization.Collect(ch)
	c.utilizationByGpuName.Collect(ch)
	c.powerDraw.Collect(ch)
	c.powerDrawByGpuName.Collect(ch)
}
