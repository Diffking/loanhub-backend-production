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

	// Phase 7: Committee handler
	committeeHandler := handlers.NewCommitteeHandler(committeeService)

	// ============================================================
	// LIFF Handler v3 - รับ lineService + otpService + smsService
	// (lineHandler/lineService constructed earlier, above MortgageService)
	// ============================================================
	otpService := services.NewOTPService(db)
	smsService := services.NewSMSService(lineService)
	liffHandler := handlers.NewLIFFHandler(db, lineService, otpService, smsService)

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
	app.Get("/swagger/*", swagger.HandlerDefault)

	// API v1 group
	apiV1 := app.Group("/api/v1")
	setupAPIV1Routes(apiV1, healthHandler, authHandler, userHandler, mortgageHandler,
		masterHandler, dashboardHandler, lineHandler, liffHandler, docCheckHandler,
		loanPrintHandler, flommastImportHandler, flommastSyncHandler, committeeHandler, cfg)

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
	cfg *config.Config,
) {
	// API Info
	router.Get("/", healthHandler.APIInfo)

	// Auth routes (public)
	authRoutes := router.Group("/auth")
	setupAuthRoutes(authRoutes, authHandler, cfg)

	// LINE routes
	lineRoutes := router.Group("/auth/line")
	setupLINERoutes(lineRoutes, lineHandler, cfg)

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
		flommastSyncHandler.Sync,
	)

	// Phase 1 (Flommast Import): Admin only
	flommastAdminRoutes := router.Group("/admin/flommast")
	flommastAdminRoutes.Use(middleware.AuthMiddleware(cfg))
	flommastAdminRoutes.Use(middleware.AdminOnly())
	setupFlommastImportRoutes(flommastAdminRoutes, flommastImportHandler)

	// Phase 3A (Missing members — list + bulk delete): JWT/Admin
	flommastAdminRoutes.Get("/missing", flommastSyncHandler.Missing)
	flommastAdminRoutes.Delete("/missing", flommastSyncHandler.DeleteMissing)

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
	committeeAdminRoutes.Post("/members", committeeHandler.AddMember)
	committeeAdminRoutes.Get("/members", committeeHandler.ListMembers)
	committeeAdminRoutes.Delete("/members/:id", committeeHandler.RemoveMember)
	committeeAdminRoutes.Get("/visibility", committeeHandler.GetVisibility)
	committeeAdminRoutes.Put("/visibility", committeeHandler.UpdateVisibility)
	committeeAdminRoutes.Get("/pdpa-settings", committeeHandler.GetPDPASettings)
	committeeAdminRoutes.Put("/pdpa-settings", committeeHandler.UpdatePDPASettings)

	// Phase 7 (Committee Members — viewer endpoints): any authenticated member,
	// authorization (is active committee member) is checked inside the service.
	committeeViewerRoutes := router.Group("/committee")
	committeeViewerRoutes.Use(middleware.AuthMiddleware(cfg))
	committeeViewerRoutes.Get("/me", committeeHandler.IsCommitteeMember)
	committeeViewerRoutes.Get("/borrowers", committeeHandler.ListBorrowersByMonth)
	committeeViewerRoutes.Get("/pdpa-status", committeeHandler.GetPDPAStatus)
}

// setupAuthRoutes configures authentication routes
func setupAuthRoutes(router fiber.Router, handler *handlers.AuthHandler, cfg *config.Config) {
	router.Post("/register", handler.Register)
	router.Post("/login", handler.Login)
	router.Post("/refresh", handler.RefreshToken)
	router.Post("/logout", handler.Logout)
	router.Get("/me", middleware.AuthMiddleware(cfg), handler.Me)
	router.Post("/logout-all", middleware.AuthMiddleware(cfg), handler.LogoutAll)
}

// setupLINERoutes configures LINE authentication routes
func setupLINERoutes(router fiber.Router, handler *handlers.LINEHandler, cfg *config.Config) {
	router.Get("/url", handler.GetLINELoginURL)
	router.Get("/callback", handler.LINECallback)
	router.Post("/link", middleware.AuthMiddleware(cfg), handler.LinkLINE)
	router.Post("/unlink", middleware.AuthMiddleware(cfg), handler.UnlinkLINE)
	router.Get("/status", middleware.AuthMiddleware(cfg), handler.GetLINEStatus)
}

// setupLIFFRoutes configures LIFF routes
func setupLIFFRoutes(router fiber.Router, handler *handlers.LIFFHandler) {
	router.Post("/check", middleware.AuthRateLimiter(), handler.CheckLineUser)
	router.Post("/otp/request", middleware.StrictRateLimiter(), handler.RequestOTP)
	router.Post("/otp/verify", middleware.StrictRateLimiter(), handler.VerifyOTP)
	router.Post("/register", middleware.StrictRateLimiter(), handler.Register)
	router.Post("/login", middleware.AuthRateLimiter(), handler.LoginWithLiff)
}

// setupUserRoutes configures user management routes (Admin only)
func setupUserRoutes(router fiber.Router, handler *handlers.UserHandler) {
	router.Get("/", handler.ListUsers)
	router.Get("/:id", handler.GetUser)
	router.Put("/:id", handler.UpdateUser)
	router.Delete("/:id", handler.DeleteUser)
	router.Put("/:id/role", handler.SetUserRole)
}

