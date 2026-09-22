package routes

import (
	"spsc-loaneasy/internal/adapters/http/handlers"
	"spsc-loaneasy/internal/adapters/http/middleware"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/core/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
	"gorm.io/gorm"
)

// audit returns the audit-log middleware for an action name (set up in Setup)
var audit func(action string) fiber.Handler

// Setup configures all routes for the application
func Setup(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	refreshTokenRepo := repositories.NewRefreshTokenRepository(db)
	memberRepo := repositories.NewMemberRepository(db)

	// Phase 4: Master repositories
	loanTypeRepo := repositories.NewLoanTypeRepository(db)
	loanStepRepo := repositories.NewLoanStepRepository(db)
	loanDocRepo := repositories.NewLoanDocRepository(db)
	loanApptRepo := repositories.NewLoanApptRepository(db)

	// Phase 4: Mortgage repositories
	mortgageRepo := repositories.NewMortgageRepository(db)
	transactionRepo := repositories.NewTransactionRepository(db)

	// Phase 7: Committee repository
	committeeRepo := repositories.NewCommitteeRepository(db)
	committeeVisibilityRepo := repositories.NewCommitteeVisibilityRepository(db)
	pdpaSettingRepo := repositories.NewPDPASettingRepository(db)

	// Phase 6: Doc Check repositories
	docItemRepo := repositories.NewDocItemRepository(db)
	docCheckRepo := repositories.NewMortgageDocCheckRepository(db)

	// Security: audit log (ใครดู/แก้ข้อมูลอะไร) — ใช้ผ่าน audit("action") ในทุก route group
	auditLogRepo := repositories.NewAuditLogRepository(db)
	audit = func(action string) fiber.Handler { return middleware.Audit(auditLogRepo, action) }

	// Phase 1 (Loan Print): repositories
	loanPurposeRepo := repositories.NewLoanPurposeRepository(db)
	flommastImportRepo := repositories.NewFlommastImportRepository(db)

	// LINE Handler (moved up: needed by MortgageService for committee notifications)
	lineHandler := handlers.NewLINEHandler(db)
	lineService := lineHandler.GetLINEService()

	// Initialize services
	authService := services.NewAuthService(userRepo, refreshTokenRepo, memberRepo, cfg)
	userService := services.NewUserService(userRepo, memberRepo)

	// Phase 4: Notification service
	notifyService := services.NewNotificationService()

	// Phase 4: Mortgage service
	mortgageService := services.NewMortgageService(
		mortgageRepo,
		transactionRepo,
		loanTypeRepo,
		loanStepRepo,
		loanDocRepo,
		loanApptRepo,
		memberRepo,
		userRepo,
		notifyService,
		committeeRepo,
		lineService,
	)

	// Phase 5: Dashboard service
	dashboardService := services.NewDashboardService(db)

	// Phase 8: Report service (รายงานประจำเดือน แยกตามขั้นตอน)
	reportService := services.NewReportService(db)

	// Phase 7: Committee service
	committeeService := services.NewCommitteeService(committeeRepo, memberRepo, mortgageRepo, committeeVisibilityRepo, pdpaSettingRepo)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(authService, cfg)
	userHandler := handlers.NewUserHandler(userService)

	// Phase 4: Handlers
	mortgageHandler := handlers.NewMortgageHandler(mortgageService)
	masterHandler := handlers.NewMasterHandler(loanTypeRepo, loanStepRepo, loanDocRepo, loanApptRepo)

	// Phase 5: Dashboard handler
	dashboardHandler := handlers.NewDashboardHandler(dashboardService)

	// Phase 8: Report handler
	reportHandler := handlers.NewReportHandler(reportService)

	// Phase 7: Committee handler
	committeeHandler := handlers.NewCommitteeHandler(committeeService)

	// ============================================================
	// LIFF Handler v3 - รับ lineService + otpService + smsService
	// (lineHandler/lineService constructed earlier, above MortgageService)
	// ============================================================
	otpService := services.NewOTPService(db)
	smsService := services.NewSMSService(lineService)
	liffHandler := handlers.NewLIFFHandler(db, lineService, otpService, smsService, authService, cfg)

	// v2.2.2: Mobile Handler (Aggregated APIs)
	mobileHandler := handlers.NewMobileHandler(
		db,
		mortgageRepo,
		loanTypeRepo,
		loanStepRepo,
		loanDocRepo,
		loanApptRepo,
		transactionRepo,
	)

	// Phase 6: DocCheck service & handler
	docCheckService := services.NewDocCheckService(
		docItemRepo,
		docCheckRepo,
		mortgageRepo,
		lineService,
		db,
	)
	docCheckHandler := handlers.NewDocCheckHandler(docCheckService, docItemRepo)

	// Phase 3a: App counter repository (auto-numbering)
	appCounterRepo := repositories.NewAppCounterRepository(db)

	// Phase 1 (Loan Print): handlers
	savingsRepo := repositories.NewSavingsRepository(db)
	loanPrintHandler := handlers.NewLoanPrintHandler(memberRepo, loanPurposeRepo, appCounterRepo, savingsRepo)
	flommastImportHandler := handlers.NewFlommastImportHandler(flommastImportRepo)
	flommastSyncHandler := handlers.NewFlommastSyncHandler(flommastImportRepo, db)

	// Health check & root routes
	app.Get("/", healthHandler.Root)
	app.Get("/health", healthHandler.HealthCheck)

	// Swagger documentation
	// prod ไม่เปิด — ไม่ให้คนนอกเห็นแผนที่ API ทั้งหมด
	if cfg.IsDev() {
		app.Get("/swagger/*", swagger.HandlerDefault)
	}

	// API v1 group
	apiV1 := app.Group("/api/v1")
	setupAPIV1Routes(apiV1, healthHandler, authHandler, userHandler, mortgageHandler,
		masterHandler, dashboardHandler, lineHandler, liffHandler, docCheckHandler,
		loanPrintHandler, flommastImportHandler, flommastSyncHandler, committeeHandler,
		reportHandler, cfg)

	// Security: audit log viewer (Admin only)
	auditHandler := handlers.NewAuditHandler(auditLogRepo)
	apiV1.Get("/admin/audit-logs",
		middleware.AuthMiddleware(cfg),
		middleware.AdminOnly(),
		audit("audit.view"),
		auditHandler.List,
	)

	// API v2 group (Mobile-optimized)
	apiV2 := app.Group("/api/v2")
	setupAPIV2Routes(apiV2, mobileHandler, cfg)
}

