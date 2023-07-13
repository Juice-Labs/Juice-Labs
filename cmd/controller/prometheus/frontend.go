/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package prometheus

import (
	"crypto/tls"
	"flag"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Juice-Labs/Juice-Labs/cmd/controller/storage"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/server"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
)

var (
	address = flag.String("prometheus-address", "0.0.0.0:9090", "The IP address and port to use for listening for Prometheus connections")
)

type Frontend struct {
	sync.Mutex

	server  *server.Server
	storage storage.Storage

	agents               prometheus.Gauge
	agentsByStatus       *prometheus.GaugeVec
	sessions             prometheus.Gauge
	sessionsByStatus     *prometheus.GaugeVec
	gpus                 prometheus.Gauge
	gpusByGpuName        *prometheus.GaugeVec
	vram                 prometheus.Gauge
	vramByGpuName        *prometheus.GaugeVec
	vramUsed             prometheus.Gauge
	vramUsedByGpuName    *prometheus.GaugeVec
	utilization          prometheus.Gauge
	utilizationByGpuName *prometheus.GaugeVec
	powerDraw            prometheus.Gauge
	powerDrawByGpuName   *prometheus.GaugeVec
}

func getGaugeOpts(name string) prometheus.GaugeOpts {
	return prometheus.GaugeOpts{
		Namespace: "Juice",
		Subsystem: "Controller",
		Name:      name,
	}
}

func NewFrontend(tlsConfig *tls.Config, storage storage.Storage) (*Frontend, error) {
	if tlsConfig == nil {
		logger.Warning("TLS is disabled, data will be unencrypted")
	}

	server, err := server.NewServer(*address, tlsConfig)
	if err != nil {
		return nil, err
	}

	frontend := &Frontend{
		server:  server,
		storage: storage,

		agents:               prometheus.NewGauge(getGaugeOpts("agents")),
		agentsByStatus:       prometheus.NewGaugeVec(getGaugeOpts("agentsByStatus"), []string{"status"}),
		sessions:             prometheus.NewGauge(getGaugeOpts("sessions")),
		sessionsByStatus:     prometheus.NewGaugeVec(getGaugeOpts("sessionsByStatus"), []string{"status"}),
		gpus:                 prometheus.NewGauge(getGaugeOpts("gpus")),
		gpusByGpuName:        prometheus.NewGaugeVec(getGaugeOpts("gpusByGpuName"), []string{"gpu"}),
		vram:                 prometheus.NewGauge(getGaugeOpts("vram")),
		vramByGpuName:        prometheus.NewGaugeVec(getGaugeOpts("vramByGpuName"), []string{"gpu"}),
		vramUsed:             prometheus.NewGauge(getGaugeOpts("vramUsed")),
		vramUsedByGpuName:    prometheus.NewGaugeVec(getGaugeOpts("vramUsedByGpuName"), []string{"gpu"}),
		utilization:          prometheus.NewGauge(getGaugeOpts("utilization")),
		utilizationByGpuName: prometheus.NewGaugeVec(getGaugeOpts("utilizationByGpuName"), []string{"gpu"}),
		powerDraw:            prometheus.NewGauge(getGaugeOpts("powerDrawWatts")),
		powerDrawByGpuName:   prometheus.NewGaugeVec(getGaugeOpts("powerDrawWattsByGpuName"), []string{"gpu"}),
	}
	prometheus.MustRegister(frontend)

	server.AddCreateEndpoint(func(group task.Group, router *mux.Router) error {
		router.Methods("GET").Path("/metrics").Handler(
			promhttp.Handler())

		return nil
	})

	return frontend, nil
}

func (frontend *Frontend) Run(group task.Group) error {
	group.Go("Frontend Server", frontend.server)

	err := frontend.update()
	if err == nil {
		ticker := time.NewTicker(5 * time.Second)
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
	for key, value := range data.AgentsByStatus {
		switch key {
		case restapi.AgentActive:
			c.agentsByStatus.WithLabelValues("Active").Set(float64(value))
		case restapi.AgentDisabled:
			c.agentsByStatus.WithLabelValues("Disabled").Set(float64(value))
		case restapi.AgentMissing:
			c.agentsByStatus.WithLabelValues("Missing").Set(float64(value))
		case restapi.AgentClosed:
			c.agentsByStatus.WithLabelValues("Closed").Set(float64(value))
		}
	}

	c.sessions.Set(float64(data.Sessions))
	for key, value := range data.SessionsByStatus {
		switch key {
		case restapi.SessionQueued:
			c.sessionsByStatus.WithLabelValues("Queued").Set(float64(value))
		case restapi.SessionAssigned:
			c.sessionsByStatus.WithLabelValues("Assigned").Set(float64(value))
		case restapi.SessionActive:
			c.sessionsByStatus.WithLabelValues("Active").Set(float64(value))
		case restapi.SessionClosed:
			c.sessionsByStatus.WithLabelValues("Closed").Set(float64(value))
		case restapi.SessionFailed:
			c.sessionsByStatus.WithLabelValues("Failed").Set(float64(value))
		case restapi.SessionCanceling:
			c.sessionsByStatus.WithLabelValues("Cancelling").Set(float64(value))
		case restapi.SessionCanceled:
			c.sessionsByStatus.WithLabelValues("Canceled").Set(float64(value))
		}
	}

	c.gpus.Set(float64(data.Gpus))
	for key, value := range data.GpusByGpuName {
		c.gpusByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.vram.Set(float64(data.Vram))
	for key, value := range data.VramByGpuName {
		c.vramByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.vramUsed.Set(float64(data.VramUsed))
	for key, value := range data.VramUsedByGpuName {
		c.vramUsedByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.utilization.Set(float64(data.Utilization))
	for key, value := range data.UtilizationByGpuName {
		c.utilizationByGpuName.WithLabelValues(key).Set(float64(value))
	}

	c.powerDraw.Set(float64(data.PowerDraw))
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
	c.utilization.Collect(ch)
	c.utilizationByGpuName.Collect(ch)
	c.powerDraw.Collect(ch)
	c.powerDrawByGpuName.Collect(ch)
}
