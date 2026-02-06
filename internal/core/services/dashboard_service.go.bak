package services

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DashboardService handles dashboard operations
type DashboardService struct {
	db *gorm.DB
}

// NewDashboardService creates a new dashboard service
func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

// ============================================================
// Admin Dashboard
// ============================================================

// AdminDashboardData represents admin dashboard data
type AdminDashboardData struct {
	// User Statistics
	TotalUsers    int64 `json:"total_users"`
	TotalAdmins   int64 `json:"total_admins"`
	TotalOfficers int64 `json:"total_officers"`
	TotalMembers  int64 `json:"total_members"`

	// Mortgage Statistics
	TotalMortgages    int64   `json:"total_mortgages"`
	TotalAmount       float64 `json:"total_amount"`
	ApprovedAmount    float64 `json:"approved_amount"`
	PendingMortgages  int64   `json:"pending_mortgages"`
	ApprovedMortgages int64   `json:"approved_mortgages"`
	RejectedMortgages int64   `json:"rejected_mortgages"`

	// Monthly Statistics
	MortgagesThisMonth int64   `json:"mortgages_this_month"`
	AmountThisMonth    float64 `json:"amount_this_month"`

	// Recent Activity
	RecentMortgages []MortgageSummary `json:"recent_mortgages"`

	// Top Officers
	TopOfficers []OfficerStats `json:"top_officers"`
}