// setupAPIV1Routes configures API v1 routes
func setupAPIV1Routes(
	router fiber.Router,
	healthHandler *handlers.HealthHandler,
	authHandler *handlers.AuthHandler,
	userHandler *handlers.UserHandler,
	mortgageHandler *handlers.MortgageHandler,
	masterHandler *handlers.MasterHandler,
	dashboardHandler *handlers.DashboardHandler,
	lineHandler *handlers.LINEHandler,
	liffHandler *handlers.LIFFHandler,
	docCheckHandler *handlers.DocCheckHandler,
	loanPrintHandler *handlers.LoanPrintHandler,
	flommastImportHandler *handlers.FlommastImportHandler,
	flommastSyncHandler *handlers.FlommastSyncHandler,
	committeeHandler *handlers.CommitteeHandler,
	reportHandler *handlers.ReportHandler,
	cfg *config.Config,
) {
	// API Info
	router.Get("/", healthHandler.APIInfo)

	// Auth routes (public)
	authRoutes := router.Group("/auth")
	setupAuthRoutes(authRoutes, authHandler, cfg)

	// LINE routes
	// /auth/line/* (LINE OAuth แบบ redirect) ปิดแล้ว 2026-09-22: ไม่มี frontend ใช้ (ใช้ LIFF แทน)
	// และออก token แบบเดิมที่ไม่ผ่าน AuthService — lineHandler ยังใช้สำหรับ LINE service อยู่

	// LIFF routes (for LIFF SDK login - PUBLIC)
	liffRoutes := router.Group("/auth/liff")
	setupLIFFRoutes(liffRoutes, liffHandler)

	// User management routes (Admin only)
	userRoutes := router.Group("/users")
	userRoutes.Use(middleware.AuthMiddleware(cfg))
	setupUserRoutes(userRoutes, userHandler)

	// Profile routes (Authenticated users)
	profileRoutes := router.Group("/profile")
	profileRoutes.Use(middleware.AuthMiddleware(cfg))
	setupProfileRoutes(profileRoutes, userHandler)

	// Phase 4: Mortgage routes (Officer/Admin)
	mortgageRoutes := router.Group("/mortgages")
	mortgageRoutes.Use(middleware.AuthMiddleware(cfg))
	setupMortgageRoutes(mortgageRoutes, mortgageHandler, cfg)

	// Phase 6: Doc Checks routes (under mortgages, Officer/Admin)
	setupDocCheckRoutes(mortgageRoutes, docCheckHandler)

	// Phase 4: Master routes (Admin only)
	masterRoutes := router.Group("/master")
	masterRoutes.Use(middleware.AuthMiddleware(cfg))
	setupMasterRoutes(masterRoutes, masterHandler)

	// Phase 6: Doc Items master routes (reuse masterRoutes auth)
	setupDocItemRoutes(masterRoutes, docCheckHandler)

	// Phase 5: Dashboard routes
	dashboardRoutes := router.Group("/dashboard")
	dashboardRoutes.Use(middleware.AuthMiddleware(cfg))
	setupDashboardRoutes(dashboardRoutes, dashboardHandler)

	// Phase 8: Report routes (Officer + Admin)
	reportRoutes := router.Group("/reports")
	reportRoutes.Use(middleware.AuthMiddleware(cfg))
	reportRoutes.Use(middleware.OfficerOrAdmin())
	reportRoutes.Get("/monthly-steps", audit("report.monthly_steps"), reportHandler.GetMonthlyStepReport)

	// Phase 1 (Loan Print): Officer + Admin
	loanPrintRoutes := router.Group("/loan-print")
	loanPrintRoutes.Use(middleware.AuthMiddleware(cfg))
	loanPrintRoutes.Use(middleware.OfficerOrAdmin())
	setupLoanPrintRoutes(loanPrintRoutes, loanPrintHandler)

	// Phase 2 (Flommast Sync — agent push): API Key auth
	//   MUST be registered BEFORE the /admin/flommast JWT group below,
	//   otherwise Fiber's prefix-Use middleware runs JWT check first.
	router.Post("/admin/flommast/sync",
		middleware.APIKeyAuth(cfg),
		audit("flommast.sync"),
		flommastSyncHandler.Sync,
	)

	// Phase 1 (Flommast Import): Admin only
	flommastAdminRoutes := router.Group("/admin/flommast")
	flommastAdminRoutes.Use(middleware.AuthMiddleware(cfg))
	flommastAdminRoutes.Use(middleware.AdminOnly())
	setupFlommastImportRoutes(flommastAdminRoutes, flommastImportHandler)

	// Phase 3A (Missing members — list + bulk delete): JWT/Admin
	flommastAdminRoutes.Get("/missing", flommastSyncHandler.Missing)
	flommastAdminRoutes.Delete("/missing", audit("flommast.delete_missing"), flommastSyncHandler.DeleteMissing)

	// Phase 2 (Flommast Sync — read-only monitoring): Officer + Admin
	flommastMonitorRoutes := router.Group("/admin/flommast")
	flommastMonitorRoutes.Use(middleware.AuthMiddleware(cfg))
	flommastMonitorRoutes.Use(middleware.OfficerOrAdmin())
	flommastMonitorRoutes.Get("/sync-history", flommastSyncHandler.History)
	flommastMonitorRoutes.Get("/sync-status", flommastSyncHandler.Status)

	// Phase 7 (Committee Members — designation management): Officer/Admin
	committeeAdminRoutes := router.Group("/admin/committee")
	committeeAdminRoutes.Use(middleware.AuthMiddleware(cfg))
	committeeAdminRoutes.Use(middleware.OfficerOrAdmin())
	committeeAdminRoutes.Post("/members", audit("committee.add_member"), committeeHandler.AddMember)
	committeeAdminRoutes.Get("/members", committeeHandler.ListMembers)
	committeeAdminRoutes.Delete("/members/:id", audit("committee.remove_member"), committeeHandler.RemoveMember)
	committeeAdminRoutes.Get("/visibility", committeeHandler.GetVisibility)
	committeeAdminRoutes.Put("/visibility", audit("committee.update_visibility"), committeeHandler.UpdateVisibility)
	committeeAdminRoutes.Get("/pdpa-settings", committeeHandler.GetPDPASettings)
	committeeAdminRoutes.Put("/pdpa-settings", audit("committee.update_pdpa"), committeeHandler.UpdatePDPASettings)

	// Phase 7 (Committee Members — viewer endpoints): any authenticated member,
	// authorization (is active committee member) is checked inside the service.
	committeeViewerRoutes := router.Group("/committee")
	committeeViewerRoutes.Use(middleware.AuthMiddleware(cfg))
	committeeViewerRoutes.Get("/me", committeeHandler.IsCommitteeMember)
	committeeViewerRoutes.Get("/borrowers", audit("committee.view_borrowers"), committeeHandler.ListBorrowersByMonth)
	committeeViewerRoutes.Get("/pdpa-status", committeeHandler.GetPDPAStatus)
}

