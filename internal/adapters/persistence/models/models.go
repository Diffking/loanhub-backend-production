package models

import (
	"time"

	"gorm.io/gorm"
)

// ============================================================
// Phase 2-3: Auth & User Tables
// ============================================================

// User represents users table
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	MembNo    string         `gorm:"uniqueIndex;size:20;not null" json:"memb_no"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	FullName  string         `gorm:"size:100" json:"full_name"`
	Email     *string        `gorm:"uniqueIndex;size:100" json:"email"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	Role      string         `gorm:"size:20;default:'USER'" json:"role"`
	DeptName  string         `gorm:"size:150" json:"dept_name"`
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

// UserResponse DTO
type UserResponse struct {
	ID        uint      `json:"id"`
	MembNo    string    `json:"memb_no"`
	Username  string    `json:"username"`
	Email     *string   `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	FullName  string    `json:"full_name,omitempty"`
	DeptName  string    `json:"dept_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) ToResponse() *UserResponse {
	return &UserResponse{
		ID:        u.ID,
		MembNo:    u.MembNo,
		Username:  u.Username,
		Email:     u.Email,
		Role:      u.Role,
		IsActive:  u.IsActive,
		FullName:  u.FullName,
		DeptName:  u.DeptName,
		CreatedAt: u.CreatedAt,
	}
}

// RefreshToken represents refresh_tokens table
type RefreshToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	TokenHash string     `gorm:"size:255;not null;index" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	RevokedAt *time.Time `gorm:"index" json:"revoked_at"`
	User      User       `gorm:"foreignKey:UserID" json:"-"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

func (rt *RefreshToken) IsRevoked() bool {
	return rt.RevokedAt != nil
}

func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// Flommast represents the legacy flommast table (Read Only!)
type Flommast struct {
	MastMembNo  string `gorm:"column:mast_memb_no;primaryKey" json:"mast_memb_no"`
	FullName    string `gorm:"column:full_name" json:"full_name"`
	DeptName    string `gorm:"column:dept_name" json:"dept_name"`
	StsTypeDesc string `gorm:"column:sts_type_desc" json:"sts_type_desc"`
}

func (Flommast) TableName() string {
	return "flommast"
}

// ============================================================
// Phase 4: Master Tables
// ============================================================

// LoanType ประเภทเงินกู้ (Master)
type LoanType struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Code         string         `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	Description  string         `gorm:"type:text" json:"description"`
	InterestRate float64        `gorm:"type:decimal(5,2);not null" json:"interest_rate"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LoanType) TableName() string {
	return "loan_types"
}

// LoanStep ขั้นตอน/สถานะ (Master)
type LoanStep struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Code        string         `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	StepOrder   int            `gorm:"not null" json:"step_order"`
	Color       string         `gorm:"size:20" json:"color"`
	IsFinal     bool           `gorm:"default:false" json:"is_final"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LoanStep) TableName() string {
	return "loan_steps"
}

// LoanDoc ประเภทเอกสาร (Master)
type LoanDoc struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Code        string         `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name        string         `gorm:"size:100;not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LoanDoc) TableName() string {
	return "loan_docs"
}

