package handlers

import (
	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/pagination"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MobileHandler struct {
	db              *gorm.DB
	mortgageRepo    *repositories.MortgageRepository
	loanTypeRepo    *repositories.LoanTypeRepository
	loanStepRepo    *repositories.LoanStepRepository
	loanDocRepo     *repositories.LoanDocRepository
	loanApptRepo    *repositories.LoanApptRepository
	transactionRepo *repositories.TransactionRepository
}

func NewMobileHandler(
	db *gorm.DB,
	mortgageRepo *repositories.MortgageRepository,
	loanTypeRepo *repositories.LoanTypeRepository,
	loanStepRepo *repositories.LoanStepRepository,
	loanDocRepo *repositories.LoanDocRepository,
	loanApptRepo *repositories.LoanApptRepository,
	transactionRepo *repositories.TransactionRepository,
) *MobileHandler {
	return &MobileHandler{
		db:              db,
		mortgageRepo:    mortgageRepo,
		loanTypeRepo:    loanTypeRepo,
		loanStepRepo:    loanStepRepo,
		loanDocRepo:     loanDocRepo,
		loanApptRepo:    loanApptRepo,
		transactionRepo: transactionRepo,
	}
}

type MasterDataResponse struct {
	LoanTypes []models.LoanType `json:"loan_types"`
	LoanSteps []models.LoanStep `json:"loan_steps"`
	LoanDocs  []models.LoanDoc  `json:"loan_docs"`
	LoanAppts []models.LoanAppt `json:"loan_appts"`
}

func (h *MobileHandler) GetMasterData(c *fiber.Ctx) error {
	loanTypes, _ := h.loanTypeRepo.List(c.Context())
	loanSteps, _ := h.loanStepRepo.List(c.Context())
	loanDocs, _ := h.loanDocRepo.List(c.Context())
	loanAppts, _ := h.loanApptRepo.List(c.Context())

	types := make([]models.LoanType, len(loanTypes))
	for i, t := range loanTypes {
		types[i] = *t
	}
	steps := make([]models.LoanStep, len(loanSteps))
	for i, s := range loanSteps {
		steps[i] = *s
	}
	docs := make([]models.LoanDoc, len(loanDocs))
	for i, d := range loanDocs {
		docs[i] = *d
	}
	appts := make([]models.LoanAppt, len(loanAppts))
	for i, a := range loanAppts {
		appts[i] = *a
	}

	c.Set("Cache-Control", "public, max-age=3600")
	return response.Success(c, "Master data retrieved successfully", fiber.Map{
		"master": MasterDataResponse{LoanTypes: types, LoanSteps: steps, LoanDocs: docs, LoanAppts: appts},
	})
}

