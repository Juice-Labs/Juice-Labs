/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

const (
	StateQueued int = iota
	StateAssigned
	StateActive
	StateInactive
	StateClosed
)

type Gpu struct {
	Index int `json:"index"`

	Name string `json:"name"`

	VendorId uint32 `json:"vendorId"`

	DeviceId uint32 `json:"deviceId"`

	Vram uint64 `json:"vram,omitempty"`

	PciBus string `json:"pciBus"`
}

type GpuRequirements struct {
	VendorId uint32 `json:"vendorId,omitempty"`

	DeviceId uint32 `json:"deviceId,omitempty"`

	VramRequired uint64 `json:"vramRequired,omitempty"`

	PciBus string `json:"pciBus"`
}

type RequestSession struct {
	Version string `json:"version"`

	Gpus []GpuRequirements `json:"gpus"`
}

type Session struct {
	Id string `json:"id"`

	State int `json:"state"`

	Address string `json:"address,omitempty"`

	Version string `json:"version,omitempty"`

	Gpus []Gpu `json:"gpus,omitempty"`
}

type Server struct {
	Version string `json:"version"`

	Hostname string `json:"hostname"`

	Address string `json:"address"`
}

type Agent struct {
	Server

	Id string `json:"id"`

	State int `json:"state"`

	MaxSessions int `json:"maxSessions"`

	Gpus []Gpu `json:"gpus"`

	Tags map[string]string `json:"tags,omitempty"`

	Taints map[string]string `json:"taints,omitempty"`

	Sessions []Session `json:"sessions,omitempty"`
}