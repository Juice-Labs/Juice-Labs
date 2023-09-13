/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package restapi

const (
	SessionClosed    = "closed"
	SessionQueued    = "queued"
	SessionAssigned  = "assigned"
	SessionActive    = "active"
	SessionCanceling = "canceling"
)

const (
	AgentClosed   = "closed"
	AgentActive   = "active"
	AgentDisabled = "disabled"
	AgentMissing  = "missing"
)

type Permission string

const (
	PermissionCreateSession Permission = "create_session"
	PermissionRegisterAgent Permission = "register_agent"
	PermissionAdmin         Permission = "admin"
)

type GpuRequirements struct {
	VramRequired uint64 `json:"vramRequired"`
	PciBus       string `json:"pciBus"`
}

type SessionRequirements struct {
	Version string `json:"version"`
	PoolId  string `json:"poolId"`

	Gpus []GpuRequirements `json:"gpus"`

	MatchLabels map[string]string `json:"matchLabels"`
	Tolerates   map[string]string `json:"tolerates"`
}

type SessionGpu struct {
	Index int `json:"index"`

	VramRequired uint64 `json:"vramRequired"`
}

type Session struct {
	Id      string `json:"id"`
	State   string `json:"state"`
	Address string `json:"address"`
	Version string `json:"version"`
	PoolId  string `json:"poolId"`

	Gpus        []SessionGpu `json:"gpus"`
	Connections []Connection `json:"connections"`
}

type ConnectionData struct {
	Id          string `json:"id"`
	Pid         string `json:"pid"`
	ProcessName string `json:"processName"`
}

type Connection struct {
	ConnectionData

	ExitCode int `json:"exitCode"`
}

type GpuMetrics struct {
	ClockCore       uint32 `json:"clockCore"`
	ClockMemory     uint32 `json:"clockMemory"`
	UtilizationGpu  uint32 `json:"utilizationGpu"`
	UtilizationVram uint32 `json:"utilizationVram"`
	TemperatureGpu  uint32 `json:"temperatureGpu"`
	VramUsed        uint64 `json:"vramUsed"`
	PowerDraw       uint32 `json:"powerDraw"`
	PowerLimit      uint32 `json:"powerLimit"`
	FanSpeed        uint32 `json:"fanSpeed"`
}

type Gpu struct {
	Index       int    `json:"index"`
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

type Agent struct {
	Id       string `json:"id"`
	State    string `json:"state"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`
	Version  string `json:"version"`
	PoolId   string `json:"poolId"`

	Gpus []Gpu `json:"gpus"`

	Labels map[string]string `json:"labels"`
	Taints map[string]string `json:"taints"`

	Sessions []Session `json:"sessions"`
}

type Status struct {
	State    string `json:"state"`
	Version  string `json:"version"`
	Hostname string `json:"hostname"`
}

type SessionUpdate struct {
	State       string                `json:"State"`
	Connections map[string]Connection `json:"connections"`
}

type AgentUpdate struct {
	Id             string                   `json:"id"`
	State          string                   `json:"state"`
	SessionsUpdate map[string]SessionUpdate `json:"sessions"`
	Gpus           []GpuMetrics             `json:"gpus"`
}

type WebhookMessage struct {
	Agent   string `json:"agent"`
	Session string `json:"session"`
	State   string `json:"state"`
}

type CreatePoolParams struct {
	Name string `json:"name"`
}

type PermissionParams struct {
	Permission Permission `json:"permission"`
	UserId     string     `json:"userId"`
	PoolId     string     `json:"poolId"`
}

type Pool struct {
	Id           string `json:"id"`
	Name         string `json:"name"`
	SessionCount int    `json:"sessionCount"`
	AgentCount   int    `json:"agentCount"`
	UserCount    int    `json:"userCount"`
}

type UserPermissions struct {
	Permissions map[Permission][]Pool `json:"permissions"`
}

type PoolPermissions struct {
	UserIds map[string][]Permission `json:"userIds"`
}