// LoanAppt ประเภทนัดหมาย (Master)
type LoanAppt struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Code            string         `gorm:"size:20;uniqueIndex;not null" json:"code"`
	Name            string         `gorm:"size:100;not null" json:"name"`
	Description     string         `gorm:"type:text" json:"description"`
	DefaultLocation string         `gorm:"size:200" json:"default_location"`
	IsActive        bool           `gorm:"default:true" json:"is_active"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (LoanAppt) TableName() string {
	return "loan_appts"
}

// ============================================================
// Phase 4: Main Tables
// ============================================================

// Mortgage ข้อมูลจำนอง (ตารางหลัก)
type Mortgage struct {
	ID              uint    `gorm:"primaryKey" json:"id"`
	ContractNo      *string `gorm:"size:50;uniqueIndex" json:"contract_no"`
	MembNo          string  `gorm:"size:20;not null;index" json:"memb_no"`
	OfficerID       uint    `gorm:"not null" json:"officer_id"`
	UserID          uint    `gorm:"not null" json:"user_id"`
	Amount          float64  `gorm:"type:decimal(15,2);not null" json:"amount"`
	ApprovedAmount  *float64 `gorm:"type:decimal(15,2)" json:"approved_amount"`
	Collateral      string  `gorm:"type:text" json:"collateral"`
	Purpose         string  `gorm:"type:text" json:"purpose"`
	GuarantorMembNo *string `gorm:"size:20" json:"guarantor_memb_no"`
	LoanTypeID      uint    `gorm:"not null" json:"loan_type_id"`
	InterestRate    float64 `gorm:"type:decimal(5,2);not null" json:"interest_rate"`
	CurrentStepID   uint    `gorm:"not null" json:"current_step_id"`

	// Appointment fields (ย้ายมาจาก loan_appt_currents)
	CurrentApptID *uint      `json:"current_appt_id"` // FK to loan_appts (master) - ประเภทนัดหมาย
	ApptDate      *time.Time `gorm:"type:date" json:"appt_date"`
	ApptTime      string     `gorm:"size:10" json:"appt_time"`
	ApptLocation  string     `gorm:"size:200" json:"appt_location"`

	// Document field (ย้ายมาจาก loan_doc_currents)
	CurrentDocID *uint `json:"current_doc_id"` // FK to loan_docs (master) - เอกสารปัจจุบันที่ต้องส่ง

	// Approval fields
	ApprovedBy *uint      `json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`
	Remark     string     `gorm:"type:text" json:"remark"`

	// Timestamps
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Officer     *User     `gorm:"foreignKey:OfficerID" json:"officer,omitempty"`
	Creator     *User     `gorm:"foreignKey:UserID" json:"creator,omitempty"`
	LoanType    *LoanType `gorm:"foreignKey:LoanTypeID" json:"loan_type,omitempty"`
	CurrentStep *LoanStep `gorm:"foreignKey:CurrentStepID" json:"current_step,omitempty"`
	CurrentAppt *LoanAppt `gorm:"foreignKey:CurrentApptID" json:"current_appt,omitempty"` // ประเภทนัดหมาย
	CurrentDoc  *LoanDoc  `gorm:"foreignKey:CurrentDocID" json:"current_doc,omitempty"`   // ประเภทเอกสาร
	Approver    *User     `gorm:"foreignKey:ApprovedBy" json:"approver,omitempty"`
}

func (Mortgage) TableName() string {
	return "mortgages"
}

// MortgageResponse DTO
type MortgageResponse struct {
	ID              uint    `json:"id"`
	ContractNo      *string `json:"contract_no"`
	MembNo          string  `json:"memb_no"`
	MemberName      string  `json:"member_name,omitempty"`
	OfficerID       uint    `json:"officer_id"`
	OfficerName     string  `json:"officer_name,omitempty"`
	Amount          float64  `json:"amount"`
	ApprovedAmount  *float64 `json:"approved_amount"`
	Collateral      string  `json:"collateral"`
	Purpose         string  `json:"purpose"`
	GuarantorMembNo *string `json:"guarantor_memb_no"`
	LoanTypeID      uint    `json:"loan_type_id"`
	LoanTypeName    string  `json:"loan_type_name,omitempty"`
	InterestRate    float64 `json:"interest_rate"`
	CurrentStepID   uint    `json:"current_step_id"`
	CurrentStepCode string  `json:"current_step_code,omitempty"`
	CurrentStepName string  `json:"current_step_name,omitempty"`

	// Appointment info
	CurrentApptID   *uint     `json:"current_appt_id"`
	CurrentApptName string    `json:"current_appt_name,omitempty"`
	CurrentAppt     *LoanAppt `json:"current_appt,omitempty"`
	ApptDate        string    `json:"appt_date,omitempty"`
	ApptTime        string    `json:"appt_time,omitempty"`
	ApptLocation    string    `json:"appt_location,omitempty"`

	// Document info
	CurrentDocID   *uint    `json:"current_doc_id"`
	CurrentDocName string   `json:"current_doc_name,omitempty"`
	CurrentDoc     *LoanDoc `json:"current_doc,omitempty"`

	// Approval info
	ApprovedBy *uint      `json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`
	Remark     string     `json:"remark"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func (m *Mortgage) ToResponse() *MortgageResponse {
	resp := &MortgageResponse{
		ID:              m.ID,
		ContractNo:      m.ContractNo,
		MembNo:          m.MembNo,
		OfficerID:       m.OfficerID,
		Amount:          m.Amount,
		ApprovedAmount:  m.ApprovedAmount,
		Collateral:      m.Collateral,
		Purpose:         m.Purpose,
		GuarantorMembNo: m.GuarantorMembNo,
		LoanTypeID:      m.LoanTypeID,
		InterestRate:    m.InterestRate,
		CurrentStepID:   m.CurrentStepID,
		CurrentApptID:   m.CurrentApptID,
		ApptTime:        m.ApptTime,
		ApptLocation:    m.ApptLocation,
		CurrentDocID:    m.CurrentDocID,
		ApprovedBy:      m.ApprovedBy,
		ApprovedAt:      m.ApprovedAt,
		Remark:          m.Remark,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}

	// Format appt_date
	if m.ApptDate != nil {
		resp.ApptDate = m.ApptDate.Format("2006-01-02")
	}

	if m.Officer != nil {
		if m.Officer.FullName != "" {
			resp.OfficerName = m.Officer.FullName
		} else {
			resp.OfficerName = m.Officer.Username
		}
	}
	if m.LoanType != nil {
		resp.LoanTypeName = m.LoanType.Name
	}
	if m.CurrentStep != nil {
		resp.CurrentStepCode = m.CurrentStep.Code
		resp.CurrentStepName = m.CurrentStep.Name
	}
	if m.CurrentAppt != nil {
		resp.CurrentApptName = m.CurrentAppt.Name
		resp.CurrentAppt = m.CurrentAppt
	}
	if m.CurrentDoc != nil {
		resp.CurrentDocName = m.CurrentDoc.Name
		resp.CurrentDoc = m.CurrentDoc
	}

	return resp
}

