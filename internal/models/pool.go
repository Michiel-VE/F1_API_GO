package models

import (
	"time"

	"github.com/google/uuid"
)

type Pool struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Name       string    `gorm:"column:name;not null" json:"name"`
	InviteCode string    `gorm:"column:invite_code;not null;unique" json:"invite_code"`
	CreatorID  uuid.UUID `gorm:"column:creator_id;type:uuid;not null" json:"creator_id"`
	Creator    *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at;type:timestamp without time zone;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;type:timestamp without time zone;autoUpdateTime" json:"updated_at"`
	Members    []*User   `gorm:"many2many:f1_api.pool_members;joinForeignKey:pool_id;joinReferences:user_id" json:"members,omitempty"`
}

func (Pool) TableName() string {
	return "f1_api.pools"
}

func (p *Pool) IsMember(userID uuid.UUID) bool {
	for _, m := range p.Members {
		if m != nil && m.ID == userID {
			return true
		}
	}
	return false
}

func (p *Pool) RemoveMember(userID uuid.UUID) bool {
	for i, m := range p.Members {
		if m != nil && m.ID == userID {
			p.Members = append(p.Members[:i], p.Members[i+1:]...)
			return true
		}
	}
	return false
}
