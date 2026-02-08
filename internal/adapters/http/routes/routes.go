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

	// Phase 2: Queue repository
	queueRepo := repositories.NewQueueRepository(db)

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
	)

	// Phase 5: Dashboard service
	dashboardService := services.NewDashboardService(db)

	// Phase 2: Queue service
	queueService := services.NewQueueService(queueRepo)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	authHandler := handlers.NewAuthHandler(authService, cfg)
	userHandler := handlers.NewUserHandler(userService)

	// Phase 4: Handlers
	mortgageHandler := handlers.NewMortgageHandler(mortgageService)
	masterHandler := handlers.NewMasterHandler(loanTypeRepo, loanStepRepo, loanDocRepo, loanApptRepo)

	// Phase 5: Dashboard handler
	dashboardHandler := handlers.NewDashboardHandler(dashboardService)

	// Phase 2: Queue handlers
	queueHandler := handlers.NewQueueHandler(queueService)
	queueAdminHandler := handlers.NewQueueAdminHandler(queueService)

	// LINE Handler
	lineHandler := handlers.NewLINEHandler(db)

	// ============================================================
	// ✅ LIFF Handler v2 - รับ lineService + otpService
	// ============================================================
	lineService := lineHandler.GetLINEService()
	otpService := services.NewOTPService(db)
	liffHandler := handlers.NewLIFFHandler(db, lineService, otpService)

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

	// Health check & root routes
	app.Get("/", healthHandler.Root)
	app.Get("/health", healthHandler.HealthCheck)

	// Swagger documentation
	app.Get("/swagger/*", swagger.HandlerDefault)

	// API v1 group
	apiV1 := app.Group("/api/v1")
	setupAPIV1Routes(apiV1, healthHandler, authHandler, userHandler, mortgageHandler,
		masterHandler, dashboardHandler, lineHandler, liffHandler,
		queueHandler, queueAdminHandler, cfg)

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
	queueHandler *handlers.QueueHandler,
	queueAdminHandler *handlers.QueueAdminHandler,
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

	// Phase 4: Master routes (Admin only)
	masterRoutes := router.Group("/master")
	masterRoutes.Use(middleware.AuthMiddleware(cfg))
	setupMasterRoutes(masterRoutes, masterHandler)

	// Phase 5: Dashboard routes
	dashboardRoutes := router.Group("/dashboard")
	dashboardRoutes.Use(middleware.AuthMiddleware(cfg))
	setupDashboardRoutes(dashboardRoutes, dashboardHandler)

	// Phase 2: Queue routes (Authenticated users)
	queueRoutes := router.Group("/queue")
	queueRoutes.Use(middleware.AuthMiddleware(cfg))
	setupQueueRoutes(queueRoutes, queueHandler)

	// Phase 2: Queue admin routes (Officer/Admin only)
	queueAdminRoutes := router.Group("/admin/queue")
	queueAdminRoutes.Use(middleware.AuthMiddleware(cfg))
	queueAdminRoutes.Use(middleware.OfficerOrAdmin())
	setupQueueAdminRoutes(queueAdminRoutes, queueAdminHandler)
}

// setupAuthRoutes configures authentication routes
func setupAuthRoutes(router fiber.Router, handler *handlers.AuthHandler, cfg *config.Config) {
	// Public routes
	router.Post("/register", handler.Register)
	router.Post("/login", handler.Login)
	router.Post("/refresh", handler.RefreshToken)
	router.Post("/logout", handler.Logout)

	// Protected routes
	router.Get("/me", middleware.AuthMiddleware(cfg), handler.Me)
	router.Post("/logout-all", middleware.AuthMiddleware(cfg), handler.LogoutAll)
}

// setupLINERoutes configures LINE authentication routes
func setupLINERoutes(router fiber.Router, handler *handlers.LINEHandler, cfg *config.Config) {
	// PUBLIC - Get LINE Login URL (for login with LINE - no auth required)
	router.Get("/url", handler.GetLINELoginURL)

	// PUBLIC - LINE callback (LINE redirects here)
	router.Get("/callback", handler.LINECallback)

	// PROTECTED - Link LINE account (requires login first)
	router.Post("/link", middleware.AuthMiddleware(cfg), handler.LinkLINE)

	// PROTECTED - Unlink LINE account
	router.Post("/unlink", middleware.AuthMiddleware(cfg), handler.UnlinkLINE)

	// PROTECTED - Get LINE status
	router.Get("/status", middleware.AuthMiddleware(cfg), handler.GetLINEStatus)
}

// ============================================================
// ✅ LIFF Routes - เพิ่ม Rate Limiter ป้องกัน spam/brute force
//    StrictRateLimiter = 3 req/min/IP (OTP, register, device change)
//    AuthRateLimiter   = 5 req/min/IP (check, login, device info)
// ============================================================
func setupLIFFRoutes(router fiber.Router, handler *handlers.LIFFHandler) {
	// Check if LINE user exists in system (5 req/min/IP)
	router.Post("/check", middleware.AuthRateLimiter(), handler.CheckLineUser)

	// OTP routes (3 req/min/IP — ป้องกัน OTP spam + brute force)
	router.Post("/otp/request", middleware.StrictRateLimiter(), handler.RequestOTP)
	router.Post("/otp/verify", middleware.StrictRateLimiter(), handler.VerifyOTP)

	// Register - Link LINE with Member Number (3 req/min/IP)
	router.Post("/register", middleware.StrictRateLimiter(), handler.Register)

	// Login with LIFF (5 req/min/IP — อนุญาต WiFi)
	router.Post("/login", middleware.AuthRateLimiter(), handler.LoginWithLiff)

	// Device management
	router.Post("/device/change", middleware.StrictRateLimiter(), handler.ChangeDevice) // 3 req/min/IP
	router.Post("/device/info", middleware.AuthRateLimiter(), handler.GetDeviceInfo)     // 5 req/min/IP
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
	// Member can view their own mortgages
	router.Get("/my", handler.GetMyMortgages)

	// Officer/Admin routes
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

	// Admin only
	adminRoutes := router.Group("")
	adminRoutes.Use(middleware.AdminOnly())
	adminRoutes.Put("/:id/officer", handler.ChangeOfficer)
}