// Transaction ธุรกรรม/History
type Transaction struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MortgageID      uint      `gorm:"not null;index" json:"mortgage_id"`
	TransactionType string    `gorm:"size:50;not null" json:"transaction_type"`
	FromStepID      *uint     `json:"from_step_id"`
	ToStepID        *uint     `json:"to_step_id"`
	FromDocID       *uint     `json:"from_doc_id"`
	ToDocID         *uint     `json:"to_doc_id"`
	FromTypeID      *uint     `json:"from_type_id"`
	ToTypeID        *uint     `json:"to_type_id"`
	FromApptID      *uint     `json:"from_appt_id"`
	ToApptID        *uint     `json:"to_appt_id"`
	Amount          *float64  `gorm:"type:decimal(15,2)" json:"amount"`
	Description     string    `gorm:"type:text" json:"description"`
	PerformedBy     uint      `gorm:"not null" json:"performed_by"`
	IPAddress       string    `gorm:"size:50" json:"ip_address"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Mortgage  *Mortgage `gorm:"foreignKey:MortgageID" json:"mortgage,omitempty"`
	Performer *User     `gorm:"foreignKey:PerformedBy" json:"performer,omitempty"`
	FromStep  *LoanStep `gorm:"foreignKey:FromStepID" json:"from_step,omitempty"`
	ToStep    *LoanStep `gorm:"foreignKey:ToStepID" json:"to_step,omitempty"`
}

func (Transaction) TableName() string {
	return "transactions"
}

// Transaction Types
const (
	TxTypeCreate        = "CREATE"
	TxTypeUpdate        = "UPDATE"
	TxTypeStatusChange  = "STATUS_CHANGE"
	TxTypeTypeChange    = "TYPE_CHANGE"
	TxTypeDocCheck      = "DOC_CHECK"
	TxTypeApptCreate    = "APPT_CREATE"
	TxTypeApptComplete  = "APPT_COMPLETE"
	TxTypeApptCancel    = "APPT_CANCEL"
	TxTypeApprove       = "APPROVE"
	TxTypeReject        = "REJECT"
	TxTypeOfficerChange = "OFFICER_CHANGE"
	TxTypeAmountChange  = "AMOUNT_CHANGE"
)

// ============================================================
// Phase 6: Document Checklist
// ============================================================

// DocItem รายการเอกสารแต่ละตัว ผูกกับ loan_type (Master)
type DocItem struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	LoanTypeID uint           `gorm:"not null;index" json:"loan_type_id"`
	Code       string         `gorm:"size:30;not null" json:"code"`
	Name       string         `gorm:"size:200;not null" json:"name"`
	IsRequired bool           `gorm:"default:true" json:"is_required"`
	SortOrder  int            `gorm:"default:0" json:"sort_order"`
	IsActive   bool           `gorm:"default:true" json:"is_active"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	LoanType *LoanType `gorm:"foreignKey:LoanTypeID" json:"loan_type,omitempty"`
}