// MortgageSummary represents mortgage summary
type MortgageSummary struct {
	ID        uint      `json:"id"`
	MembNo    string    `json:"memb_no"`
	Amount    float64   `json:"amount"`
	LoanType  string    `json:"loan_type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// OfficerStats represents officer statistics
type OfficerStats struct {
	OfficerID  uint   `json:"officer_id"`
	Username   string `json:"username"`
	TotalCases int64  `json:"total_cases"`
	Approved   int64  `json:"approved"`
	Rejected   int64  `json:"rejected"`
	Pending    int64  `json:"pending"`
}

// GetAdminDashboard returns admin dashboard data
func (s *DashboardService) GetAdminDashboard(ctx context.Context) (*AdminDashboardData, error) {
	data := &AdminDashboardData{}

	// User counts by role
	s.db.WithContext(ctx).Table("users").Where("deleted_at IS NULL").Count(&data.TotalUsers)
	s.db.WithContext(ctx).Table("users").Where("role = ? AND deleted_at IS NULL", "ADMIN").Count(&data.TotalAdmins)
	s.db.WithContext(ctx).Table("users").Where("role = ? AND deleted_at IS NULL", "OFFICER").Count(&data.TotalOfficers)
	s.db.WithContext(ctx).Table("users").Where("role = ? AND deleted_at IS NULL", "USER").Count(&data.TotalMembers)

	// Mortgage counts
	s.db.WithContext(ctx).Table("mortgages").Where("deleted_at IS NULL").Count(&data.TotalMortgages)

	// Total amount
	s.db.WithContext(ctx).Table("mortgages").
		Where("deleted_at IS NULL").
		Select("COALESCE(SUM(amount), 0)").
		Scan(&data.TotalAmount)

	// Approved amount
	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("loan_steps.code = ? AND mortgages.deleted_at IS NULL", "APPROVED").
		Select("COALESCE(SUM(mortgages.amount), 0)").
		Scan(&data.ApprovedAmount)

	// Mortgage counts by status
	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("loan_steps.is_final = ? AND mortgages.deleted_at IS NULL", false).
		Count(&data.PendingMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("loan_steps.code = ? AND mortgages.deleted_at IS NULL", "APPROVED").
		Count(&data.ApprovedMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("loan_steps.code = ? AND mortgages.deleted_at IS NULL", "REJECTED").
		Count(&data.RejectedMortgages)

	// This month statistics
	startOfMonth := time.Now().AddDate(0, 0, -time.Now().Day()+1).Truncate(24 * time.Hour)
	s.db.WithContext(ctx).Table("mortgages").
		Where("created_at >= ? AND deleted_at IS NULL", startOfMonth).
		Count(&data.MortgagesThisMonth)

	s.db.WithContext(ctx).Table("mortgages").
		Where("created_at >= ? AND deleted_at IS NULL", startOfMonth).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&data.AmountThisMonth)

	// Recent mortgages
	var recentMortgages []struct {
		ID        uint
		MembNo    string
		Amount    float64
		LoanType  string
		Status    string
		CreatedAt time.Time
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select("mortgages.id, mortgages.memb_no, mortgages.amount, loan_types.name as loan_type, loan_steps.name as status, mortgages.created_at").
		Joins("LEFT JOIN loan_types ON mortgages.loan_type_id = loan_types.id").
		Joins("LEFT JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.deleted_at IS NULL").
		Order("mortgages.created_at DESC").
		Limit(10).
		Scan(&recentMortgages)

	data.RecentMortgages = make([]MortgageSummary, len(recentMortgages))
	for i, m := range recentMortgages {
		data.RecentMortgages[i] = MortgageSummary{
			ID:        m.ID,
			MembNo:    m.MembNo,
			Amount:    m.Amount,
			LoanType:  m.LoanType,
			Status:    m.Status,
			CreatedAt: m.CreatedAt,
		}
	}

	// Top officers
	var topOfficers []struct {
		OfficerID  uint
		Username   string
		TotalCases int64
		Approved   int64
		Rejected   int64
		Pending    int64
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select(`
			mortgages.officer_id,
			users.username,
			COUNT(*) as total_cases,
			SUM(CASE WHEN loan_steps.code = 'APPROVED' THEN 1 ELSE 0 END) as approved,
			SUM(CASE WHEN loan_steps.code = 'REJECTED' THEN 1 ELSE 0 END) as rejected,
			SUM(CASE WHEN loan_steps.is_final = 0 THEN 1 ELSE 0 END) as pending
		`).
		Joins("LEFT JOIN users ON mortgages.officer_id = users.id").
		Joins("LEFT JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.deleted_at IS NULL AND mortgages.officer_id IS NOT NULL").
		Group("mortgages.officer_id, users.username").
		Order("total_cases DESC").
		Limit(5).
		Scan(&topOfficers)

	data.TopOfficers = make([]OfficerStats, len(topOfficers))
	for i, o := range topOfficers {
		data.TopOfficers[i] = OfficerStats{
			OfficerID:  o.OfficerID,
			Username:   o.Username,
			TotalCases: o.TotalCases,
			Approved:   o.Approved,
			Rejected:   o.Rejected,
			Pending:    o.Pending,
		}
	}

	return data, nil
}

// ============================================================
// Officer Dashboard
// ============================================================

// OfficerDashboardData represents officer dashboard data
type OfficerDashboardData struct {
	// My Statistics
	TotalAssigned      int64   `json:"total_assigned"`
	PendingCases       int64   `json:"pending_cases"`
	ApprovedCases      int64   `json:"approved_cases"`
	RejectedCases      int64   `json:"rejected_cases"`
	TotalAmountHandled float64 `json:"total_amount_handled"`

	// Today's Tasks
	TodayAppointments []AppointmentInfo `json:"today_appointments"`

	// Pending Actions
	PendingMortgages []MortgageSummary `json:"pending_mortgages"`

	// This Week Appointments
	WeekAppointments []AppointmentInfo `json:"week_appointments"`

	// Recent Activity
	RecentTransactions []TransactionInfo `json:"recent_transactions"`
}

// AppointmentInfo represents appointment information
type AppointmentInfo struct {
	ID         uint   `json:"id"`
	MortgageID uint   `json:"mortgage_id"`
	MembNo     string `json:"memb_no"`
	ApptType   string `json:"appt_type"`
	ApptDate   string `json:"appt_date"`
	ApptTime   string `json:"appt_time"`
	Location   string `json:"location"`
}

// TransactionInfo represents transaction information
type TransactionInfo struct {
	ID         uint      `json:"id"`
	MortgageID uint      `json:"mortgage_id"`
	Action     string    `json:"action"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetOfficerDashboard returns officer dashboard data
func (s *DashboardService) GetOfficerDashboard(ctx context.Context, officerID uint) (*OfficerDashboardData, error) {
	data := &OfficerDashboardData{}

	// My statistics
	s.db.WithContext(ctx).Table("mortgages").
		Where("officer_id = ? AND deleted_at IS NULL", officerID).
		Count(&data.TotalAssigned)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.officer_id = ? AND loan_steps.is_final = ? AND mortgages.deleted_at IS NULL", officerID, false).
		Count(&data.PendingCases)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.officer_id = ? AND loan_steps.code = ? AND mortgages.deleted_at IS NULL", officerID, "APPROVED").
		Count(&data.ApprovedCases)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.officer_id = ? AND loan_steps.code = ? AND mortgages.deleted_at IS NULL", officerID, "REJECTED").
		Count(&data.RejectedCases)

	s.db.WithContext(ctx).Table("mortgages").
		Where("officer_id = ? AND deleted_at IS NULL", officerID).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&data.TotalAmountHandled)

	// Today's appointments - ใช้ mortgages.appt_date แทน loan_appt_currents
	today := time.Now().Format("2006-01-02")
	var todayAppts []struct {
		ID         uint
		MortgageID uint
		MembNo     string
		ApptType   string
		ApptDate   string
		ApptTime   string
		Location   string
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select(`
			mortgages.id,
			mortgages.id as mortgage_id,
			mortgages.memb_no,
			COALESCE(loan_appts.name, 'นัดหมาย') as appt_type,
			DATE_FORMAT(mortgages.appt_date, '%Y-%m-%d') as appt_date,
			mortgages.appt_time,
			mortgages.appt_location as location
		`).
		Joins("LEFT JOIN loan_appts ON mortgages.current_appt_id = loan_appts.id").
		Where("mortgages.officer_id = ? AND DATE(mortgages.appt_date) = ? AND mortgages.deleted_at IS NULL", officerID, today).
		Order("mortgages.appt_time ASC").
		Scan(&todayAppts)

	data.TodayAppointments = make([]AppointmentInfo, len(todayAppts))
	for i, a := range todayAppts {
		data.TodayAppointments[i] = AppointmentInfo{
			ID:         a.ID,
			MortgageID: a.MortgageID,
			MembNo:     a.MembNo,
			ApptType:   a.ApptType,
			ApptDate:   a.ApptDate,
			ApptTime:   a.ApptTime,
			Location:   a.Location,
		}
	}

	// Pending mortgages
	var pendingMortgages []struct {
		ID        uint
		MembNo    string
		Amount    float64
		LoanType  string
		Status    string
		CreatedAt time.Time
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select("mortgages.id, mortgages.memb_no, mortgages.amount, loan_types.name as loan_type, loan_steps.name as status, mortgages.created_at").
		Joins("LEFT JOIN loan_types ON mortgages.loan_type_id = loan_types.id").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.officer_id = ? AND loan_steps.is_final = ? AND mortgages.deleted_at IS NULL", officerID, false).
		Order("mortgages.created_at ASC").
		Limit(10).
		Scan(&pendingMortgages)

	data.PendingMortgages = make([]MortgageSummary, len(pendingMortgages))
	for i, m := range pendingMortgages {
		data.PendingMortgages[i] = MortgageSummary{
			ID:        m.ID,
			MembNo:    m.MembNo,
			Amount:    m.Amount,
			LoanType:  m.LoanType,
			Status:    m.Status,
			CreatedAt: m.CreatedAt,
		}
	}

	// This week appointments - ใช้ mortgages.appt_date แทน loan_appt_currents
	startOfWeek := time.Now().AddDate(0, 0, -int(time.Now().Weekday()))
	endOfWeek := startOfWeek.AddDate(0, 0, 7)
	var weekAppts []struct {
		ID         uint
		MortgageID uint
		MembNo     string
		ApptType   string
		ApptDate   string
		ApptTime   string
		Location   string
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select(`
			mortgages.id,
			mortgages.id as mortgage_id,
			mortgages.memb_no,
			COALESCE(loan_appts.name, 'นัดหมาย') as appt_type,
			DATE_FORMAT(mortgages.appt_date, '%Y-%m-%d') as appt_date,
			mortgages.appt_time,
			mortgages.appt_location as location
		`).
		Joins("LEFT JOIN loan_appts ON mortgages.current_appt_id = loan_appts.id").
		Where("mortgages.officer_id = ? AND mortgages.appt_date >= ? AND mortgages.appt_date < ? AND mortgages.deleted_at IS NULL",
			officerID, startOfWeek.Format("2006-01-02"), endOfWeek.Format("2006-01-02")).
		Order("mortgages.appt_date ASC, mortgages.appt_time ASC").
		Scan(&weekAppts)

	data.WeekAppointments = make([]AppointmentInfo, len(weekAppts))
	for i, a := range weekAppts {
		data.WeekAppointments[i] = AppointmentInfo{
			ID:         a.ID,
			MortgageID: a.MortgageID,
			MembNo:     a.MembNo,
			ApptType:   a.ApptType,
			ApptDate:   a.ApptDate,
			ApptTime:   a.ApptTime,
			Location:   a.Location,
		}
	}

	// Recent transactions
	var recentTxns []struct {
		ID         uint
		MortgageID uint
		Action     string
		OldValue   string
		NewValue   string
		Remark     string
		CreatedAt  time.Time
	}
	s.db.WithContext(ctx).Table("transactions").
		Select("transactions.id, transactions.mortgage_id, transactions.transaction_type as action, COALESCE(from_step.name, '') as old_value, COALESCE(to_step.name, '') as new_value, transactions.description as remark, transactions.created_at").
		Joins("LEFT JOIN loan_steps as from_step ON transactions.from_step_id = from_step.id").
		Joins("LEFT JOIN loan_steps as to_step ON transactions.to_step_id = to_step.id").
		Where("transactions.performed_by = ?", officerID).
		Order("transactions.created_at DESC").
		Limit(10).
		Scan(&recentTxns)

	data.RecentTransactions = make([]TransactionInfo, len(recentTxns))
	for i, t := range recentTxns {
		data.RecentTransactions[i] = TransactionInfo{
			ID:         t.ID,
			MortgageID: t.MortgageID,
			Action:     t.Action,
			OldValue:   t.OldValue,
			NewValue:   t.NewValue,
			Remark:     t.Remark,
			CreatedAt:  t.CreatedAt,
		}
	}

	return data, nil
}

// ============================================================
// User Dashboard
// ============================================================

// UserDashboardData represents user dashboard data
type UserDashboardData struct {
	// My Mortgages Summary
	TotalMortgages    int64   `json:"total_mortgages"`
	PendingMortgages  int64   `json:"pending_mortgages"`
	ApprovedMortgages int64   `json:"approved_mortgages"`
	RejectedMortgages int64   `json:"rejected_mortgages"`
	TotalBorrowed     float64 `json:"total_borrowed"`

	// My Mortgages List
	Mortgages []UserMortgageInfo `json:"mortgages"`

	// Upcoming Appointments
	UpcomingAppointments []AppointmentInfo `json:"upcoming_appointments"`
}

// UserMortgageInfo represents user mortgage information
type UserMortgageInfo struct {
	ID          uint       `json:"id"`
	ContractNo  string     `json:"contract_no"`
	Amount      float64    `json:"amount"`
	LoanType    string     `json:"loan_type"`
	Status      string     `json:"status"`
	StatusColor string     `json:"status_color"`
	OfficerName string     `json:"officer_name"`
	CreatedAt   time.Time  `json:"created_at"`
	ApprovedAt  *time.Time `json:"approved_at"`
}

// GetUserDashboard returns user dashboard data
func (s *DashboardService) GetUserDashboard(ctx context.Context, membNo string) (*UserDashboardData, error) {
	data := &UserDashboardData{}

	// My statistics
	s.db.WithContext(ctx).Table("mortgages").
		Where("memb_no = ? AND deleted_at IS NULL", membNo).
		Count(&data.TotalMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.memb_no = ? AND loan_steps.is_final = ? AND mortgages.deleted_at IS NULL", membNo, false).
		Count(&data.PendingMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.memb_no = ? AND loan_steps.code = ? AND mortgages.deleted_at IS NULL", membNo, "APPROVED").
		Count(&data.ApprovedMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.memb_no = ? AND loan_steps.code = ? AND mortgages.deleted_at IS NULL", membNo, "REJECTED").
		Count(&data.RejectedMortgages)

	s.db.WithContext(ctx).Table("mortgages").
		Joins("JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Where("mortgages.memb_no = ? AND loan_steps.code = ? AND mortgages.deleted_at IS NULL", membNo, "APPROVED").
		Select("COALESCE(SUM(mortgages.amount), 0)").
		Scan(&data.TotalBorrowed)

	// My mortgages
	var mortgages []struct {
		ID          uint
		ContractNo  string
		Amount      float64
		LoanType    string
		Status      string
		StatusColor string
		OfficerName string
		CreatedAt   time.Time
		ApprovedAt  *time.Time
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select("mortgages.id, mortgages.contract_no, mortgages.amount, loan_types.name as loan_type, loan_steps.name as status, loan_steps.color as status_color, users.username as officer_name, mortgages.created_at, mortgages.approved_at").
		Joins("LEFT JOIN loan_types ON mortgages.loan_type_id = loan_types.id").
		Joins("LEFT JOIN loan_steps ON mortgages.current_step_id = loan_steps.id").
		Joins("LEFT JOIN users ON mortgages.officer_id = users.id").
		Where("mortgages.memb_no = ? AND mortgages.deleted_at IS NULL", membNo).
		Order("mortgages.created_at DESC").
		Scan(&mortgages)

	data.Mortgages = make([]UserMortgageInfo, len(mortgages))
	for i, m := range mortgages {
		data.Mortgages[i] = UserMortgageInfo{
			ID:          m.ID,
			ContractNo:  m.ContractNo,
			Amount:      m.Amount,
			LoanType:    m.LoanType,
			Status:      m.Status,
			StatusColor: m.StatusColor,
			OfficerName: m.OfficerName,
			CreatedAt:   m.CreatedAt,
			ApprovedAt:  m.ApprovedAt,
		}
	}

	// Upcoming appointments - ใช้ mortgages.appt_date แทน loan_appt_currents
	var upcomingAppts []struct {
		ID         uint
		MortgageID uint
		MembNo     string
		ApptType   string
		ApptDate   string
		ApptTime   string
		Location   string
	}
	s.db.WithContext(ctx).Table("mortgages").
		Select(`
			mortgages.id,
			mortgages.id as mortgage_id,
			mortgages.memb_no,
			COALESCE(loan_appts.name, 'นัดหมาย') as appt_type,
			DATE_FORMAT(mortgages.appt_date, '%Y-%m-%d') as appt_date,
			mortgages.appt_time,
			mortgages.appt_location as location
		`).
		Joins("LEFT JOIN loan_appts ON mortgages.current_appt_id = loan_appts.id").
		Where("mortgages.memb_no = ? AND mortgages.appt_date >= ? AND mortgages.deleted_at IS NULL",
			membNo, time.Now().Format("2006-01-02")).
		Order("mortgages.appt_date ASC, mortgages.appt_time ASC").
		Limit(5).
		Scan(&upcomingAppts)

	data.UpcomingAppointments = make([]AppointmentInfo, len(upcomingAppts))
	for i, a := range upcomingAppts {
		data.UpcomingAppointments[i] = AppointmentInfo{
			ID:         a.ID,
			MortgageID: a.MortgageID,
			MembNo:     a.MembNo,
			ApptType:   a.ApptType,
			ApptDate:   a.ApptDate,
			ApptTime:   a.ApptTime,
			Location:   a.Location,
		}
	}

	return data, nil
}
