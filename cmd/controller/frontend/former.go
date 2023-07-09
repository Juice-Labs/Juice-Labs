/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package frontend

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Juice-Labs/Juice-Labs/cmd/internal/build"
	"github.com/Juice-Labs/Juice-Labs/pkg/logger"
	pkgnet "github.com/Juice-Labs/Juice-Labs/pkg/net"
	"github.com/Juice-Labs/Juice-Labs/pkg/restapi"
	"github.com/Juice-Labs/Juice-Labs/pkg/task"
	"github.com/gorilla/mux"
)

// Pulled from former Node version
type GpuData struct {
	Vendor         string  `json:"vendor"`
	Model          string  `json:"model"`
	Bus            string  `json:"bus"`
	BusAddress     string  `json:"busAddress"`
	Vram           uint64  `json:"vram"`
	VramDynamic    bool    `json:"vramDynamic"`
	DriverVersion  string  `json:"driverVersion"`
	SubDeviceId    string  `json:"subDeviceId"`
	Name           string  `json:"name"`
	PciBus         string  `json:"pciBus"`
	MemoryTotal    uint64  `json:"memoryTotal"`
	MemoryUsed     uint64  `json:"memoryUsed"`
	MemoryFree     uint64  `json:"memoryFree"`
	TemperatureGpu uint32  `json:"temperatureGpu"`
	PowerDraw      float32 `json:"powerDraw"`
	PowerLimit     float32 `json:"powerLimit"`
	ClockCore      uint32  `json:"clockCore"`
	ClockMemory    uint32  `json:"clockMemory"`
	Uuid           string  `json:"uuid"`
	Ordinal        string  `json:"ordinal"`
}

type AgentData struct {
	Hostname string    `json:"hostname"`
	Port     int       `json:"port"`
	Id       string    `json:"uuid"`
	Action   string    `json:"action"`
	Nonce    int       `json:"nonce"`
	GpuCount int       `json:"gpu_count"`
	Gpus     []GpuData `json:"data"`
	Ip       string    `json:"ip"`
}

type StatusFormer struct {
	Status   string      `json:"status"`
	Version  string      `json:"version"`
	UptimeMs int64       `json:"uptime_ms"`
	Hosts    []AgentData `json:"hosts"`
}

func (frontend *Frontend) GetActiveAgents() ([]restapi.Agent, error) {
	iterator, err := frontend.storage.GetAvailableAgentsMatching(0, map[string]string{}, map[string]string{})
	if err != nil {
		return nil, err
	}

	agents := make([]restapi.Agent, 0)
	for iterator.Next() {
		agents = append(agents, iterator.Value())
	}

	return agents, nil
}

func (frontend *Frontend) getStatusFormer(group task.Group, router *mux.Router) error {
	router.Methods("GET").Path("/status").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			agents, err := frontend.GetActiveAgents()
			if err == nil {
				hosts := make([]AgentData, len(agents))
				for index, agent := range agents {
					ip, portStr, err_ := net.SplitHostPort(agent.Address)
					if err_ != nil {
						err = err_
						break
					}

					port, err_ := strconv.Atoi(portStr)
					if err_ != nil {
						err = err_
						break
					}

					gpus := make([]GpuData, len(agent.Gpus))
					for index, gpu := range agent.Gpus {
						gpus[index] = GpuData{
							Vendor:         gpu.Vendor,
							Model:          gpu.Model,
							Bus:            "OnBoard",
							BusAddress:     gpu.PciBus,
							Vram:           gpu.Vram,
							VramDynamic:    false,
							DriverVersion:  gpu.Driver,
							SubDeviceId:    fmt.Sprint("0x", strconv.FormatInt(int64(gpu.SubDeviceId), 16)),
							Name:           gpu.Name,
							PciBus:         gpu.PciBus,
							MemoryTotal:    gpu.Vram,
							MemoryUsed:     gpu.Metrics.VramUsed,
							MemoryFree:     gpu.Vram - gpu.Metrics.VramUsed,
							TemperatureGpu: gpu.Metrics.TemperatureGpu,
							PowerDraw:      float32(gpu.Metrics.PowerDraw) / 1000.0,
							PowerLimit:     float32(gpu.Metrics.PowerLimit) / 1000.0,
							ClockCore:      gpu.Metrics.ClockCore,
							ClockMemory:    gpu.Metrics.ClockMemory,
							Uuid:           gpu.Uuid,
							Ordinal:        strconv.FormatInt(int64(gpu.Ordinal), 10),
						}
					}

					hosts[index] = AgentData{
						Hostname: agent.Hostname,
						Port:     port,
						Id:       agent.Id,
						Action:   "UPDATE",
						Nonce:    0,
						GpuCount: len(agent.Gpus),
						Gpus:     gpus,
						Ip:       ip,
					}
				}

				if err == nil {
					err = pkgnet.Respond(w, http.StatusOK, StatusFormer{
						Status:   "ok",
						Version:  build.Version,
						UptimeMs: time.Since(frontend.startTime).Milliseconds(),
						Hosts:    hosts,
					})
				}
			}

			if err != nil {
				err = errors.Join(err, pkgnet.RespondWithString(w, http.StatusInternalServerError, err.Error()))
				logger.Error(err)
			}
		})
	return nil
}
