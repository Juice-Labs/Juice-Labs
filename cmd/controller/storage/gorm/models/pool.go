package models

import (
	"time"

	uuid "github.com/satori/go.uuid"
	"gorm.io/gorm"
)

type Pool struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	PoolName  string    `gorm:"type:varchar(255);not null"`
	MaxAgents int       `gorm:"default:0"`

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Permissions []Permission
	Sessions    []Session
	Agents      []Agent
}
