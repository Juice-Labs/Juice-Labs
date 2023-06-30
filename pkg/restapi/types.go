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
	VramRequired uint64 `json:"vramRequired,omitempty"`
	PciBus       string `json:"pciBus,omitempty"`

	Tags      map[string]string `json:"tags,omitempty"`
	Tolerates map[string]string `json:"taints,omitempty"`
}

type SessionRequirements struct {
	Version string `json:"version"`

	Gpus []GpuRequirements `json:"gpus"`

	Tags      map[string]string `json:"tags,omitempty"`
	Tolerates map[string]string `json:"taints,omitempty"`
}

type Status struct {
	State    string `json:"status"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
}

type Session struct {
	Id      string `json:"id"`
	State   int    `json:"state"`
	Address string `json:"address,omitempty"`
	Version string `json:"version,omitempty"`

	Gpus []Gpu `json:"gpus,omitempty"`
}

type Agent struct {
	Id       string `json:"id"`
	State    int    `json:"state"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
	Version  string `json:"version"`

	MaxSessions int   `json:"maxSessions"`
	Gpus        []Gpu `json:"gpus"`

	Tags   map[string]string `json:"tags,omitempty"`
	Taints map[string]string `json:"taints,omitempty"`

	Sessions []Session `json:"sessions,omitempty"`
}
