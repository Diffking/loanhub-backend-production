package models

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================
// Phase Queue: Queue System Tables
// ============================================================

type Branch struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Code         string         `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	BranchType   string         `gorm:"size:10;default:'OFFICE'" json:"branch_type"`
	Address      *string        `gorm:"size:255" json:"address"`
	Latitude     *float64       `gorm:"type:decimal(10,7)" json:"latitude"`
	Longitude    *float64       `gorm:"type:decimal(10,7)" json:"longitude"`
	Phone        *string        `gorm:"size:20" json:"phone"`
	OpenTime     *string        `gorm:"size:10;default:'08:30'" json:"open_time"`
	CloseTime    *string        `gorm:"size:10;default:'16:30'" json:"close_time"`
	ScheduleNote *string        `gorm:"size:255" json:"schedule_note"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Branch) TableName() string {
	return "branches"
}

type ServiceType struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Code         string    `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Description  string    `gorm:"type:text" json:"description"`
	Icon         string    `gorm:"size:50" json:"icon"`
	Color        string    `gorm:"size:20" json:"color"`
	DisplayOrder int       `gorm:"default:0" json:"display_order"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ServiceType) TableName() string {
	return "service_types"
}

type ServiceCounter struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	BranchID      uint        `gorm:"not null;index" json:"branch_id"`
	ServiceTypeID uint        `gorm:"not null;index" json:"service_type_id"`
	CounterNumber int         `gorm:"not null" json:"counter_number"`
	CounterName   string      `gorm:"size:50" json:"counter_name"`
	StaffUserID   *uint       `gorm:"index" json:"staff_user_id"`
	Status        string      `gorm:"size:10;default:'CLOSED'" json:"status"`
	IsActive      bool        `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
	Branch        Branch      `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
	ServiceType   ServiceType `gorm:"foreignKey:ServiceTypeID" json:"service_type,omitempty"`
	StaffUser     *User       `gorm:"foreignKey:StaffUserID" json:"staff_user,omitempty"`
}

func (ServiceCounter) TableName() string {
	return "service_counters"
}

type QueueTicket struct {
	ID            uint            `gorm:"primaryKey" json:"id"`
	TicketNumber  string          `gorm:"size:20;not null" json:"ticket_number"`
	QueueType     string          `gorm:"size:10;not null" json:"queue_type"`
	QueueDate     time.Time       `gorm:"type:date;not null;index" json:"queue_date"`
	BranchID      uint            `gorm:"not null;index" json:"branch_id"`
	ServiceTypeID uint            `gorm:"not null;index" json:"service_type_id"`
	CounterID     *uint           `gorm:"index" json:"counter_id"`
	UserID        uint            `gorm:"not null;index" json:"user_id"`
	Status        string          `gorm:"size:15;default:'WAITING';index" json:"status"`
	BookingDate   *time.Time      `gorm:"type:date" json:"booking_date"`
	BookingSlot   string          `gorm:"size:10" json:"booking_slot"`
	BookingNote   string          `gorm:"size:255" json:"booking_note"`
	IssuedAt      time.Time       `gorm:"not null" json:"issued_at"`
	CalledAt      *time.Time      `json:"called_at"`
	ServingAt     *time.Time      `json:"serving_at"`
	CompletedAt   *time.Time      `json:"completed_at"`
	EstimatedAt   *time.Time      `json:"estimated_at"`
	CalledBy      *uint           `json:"called_by"`
	ServedBy      *uint           `json:"served_by"`
	Priority      int             `gorm:"default:0" json:"priority"`
	SkipCount     int             `gorm:"default:0" json:"skip_count"`
	NotifySent    bool            `gorm:"default:false" json:"notify_sent"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
	Branch        Branch          `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
	ServiceType   ServiceType     `gorm:"foreignKey:ServiceTypeID" json:"service_type,omitempty"`
	Counter       *ServiceCounter `gorm:"foreignKey:CounterID" json:"counter,omitempty"`
	User          User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CalledByUser  *User           `gorm:"foreignKey:CalledBy" json:"called_by_user,omitempty"`
	ServedByUser  *User           `gorm:"foreignKey:ServedBy" json:"served_by_user,omitempty"`
}

func (QueueTicket) TableName() string {
	return "queue_tickets"
}

type BookingSlot struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	BranchID        uint        `gorm:"not null;index" json:"branch_id"`
	ServiceTypeID   uint        `gorm:"not null;index" json:"service_type_id"`
	SlotDate        time.Time   `gorm:"type:date;not null" json:"slot_date"`
	SlotTime        string      `gorm:"size:10;not null" json:"slot_time"`
	MaxBookings     int         `gorm:"default:5" json:"max_bookings"`
	CurrentBookings int         `gorm:"default:0" json:"current_bookings"`
	IsAvailable     bool        `gorm:"default:true" json:"is_available"`
	Branch          Branch      `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
	ServiceType     ServiceType `gorm:"foreignKey:ServiceTypeID" json:"service_type,omitempty"`
}

func (BookingSlot) TableName() string {
	return "booking_slots"
}

type QueueConfig struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	BranchID    uint   `gorm:"not null;index" json:"branch_id"`
	ConfigKey   string `gorm:"size:50;not null" json:"config_key"`
	ConfigValue string `gorm:"size:255;not null" json:"config_value"`
	Description string `gorm:"size:255" json:"description"`
	Branch      Branch `gorm:"foreignKey:BranchID" json:"branch,omitempty"`
}

func (QueueConfig) TableName() string {
	return "queue_config"
}
