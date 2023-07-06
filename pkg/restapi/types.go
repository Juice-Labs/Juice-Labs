/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

import "time"

const (
	StateQueued int = iota
	StateAssigned
	StateActive
	StateInactive
	StateClosed
)

type GpuMetrics struct {
	Time            time.Time `json:"time"`
	ClockCore       uint32    `json:"clockCore"`
	ClockMemory     uint32    `json:"clockMemory"`
	UtilizationGpu  uint32    `json:"utilizationGpu"`
	UtilizationVram uint32    `json:"utilizationVram"`
	TemperatureGpu  uint32    `json:"temperatureGpu"`
	VramUsed        uint64    `json:"vramUsed"`
	PowerDraw       uint32    `json:"powerDraw"`
	PowerLimit      uint32    `json:"powerLimit"`
	FanSpeed        uint32    `json:"fanSpeed"`
}

type Gpu struct {
	Index       int    `json:"index"`
	Ordinal     int    `json:"ordinal"`
	Uuid        string `json:"uuid"`
	Name        string `json:"name"`
	Vendor      string `json:"vendor"`
	Model       string `json:"model"`
	VendorId    uint32 `json:"vendorId"`
	DeviceId    uint32 `json:"deviceId"`
	SubDeviceId uint32 `json:"subDeviceId"`
	Driver      string `json:"driver"`
	Vram        uint64 `json:"vram"`
	PciBus      string `json:"pciBus"`

	Metrics GpuMetrics `json:"metrics"`
}

type GpuRequirements struct {
	VramRequired uint64 `json:"vramRequired"`
	PciBus       string `json:"pciBus"`

	Tags      map[string]string `json:"tags"`
	Tolerates map[string]string `json:"taints"`
}

type SessionRequirements struct {
	Version    string `json:"version"`
	Persistent bool   `json:"persistent"`

	Gpus []GpuRequirements `json:"gpus"`

	Tags      map[string]string `json:"tags"`
	Tolerates map[string]string `json:"taints"`
}

type Status struct {
	State    string `json:"status"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
}

type SessionGpu struct {
	Gpu

	VramRequired uint64 `json:"vramRequired"`
}

type Session struct {
	Id         string `json:"id"`
	State      int    `json:"state"`
	Address    string `json:"address"`
	Version    string `json:"version"`
	Persistent bool   `json:"persistent"`

	Gpus []SessionGpu `json:"gpus"`
}

type Agent struct {
	Id       string `json:"id"`
	State    int    `json:"state"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
	Version  string `json:"version"`

	MaxSessions int   `json:"maxSessions"`
	Gpus        []Gpu `json:"gpus"`

	Tags   map[string]string `json:"tags"`
	Taints map[string]string `json:"taints"`

	Sessions []Session `json:"sessions"`
}