type MyLoansLiteResponse struct {
	ID           uint    `json:"id"`
	MembNo       string  `json:"memb_no"`
	Amount       float64 `json:"amount"`
	LoanTypeName string  `json:"loan_type_name"`
	CurrentStep  string  `json:"current_step"`
	StepColor    string  `json:"step_color"`
	ApptDate     string  `json:"appt_date,omitempty"`
	ApptTime     string  `json:"appt_time,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func (h *MobileHandler) GetMyLoans(c *fiber.Ctx) error {
	membNo, ok := c.Locals("membNo").(string)
	if !ok || membNo == "" {
		return response.Unauthorized(c, "User not found in context")
	}

	params := pagination.GetParams(c)
	var total int64
	h.db.Model(&models.Mortgage{}).Where("memb_no = ?", membNo).Count(&total)

	var mortgages []models.Mortgage
	h.db.Preload("LoanType").Preload("CurrentStep").Where("memb_no = ?", membNo).Order("created_at DESC").Offset(params.Offset).Limit(params.Limit).Find(&mortgages)

	liteLoans := make([]MyLoansLiteResponse, len(mortgages))
	for i, m := range mortgages {
		liteLoans[i] = MyLoansLiteResponse{ID: m.ID, MembNo: m.MembNo, Amount: m.Amount, ApptTime: m.ApptTime, CreatedAt: m.CreatedAt.Format("2006-01-02")}
		if m.LoanType != nil {
			liteLoans[i].LoanTypeName = m.LoanType.Name
		}
		if m.CurrentStep != nil {
			liteLoans[i].CurrentStep = m.CurrentStep.Name
			liteLoans[i].StepColor = m.CurrentStep.Color
		}
		if m.ApptDate != nil {
			liteLoans[i].ApptDate = m.ApptDate.Format("2006-01-02")
		}
	}

	c.Set("Cache-Control", "private, max-age=60")
	return response.Success(c, "Loans retrieved successfully", fiber.Map{"loans": liteLoans, "meta": pagination.GetMeta(params, total)})
}

type MobileDashboardResponse struct {
	User        UserInfo             `json:"user"`
	Stats       DashboardStats       `json:"stats"`
	RecentLoans []MyLoansLiteResponse `json:"recent_loans"`
	Master      MasterDataResponse   `json:"master"`
}

type UserInfo struct {
	ID       uint   `json:"id"`
	MembNo   string `json:"memb_no"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

type DashboardStats struct {
	TotalLoans    int64   `json:"total_loans"`
	PendingLoans  int64   `json:"pending_loans"`
	ApprovedLoans int64   `json:"approved_loans"`
	TotalAmount   float64 `json:"total_amount"`
}

func (h *MobileHandler) GetDashboard(c *fiber.Ctx) error {
	userID, _ := c.Locals("userID").(uint)
	membNo, ok := c.Locals("membNo").(string)
	if !ok || membNo == "" {
		return response.Unauthorized(c, "User not found in context")
	}
	username, _ := c.Locals("username").(string)
	role, _ := c.Locals("role").(string)

	dashboard := MobileDashboardResponse{}
	dashboard.User = UserInfo{ID: userID, MembNo: membNo, Username: username, Role: role}

	var member models.Flommast
	if err := h.db.Where("mast_memb_no = ?", membNo).First(&member).Error; err == nil {
		dashboard.User.FullName = member.FullName
	}

	var stats DashboardStats
	h.db.Model(&models.Mortgage{}).Where("memb_no = ?", membNo).Count(&stats.TotalLoans)
	h.db.Model(&models.Mortgage{}).Where("memb_no = ? AND current_step_id IN (1,2,3,4)", membNo).Count(&stats.PendingLoans)
	h.db.Model(&models.Mortgage{}).Where("memb_no = ? AND current_step_id = 5", membNo).Count(&stats.ApprovedLoans)
	h.db.Model(&models.Mortgage{}).Where("memb_no = ?", membNo).Select("COALESCE(SUM(amount), 0)").Scan(&stats.TotalAmount)
	dashboard.Stats = stats

	var recentMortgages []models.Mortgage
	h.db.Preload("LoanType").Preload("CurrentStep").Where("memb_no = ?", membNo).Order("created_at DESC").Limit(5).Find(&recentMortgages)

	recentLoans := make([]MyLoansLiteResponse, len(recentMortgages))
	for i, m := range recentMortgages {
		recentLoans[i] = MyLoansLiteResponse{ID: m.ID, MembNo: m.MembNo, Amount: m.Amount, ApptTime: m.ApptTime, CreatedAt: m.CreatedAt.Format("2006-01-02")}
		if m.LoanType != nil {
			recentLoans[i].LoanTypeName = m.LoanType.Name
		}
		if m.CurrentStep != nil {
			recentLoans[i].CurrentStep = m.CurrentStep.Name
			recentLoans[i].StepColor = m.CurrentStep.Color
		}
		if m.ApptDate != nil {
			recentLoans[i].ApptDate = m.ApptDate.Format("2006-01-02")
		}
	}
	dashboard.RecentLoans = recentLoans

	loanTypes, _ := h.loanTypeRepo.List(c.Context())
	loanSteps, _ := h.loanStepRepo.List(c.Context())
	loanDocs, _ := h.loanDocRepo.List(c.Context())
	loanAppts, _ := h.loanApptRepo.List(c.Context())

	types := make([]models.LoanType, len(loanTypes))
	for i, t := range loanTypes {
		types[i] = *t
	}
	steps := make([]models.LoanStep, len(loanSteps))
	for i, s := range loanSteps {
		steps[i] = *s
	}
	docs := make([]models.LoanDoc, len(loanDocs))
	for i, d := range loanDocs {
		docs[i] = *d
	}
	appts := make([]models.LoanAppt, len(loanAppts))
	for i, a := range loanAppts {
		appts[i] = *a
	}
	dashboard.Master = MasterDataResponse{LoanTypes: types, LoanSteps: steps, LoanDocs: docs, LoanAppts: appts}

	c.Set("Cache-Control", "private, max-age=60")
	return response.Success(c, "Dashboard retrieved successfully", dashboard)
}
