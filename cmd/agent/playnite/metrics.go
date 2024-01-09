/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package playnite

import (
	"encoding/binary"
	"encoding/json"
	"net"

	"github.com/Xdevlab/Run/cmd/agent/app"
	"github.com/Xdevlab/Run/cmd/agent/gpu"
	"github.com/Xdevlab/Run/pkg/logger"
	"github.com/Xdevlab/Run/pkg/restapi"
	"github.com/Xdevlab/Run/pkg/utilities"
)

type GpuData struct {
	Name              string `json:"name"`
	GpuUtilization    int    `json:"gpuUtilization"`
	MemoryUtilization int    `json:"memoryUtilization"`
	MemoryTotal       int64  `json:"memoryTotal"`
	MemoryUsed        int64  `json:"memoryUsed"`
	PowerUsage        int    `json:"powerUsage"`
	PowerLimit        int    `json:"powerLimit"`
	FanSpeed          int    `json:"fanSpeed"`
	TemperatureGpu    int    `json:"temperatureGpu"`
	TemperatureMemory int    `json:"temperatureMemory"`
	ClockCore         int    `json:"clockCore"`
	ClockMemory       int    `json:"clockMemory"`
}

type GpuUpdate struct {
	Hostname string    `json:"hostname"`
	Port     int       `json:"port"`
	Uuid     string    `json:"uuid"`
	Action   string    `json:"action"`
	Nonce    int       `json:"nonce"`
	GpuCount int       `json:"gpu_count"`
	Data     []GpuData `json:"data"`
}

func NewGpuMetricsConsumer(agent *app.Agent) (gpu.MetricsConsumerFn, error) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	broadcastAddresses := make([]net.Addr, 0)
	for _, netInterface := range netInterfaces {
		if netInterface.Flags&net.FlagRunning != 0 && netInterface.Flags&net.FlagBroadcast != 0 {
			addrs, err := netInterface.Addrs()
			if err != nil {
				return nil, err
			} else {
				for _, addrAny := range addrs {
					addr, err := utilities.Cast[*net.IPNet](addrAny)
					if err != nil {
						logger.Warning(err)
					} else if ipv4 := addr.IP.To4(); ipv4 != nil {
						broadcastIp := ipv4.Mask(addr.Mask)
						binary.BigEndian.PutUint32(broadcastIp, binary.BigEndian.Uint32(broadcastIp)|^binary.BigEndian.Uint32(net.IP(addr.Mask).To4()))

						broadcastAddresses = append(broadcastAddresses, &net.UDPAddr{
							IP:   broadcastIp,
							Port: 43210,
						})
					}
				}
			}
		}
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}

	udpConn, err := utilities.Cast[*net.UDPConn](conn)
	if err != nil {
		return nil, err
	}

	nonce := 0
	return func(metrics []restapi.Gpu) {
		gpuUpdate := GpuUpdate{
			Hostname: agent.Hostname,
			Port:     agent.Server.Port(),
			Uuid:     agent.Id,
			Action:   "UPDATE",
			Nonce:    nonce,
			GpuCount: agent.Gpus.Count(),
			Data:     make([]GpuData, agent.Gpus.Count()),
		}

		for index, gpu := range metrics {
			gpuUpdate.Data[index] = GpuData{
				Name:              gpu.Name,
				GpuUtilization:    int(gpu.Metrics.UtilizationGpu),
				MemoryUtilization: int(gpu.Metrics.UtilizationVram),
				MemoryTotal:       int64(gpu.Vram),
				MemoryUsed:        int64(gpu.Metrics.VramUsed),
				PowerUsage:        int(float32(gpu.Metrics.PowerDraw) / 1000.0),
				PowerLimit:        int(float32(gpu.Metrics.PowerLimit) / 1000.0),
				FanSpeed:          int(gpu.Metrics.FanSpeed),
				TemperatureGpu:    int(gpu.Metrics.TemperatureGpu),
				ClockCore:         int(gpu.Metrics.ClockCore),
				ClockMemory:       int(gpu.Metrics.ClockMemory),
			}
		}

		bytes, err := json.Marshal(gpuUpdate)
		if err == nil {
			for _, addr := range broadcastAddresses {
				udpConn.WriteTo([]byte(bytes), addr)
			}
		} else {
			logger.Warning(err)
		}

		nonce++
	}, nil
}
