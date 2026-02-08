// ============================================================
// Phase 2: Queue Routes — เพิ่มใน routes.go
// ============================================================
//
// วิธีใช้:
// 1. เพิ่ม code ด้านล่างในไฟล์ routes.go ตามตำแหน่งที่ระบุ
// 2. ไม่ต้องลบอะไรเดิม — เพิ่มอย่างเดียว
//
// ============================================================

// ─── ส่วนที่ 1: เพิ่มใน func Setup() ─── (หลัง dashboardHandler)
// เพิ่มบรรทัดเหล่านี้ใน func Setup() ก่อนบรรทัด "// Health check & root routes"

/*
	// Phase 2: Queue repository + service + handlers
	queueRepo := repositories.NewQueueRepository(db)
	queueService := services.NewQueueService(queueRepo)
	queueHandler := handlers.NewQueueHandler(queueService)
	queueAdminHandler := handlers.NewQueueAdminHandler(queueService)
*/

// ─── ส่วนที่ 2: เพิ่ม parameter ใน setupAPIV1Routes() call ─── 
// แก้ตรง setupAPIV1Routes() call ใน func Setup() เป็น:

/*
	setupAPIV1Routes(apiV1, healthHandler, authHandler, userHandler, mortgageHandler,
		masterHandler, dashboardHandler, lineHandler, liffHandler,
		queueHandler, queueAdminHandler, cfg)
*/

// ─── ส่วนที่ 3: แก้ func signature ของ setupAPIV1Routes() ─── 
// เพิ่ม parameter 2 ตัว:

/*
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
	queueHandler *handlers.QueueHandler,           // ← เพิ่ม
	queueAdminHandler *handlers.QueueAdminHandler,  // ← เพิ่ม
	cfg *config.Config,
) {
*/

// ─── ส่วนที่ 4: เพิ่ม route groups ─── 
// เพิ่มก่อนปิด func setupAPIV1Routes() (หลัง setupDashboardRoutes)

/*
	// Phase 2: Queue routes (Authenticated users)
	queueRoutes := router.Group("/queue")
	queueRoutes.Use(middleware.AuthMiddleware(cfg))
	setupQueueRoutes(queueRoutes, queueHandler)

	// Phase 2: Queue admin routes (Officer/Admin only)
	queueAdminRoutes := router.Group("/admin/queue")
	queueAdminRoutes.Use(middleware.AuthMiddleware(cfg))
	queueAdminRoutes.Use(middleware.OfficerOrAdmin())
	setupQueueAdminRoutes(queueAdminRoutes, queueAdminHandler)
*/

// ─── ส่วนที่ 5: เพิ่ม 2 functions ใหม่ ─── 
// เพิ่มท้ายไฟล์ routes.go

/*
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

// setupQueueAdminRoutes configures queue admin routes (Phase 2)
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

	// Config (Admin only — เพิ่ม middleware ถ้าต้องการ)
	router.Get("/config", handler.GetConfig)
	router.Put("/config", handler.UpdateConfig)
}
*/