// setupProfileRoutes configures profile routes (Authenticated)
func setupProfileRoutes(router fiber.Router, handler *handlers.UserHandler) {
	router.Get("/", handler.GetProfile)
	router.Put("/", handler.UpdateProfile)
	router.Put("/password", handler.ChangePassword)
}

// setupMortgageRoutes configures mortgage routes (Phase 4)
func setupMortgageRoutes(router fiber.Router, handler *handlers.MortgageHandler, cfg *config.Config) {
	router.Get("/my", handler.GetMyMortgages)
	router.Put("/:id/consent", handler.SetConsent)

	officerRoutes := router.Group("")
	officerRoutes.Use(middleware.OfficerOrAdmin())
	officerRoutes.Post("/", handler.Create)
	officerRoutes.Get("/", handler.List)
	officerRoutes.Get("/:id", handler.GetByID)
	officerRoutes.Get("/:id/history", handler.GetHistory)
	officerRoutes.Get("/:id/docs", handler.GetDocs)
	officerRoutes.Put("/:id/docs", handler.UpdateDoc)
	officerRoutes.Get("/:id/appts", handler.GetAppts)
	officerRoutes.Post("/:id/appts", handler.CreateAppt)
	officerRoutes.Put("/:id/appts/:appt_id/complete", handler.CompleteAppt)
	officerRoutes.Put("/:id/step", handler.ChangeStep)
	officerRoutes.Put("/:id/approve", handler.Approve)
	officerRoutes.Put("/:id/reject", handler.Reject)
	officerRoutes.Put("/:id/officer", handler.ChangeOfficer)
	officerRoutes.Put("/:id/amount", handler.UpdateAmount)
}

// setupMasterRoutes configures master data routes (Phase 4)
func setupMasterRoutes(router fiber.Router, handler *handlers.MasterHandler) {
	router.Get("/loan-types", handler.ListLoanTypes)
	router.Get("/loan-types/:id", handler.GetLoanType)
	router.Post("/loan-types", handler.CreateLoanType)
	router.Put("/loan-types/:id", handler.UpdateLoanType)
	router.Delete("/loan-types/:id", handler.DeleteLoanType)

	router.Get("/loan-steps", handler.ListLoanSteps)
	router.Get("/loan-steps/:id", handler.GetLoanStep)
	router.Post("/loan-steps", handler.CreateLoanStep)
	router.Put("/loan-steps/:id", handler.UpdateLoanStep)
	router.Delete("/loan-steps/:id", handler.DeleteLoanStep)

	router.Get("/loan-docs", handler.ListLoanDocs)
	router.Get("/loan-docs/:id", handler.GetLoanDoc)
	router.Post("/loan-docs", handler.CreateLoanDoc)
	router.Put("/loan-docs/:id", handler.UpdateLoanDoc)
	router.Delete("/loan-docs/:id", handler.DeleteLoanDoc)

	router.Get("/loan-appts", handler.ListLoanAppts)
	router.Get("/loan-appts/:id", handler.GetLoanAppt)
	router.Post("/loan-appts", handler.CreateLoanAppt)
	router.Put("/loan-appts/:id", handler.UpdateLoanAppt)
	router.Delete("/loan-appts/:id", handler.DeleteLoanAppt)
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
	router.Post("/doc-items", handler.CreateDocItem)
	router.Put("/doc-items/:id", handler.UpdateDocItem)
	router.Delete("/doc-items/:id", handler.DeleteDocItem)
}

// setupDocCheckRoutes configures mortgage doc check routes
func setupDocCheckRoutes(router fiber.Router, handler *handlers.DocCheckHandler) {
	docCheckRoutes := router.Group("/:id/doc-checks")
	docCheckRoutes.Use(middleware.OfficerOrAdmin())
	docCheckRoutes.Get("/", handler.GetDocChecks)
	docCheckRoutes.Put("/", handler.UpdateDocChecks)
	docCheckRoutes.Get("/incomplete", handler.GetIncompleteDoc)
	docCheckRoutes.Post("/notify-line", handler.NotifyLineIncompleteDoc)
}

// ============================================================
// Phase 1 (Loan Print): Officer + Admin
// ============================================================

// setupLoanPrintRoutes configures loan-print endpoints (search members, get full data, list purposes)
func setupLoanPrintRoutes(router fiber.Router, handler *handlers.LoanPrintHandler) {
	router.Get("/members/search", handler.SearchMembers)
	router.Get("/members/:memb_no", handler.GetMember)
	router.Get("/purposes", handler.ListPurposes)

	// Phase 3a: Auto-numbering
	router.Get("/next-number", handler.PeekNextNumber)
	router.Post("/issue-number", handler.IssueNextNumber)
	// Phase 3b: Collateral endpoint
	router.Get("/collateral/:memb_no", handler.GetCollateral)
}

// ============================================================
// Phase 1 (Flommast Import): Admin only
// ============================================================

// setupFlommastImportRoutes configures admin endpoints for uploading flommast .sql files
func setupFlommastImportRoutes(router fiber.Router, handler *handlers.FlommastImportHandler) {
	router.Post("/preview", handler.Preview)
	router.Post("/apply", handler.Apply)
}
