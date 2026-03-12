package models

import "time"

type BountyStatus string

const (
	BountyStatusPending    BountyStatus = "PENDING"
	BountyStatusInProgress BountyStatus = "IN_PROGRESS"
	BountyStatusCompleted  BountyStatus = "COMPLETED"
	BountyStatusCanceled   BountyStatus = "CANCELED"
)

type ApplicationStatus string

const (
	AppStatusApplied  ApplicationStatus = "APPLIED"
	AppStatusAccepted ApplicationStatus = "ACCEPTED"
	AppStatusRejected ApplicationStatus = "REJECTED"
)

// Bounty 映射 bounties 表
type Bounty struct {
	ID           int64        `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployerID   int64        `gorm:"not null;index" json:"employer_id"`
	Title        string       `gorm:"type:varchar(255);not null" json:"title"`
	Description  string       `gorm:"type:text;not null" json:"description"`
	RewardAmount int64        `gorm:"not null" json:"reward_amount"`
	Status       BountyStatus `gorm:"type:varchar(50);not null;default:'PENDING'" json:"status"`
	CreatedAt    time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"autoUpdateTime" json:"updated_at"`

	// GORM 关联: 一个悬赏有多个申请 (Has Many 关系)
	Applications []BountyApplication `gorm:"foreignKey:BountyID;constraint:OnDelete:CASCADE;" json:"applications,omitempty"`
}

// BountyApplication 映射 bounty_applications 表
type BountyApplication struct {
	ID        int64             `gorm:"primaryKey;autoIncrement" json:"id"`
	BountyID  int64             `gorm:"not null;uniqueIndex:idx_unique_bounty_hunter" json:"bounty_id"`
	HunterID  int64             `gorm:"not null;uniqueIndex:idx_unique_bounty_hunter;index" json:"hunter_id"`
	Status    ApplicationStatus `gorm:"type:varchar(50);not null;default:'APPLIED'" json:"status"`
	CreatedAt time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}
