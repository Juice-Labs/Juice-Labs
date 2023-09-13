package models

import (
	"strings"
	"time"

	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type PermissionType int

const (
	CreateSession PermissionType = iota
	RegisterAgent
	Admin
)

func (p PermissionType) String() string {
	switch p {
	case CreateSession:
		return "create_session"
	case RegisterAgent:
		return "register_agent"
	case Admin:
		return "admin"
	default:
		return "unknown"
	}
}

var permissionStateMappings = map[string]PermissionType{
	"create_session": CreateSession,
	"register_agent": RegisterAgent,
	"admin":          Admin,
}

func PermissionTypeFromString(value string) PermissionType {
	status, ok := permissionStateMappings[strings.ToLower(value)]
	if !ok {
		return PermissionType(-1)
	}
	return status
}

type Permission struct {
	ID         uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	UserID     string         `gorm:"type:text;not null;"`
	PoolID     uuid.UUID      `gorm:"type:uuid;not null;"`
	Pool       Pool           `gorm:"constraint:OnDelete:CASCADE;"`
	Permission PermissionType `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
