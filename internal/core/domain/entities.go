package domain

import "time"

// Role represents user role in the system
type Role string

const (
	RoleUser    Role = "USER"
	RoleOfficer Role = "OFFICER"
	RoleAdmin   Role = "ADMIN"
)

// User represents a user in the domain layer
type User struct {
	ID        uint
	MembNo    string // Maps to flommast.MAST_MEMB_NO
	Username  string
	Email     string
	Password  string // Hashed
	Role      Role
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Member represents a member from flommast (Legacy - Read Only)
type Member struct {
	MembNo      string // MAST_MEMB_NO
	FullName    string // Full_Name
	DeptName    string // DEPT_NAME
	StsTypeDesc string // STS_TYPE_DESC
}

// RefreshToken represents a refresh token in the domain
type RefreshToken struct {
	ID        uint
	UserID    uint
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

// Loan represents a loan in the domain (Phase 4)
type Loan struct {
	ID          uint
	MembNo      string
	Amount      float64
	Status      string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