// setupAuthRoutes configures authentication routes
func setupAuthRoutes(router fiber.Router, handler *handlers.AuthHandler, cfg *config.Config) {
	router.Post("/register", middleware.AuthRateLimiter(), audit("auth.register"), handler.Register)
	router.Post("/login", middleware.AuthRateLimiter(), audit("auth.login"), handler.Login)
	router.Post("/refresh", middleware.RefreshRateLimiter(), handler.RefreshToken)
	router.Post("/logout", audit("auth.logout"), handler.Logout)
	router.Get("/me", middleware.AuthMiddleware(cfg), handler.Me)
	router.Post("/logout-all", middleware.AuthMiddleware(cfg), audit("auth.logout_all"), handler.LogoutAll)
}

// setupLIFFRoutes configures LIFF routes
func setupLIFFRoutes(router fiber.Router, handler *handlers.LIFFHandler) {
	router.Post("/check", middleware.AuthRateLimiter(), handler.CheckLineUser)
	router.Post("/otp/request", middleware.StrictRateLimiter(), audit("auth.otp_request"), handler.RequestOTP)
	router.Post("/otp/verify", middleware.StrictRateLimiter(), audit("auth.otp_verify"), handler.VerifyOTP)
	router.Post("/register", middleware.StrictRateLimiter(), audit("auth.liff_register"), handler.Register)
	router.Post("/login", middleware.AuthRateLimiter(), audit("auth.liff_login"), handler.LoginWithLiff)
}