// setupMasterRoutes configures master data routes (Admin only) (Phase 4)
func setupMasterRoutes(router fiber.Router, handler *handlers.MasterHandler) {
	// Loan Types
	router.Get("/loan-types", handler.ListLoanTypes)
	router.Get("/loan-types/:id", handler.GetLoanType)
	router.Post("/loan-types", handler.CreateLoanType)
	router.Put("/loan-types/:id", handler.UpdateLoanType)
	router.Delete("/loan-types/:id", handler.DeleteLoanType)

	// Loan Steps
	router.Get("/loan-steps", handler.ListLoanSteps)
	router.Get("/loan-steps/:id", handler.GetLoanStep)
	router.Post("/loan-steps", handler.CreateLoanStep)
	router.Put("/loan-steps/:id", handler.UpdateLoanStep)
	router.Delete("/loan-steps/:id", handler.DeleteLoanStep)

	// Loan Docs
	router.Get("/loan-docs", handler.ListLoanDocs)
	router.Get("/loan-docs/:id", handler.GetLoanDoc)
	router.Post("/loan-docs", handler.CreateLoanDoc)
	router.Put("/loan-docs/:id", handler.UpdateLoanDoc)
	router.Delete("/loan-docs/:id", handler.DeleteLoanDoc)

	// Loan Appts
	router.Get("/loan-appts", handler.ListLoanAppts)
	router.Get("/loan-appts/:id", handler.GetLoanAppt)
	router.Post("/loan-appts", handler.CreateLoanAppt)
	router.Put("/loan-appts/:id", handler.UpdateLoanAppt)
	router.Delete("/loan-appts/:id", handler.DeleteLoanAppt)
}

// setupDashboardRoutes configures dashboard routes (Phase 5)
func setupDashboardRoutes(router fiber.Router, handler *handlers.DashboardHandler) {
	// Auto-detect role dashboard (All authenticated users)
	router.Get("/", handler.GetMyDashboard)

	// User dashboard (All authenticated users)
	router.Get("/user", handler.GetUserDashboard)

	// Officer dashboard (Officer/Admin only)
	router.Get("/officer", middleware.OfficerOrAdmin(), handler.GetOfficerDashboard)

	// Admin dashboard (Admin only)
	router.Get("/admin", middleware.AdminOnly(), handler.GetAdminDashboard)
}

// ============================================================
// Phase 2: Queue Routes
// ============================================================

// setupQueueRoutes configures queue routes for users (Phase 2)
func setupQueueRoutes(router fiber.Router, handler *handlers.QueueHandler) {
	// Branch info
	router.Get("/branches", handler.GetBranches)
	router.Get("/branches/:id/services", handler.GetBranchServices)
	router.Get("/branches/:id/status", handler.GetBranchStatus)

	// Walk-in
	router.Post("/walkin", handler.CreateWalkin)

	// My tickets
	router.Get("/my-tickets", handler.GetMyTickets)
	router.Get("/my-tickets/:id", handler.GetMyTicketByID)

	// Track by ticket number
	router.Get("/track/:ticket_number", handler.TrackTicket)
}

// setupQueueAdminRoutes configures queue admin routes for officers (Phase 2)
func setupQueueAdminRoutes(router fiber.Router, handler *handlers.QueueAdminHandler) {
	// Counter management
	router.Post("/counter/open", handler.OpenCounter)
	router.Post("/counter/close", handler.CloseCounter)
	router.Post("/counter/break", handler.BreakCounter)

	// Call & Serve
	router.Post("/call-next", handler.CallNext)
	router.Post("/call/:id", handler.CallSpecific)
	router.Post("/recall/:id", handler.Recall)
	router.Post("/serve/:id", handler.Serve)
	router.Post("/complete/:id", handler.Complete)
	router.Post("/skip/:id", handler.Skip)
	router.Post("/transfer/:id", handler.Transfer)

	// Dashboard & History
	router.Get("/dashboard", handler.Dashboard)
	router.Get("/history", handler.History)

	// Config
	router.Get("/config", handler.GetConfig)
	router.Put("/config", handler.UpdateConfig)
}

// setupAPIV2Routes configures API v2 routes (Mobile-optimized)
func setupAPIV2Routes(router fiber.Router, mobileHandler *handlers.MobileHandler, cfg *config.Config) {
	// Mobile routes group (requires authentication)
	mobileRoutes := router.Group("/mobile")
	mobileRoutes.Use(middleware.AuthMiddleware(cfg))

	// GET /api/v2/mobile/dashboard
	mobileRoutes.Get("/dashboard", mobileHandler.GetDashboard)

	// GET /api/v2/mobile/my-loans
	mobileRoutes.Get("/my-loans", mobileHandler.GetMyLoans)

	// GET /api/v2/mobile/master
	mobileRoutes.Get("/master", mobileHandler.GetMasterData)
}