func (DocItem) TableName() string {
	return "doc_items"
}

// MortgageDocCheck เช็คลิสต์เอกสารต่อ mortgage
type MortgageDocCheck struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	MortgageID    uint       `gorm:"not null;uniqueIndex:uk_mortgage_doc" json:"mortgage_id"`
	DocItemID     uint       `gorm:"not null;uniqueIndex:uk_mortgage_doc" json:"doc_item_id"`
	IsChecked     bool       `gorm:"default:false" json:"is_checked"`
	IsRecommended bool       `gorm:"default:false" json:"is_recommended"`
	CheckedBy     *uint      `json:"checked_by"`
	CheckedAt     *time.Time `json:"checked_at"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// Relations
	Mortgage *Mortgage `gorm:"foreignKey:MortgageID" json:"mortgage,omitempty"`
	DocItem  *DocItem  `gorm:"foreignKey:DocItemID" json:"doc_item,omitempty"`
	Checker  *User     `gorm:"foreignKey:CheckedBy" json:"checker,omitempty"`
}

func (MortgageDocCheck) TableName() string {
	return "mortgage_doc_checks"
}

// MortgageDocCheckResponse DTO
type MortgageDocCheckResponse struct {
	ID            uint       `json:"id"`
	MortgageID    uint       `json:"mortgage_id"`
	DocItemID     uint       `json:"doc_item_id"`
	DocItemCode   string     `json:"doc_item_code"`
	DocItemName   string     `json:"doc_item_name"`
	IsRequired    bool       `json:"is_required"`
	IsChecked     bool       `json:"is_checked"`
	IsRecommended bool       `json:"is_recommended"`
	SortOrder     int        `json:"sort_order"`
	CheckedBy     *uint      `json:"checked_by"`
	CheckedAt     *time.Time `json:"checked_at"`
}

func (m *MortgageDocCheck) ToResponse() *MortgageDocCheckResponse {
	resp := &MortgageDocCheckResponse{
		ID:            m.ID,
		MortgageID:    m.MortgageID,
		DocItemID:     m.DocItemID,
		IsChecked:     m.IsChecked,
		IsRecommended: m.IsRecommended,
		CheckedBy:     m.CheckedBy,
		CheckedAt:     m.CheckedAt,
	}
	if m.DocItem != nil {
		resp.DocItemCode = m.DocItem.Code
		resp.DocItemName = m.DocItem.Name
		resp.IsRequired = m.DocItem.IsRequired || m.IsRecommended
		resp.SortOrder = m.DocItem.SortOrder
	}
	return resp
}

// ============================================================
// Auto Migration
// ============================================================

// AutoMigrate runs auto migration for new tables only
// ห้าม migrate ตาราง flommast!
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// Phase 2-3
		&User{},
		&RefreshToken{},
		// Phase 4: Master Tables
		&LoanType{},
		&LoanStep{},
		&LoanDoc{},
		&LoanAppt{},
		// Phase 4: Main Tables
		&Mortgage{},
		&Transaction{},
		// Phase 6: Document Checklist
		&DocItem{},
		&MortgageDocCheck{},
	)
}