// setupUserRoutes configures user management routes
// list: Officer/Admin (หน้า Mortgages ใช้เลือกเจ้าหน้าที่) — อย่างอื่น Admin only
func setupUserRoutes(router fiber.Router, handler *handlers.UserHandler) {
	router.Get("/", middleware.OfficerOrAdmin(), audit("user.list"), handler.ListUsers)
	router.Get("/:id", middleware.AdminOnly(), audit("user.view"), handler.GetUser)
	router.Put("/:id", middleware.AdminOnly(), audit("user.update"), handler.UpdateUser)
	router.Delete("/:id", middleware.AdminOnly(), audit("user.delete"), handler.DeleteUser)
	router.Put("/:id/role", middleware.AdminOnly(), audit("user.set_role"), handler.SetUserRole)
}

// setupProfileRoutes configures profile routes (Authenticated)
func setupProfileRoutes(router fiber.Router, handler *handlers.UserHandler) {
	router.Get("/", handler.GetProfile)
	router.Put("/", audit("profile.update"), handler.UpdateProfile)
	router.Put("/password", audit("profile.change_password"), handler.ChangePassword)
}

// setupMortgageRoutes configures mortgage routes (Phase 4)
func setupMortgageRoutes(router fiber.Router, handler *handlers.MortgageHandler, cfg *config.Config) {
	router.Get("/my", handler.GetMyMortgages)
	router.Put("/:id/consent", audit("mortgage.consent"), handler.SetConsent)

	officerRoutes := router.Group("")
	officerRoutes.Use(middleware.OfficerOrAdmin())
	officerRoutes.Post("/", audit("mortgage.create"), handler.Create)
	officerRoutes.Get("/", audit("mortgage.list"), handler.List)
	officerRoutes.Get("/:id", audit("mortgage.view"), handler.GetByID)
	officerRoutes.Get("/:id/history", handler.GetHistory)
	officerRoutes.Get("/:id/docs", handler.GetDocs)
	officerRoutes.Put("/:id/docs", audit("mortgage.update_doc"), handler.UpdateDoc)
	officerRoutes.Get("/:id/appts", handler.GetAppts)
	officerRoutes.Post("/:id/appts", audit("mortgage.create_appt"), handler.CreateAppt)
	officerRoutes.Put("/:id/appts/:appt_id/complete", audit("mortgage.complete_appt"), handler.CompleteAppt)
	officerRoutes.Put("/:id/step", audit("mortgage.change_step"), handler.ChangeStep)
	officerRoutes.Put("/:id/approve", audit("mortgage.approve"), handler.Approve)
	officerRoutes.Put("/:id/reject", audit("mortgage.reject"), handler.Reject)
	officerRoutes.Put("/:id/officer", audit("mortgage.change_officer"), handler.ChangeOfficer)
	officerRoutes.Put("/:id/amount", audit("mortgage.update_amount"), handler.UpdateAmount)
}

// setupMasterRoutes configures master data routes (Phase 4)
// read: ทุกคนที่ login (dropdown) — create/update/delete: Admin only
func setupMasterRoutes(router fiber.Router, handler *handlers.MasterHandler) {
	router.Get("/loan-types", handler.ListLoanTypes)
	router.Get("/loan-types/:id", handler.GetLoanType)
	router.Post("/loan-types", middleware.AdminOnly(), audit("master.change"), handler.CreateLoanType)
	router.Put("/loan-types/:id", middleware.AdminOnly(), audit("master.change"), handler.UpdateLoanType)
	router.Delete("/loan-types/:id", middleware.AdminOnly(), audit("master.change"), handler.DeleteLoanType)

	router.Get("/loan-steps", handler.ListLoanSteps)
	router.Get("/loan-steps/:id", handler.GetLoanStep)
	router.Post("/loan-steps", middleware.AdminOnly(), audit("master.change"), handler.CreateLoanStep)
	router.Put("/loan-steps/:id", middleware.AdminOnly(), audit("master.change"), handler.UpdateLoanStep)
	router.Delete("/loan-steps/:id", middleware.AdminOnly(), audit("master.change"), handler.DeleteLoanStep)

	router.Get("/loan-docs", handler.ListLoanDocs)
	router.Get("/loan-docs/:id", handler.GetLoanDoc)
	router.Post("/loan-docs", middleware.AdminOnly(), audit("master.change"), handler.CreateLoanDoc)
	router.Put("/loan-docs/:id", middleware.AdminOnly(), audit("master.change"), handler.UpdateLoanDoc)
	router.Delete("/loan-docs/:id", middleware.AdminOnly(), audit("master.change"), handler.DeleteLoanDoc)

	router.Get("/loan-appts", handler.ListLoanAppts)
	router.Get("/loan-appts/:id", handler.GetLoanAppt)
	router.Post("/loan-appts", middleware.AdminOnly(), audit("master.change"), handler.CreateLoanAppt)
	router.Put("/loan-appts/:id", middleware.AdminOnly(), audit("master.change"), handler.UpdateLoanAppt)
	router.Delete("/loan-appts/:id", middleware.AdminOnly(), audit("master.change"), handler.DeleteLoanAppt)
}

// setupDashboardRoutes configures dashboard routes (Phase 5)
func setupDashboardRoutes(router fiber.Router, handler *handlers.DashboardHandler) {
	router.Get("/", handler.GetMyDashboard)
	router.Get("/user", handler.GetUserDashboard)
	router.Get("/officer", middleware.OfficerOrAdmin(), handler.GetOfficerDashboard)
	router.Get("/admin", middleware.AdminOnly(), handler.GetAdminDashboard)
}

// setupAPIV2Routes configures API v2 routes (Mobile-optimized)
func setupAPIV2Routes(router fiber.Router, mobileHandler *handlers.MobileHandler, cfg *config.Config) {
	mobileRoutes := router.Group("/mobile")
	mobileRoutes.Use(middleware.AuthMiddleware(cfg))
	mobileRoutes.Get("/dashboard", mobileHandler.GetDashboard)
	mobileRoutes.Get("/my-loans", mobileHandler.GetMyLoans)
	mobileRoutes.Get("/master", mobileHandler.GetMasterData)
}

// ============================================================
// Phase 6: Doc Items & Doc Checks routes
// ============================================================

// setupDocItemRoutes configures doc item master data routes
func setupDocItemRoutes(router fiber.Router, handler *handlers.DocCheckHandler) {
	router.Get("/doc-items", handler.ListDocItems)
	router.Get("/doc-items/:id", handler.GetDocItem)
	router.Post("/doc-items", middleware.AdminOnly(), audit("master.change"), handler.CreateDocItem)
	router.Put("/doc-items/:id", middleware.AdminOnly(), audit("master.change"), handler.UpdateDocItem)
	router.Delete("/doc-items/:id", middleware.AdminOnly(), audit("master.change"), handler.DeleteDocItem)
}

// setupDocCheckRoutes configures mortgage doc check routes
func setupDocCheckRoutes(router fiber.Router, handler *handlers.DocCheckHandler) {
	docCheckRoutes := router.Group("/:id/doc-checks")
	docCheckRoutes.Use(middleware.OfficerOrAdmin())
	docCheckRoutes.Get("/", handler.GetDocChecks)
	docCheckRoutes.Put("/", audit("doccheck.update"), handler.UpdateDocChecks)
	docCheckRoutes.Get("/incomplete", handler.GetIncompleteDoc)
	docCheckRoutes.Post("/notify-line", audit("doccheck.notify_line"), handler.NotifyLineIncompleteDoc)
}

// ============================================================
// Phase 1 (Loan Print): Officer + Admin
// ============================================================

// setupLoanPrintRoutes configures loan-print endpoints (search members, get full data, list purposes)
func setupLoanPrintRoutes(router fiber.Router, handler *handlers.LoanPrintHandler) {
	router.Get("/members/search", audit("member.search"), handler.SearchMembers)
	router.Get("/members/:memb_no", audit("member.view"), handler.GetMember)
	router.Get("/purposes", handler.ListPurposes)

	// Phase 3a: Auto-numbering
	router.Get("/next-number", handler.PeekNextNumber)
	router.Post("/issue-number", audit("loanprint.issue_number"), handler.IssueNextNumber)
	// Phase 3b: Collateral endpoint
	router.Get("/collateral/:memb_no", audit("member.view_collateral"), handler.GetCollateral)
}

// ============================================================
// Phase 1 (Flommast Import): Admin only
// ============================================================

// setupFlommastImportRoutes configures admin endpoints for uploading flommast .sql files
func setupFlommastImportRoutes(router fiber.Router, handler *handlers.FlommastImportHandler) {
	router.Post("/preview", audit("flommast.preview"), handler.Preview)
	router.Post("/apply", audit("flommast.apply"), handler.Apply)
}
