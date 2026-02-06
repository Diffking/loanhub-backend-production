package services

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// CronService handles scheduled tasks
type CronService struct {
	db          *gorm.DB
	cron        *cron.Cron
	lineService *LINEService
}

// AppointmentReminder represents appointment data for reminder
type AppointmentReminder struct {
	MembNo          string    `json:"memb_no"`
	FullName        string    `json:"full_name"`
	LineUserID      string    `json:"line_user_id"`
	LineDisplayName string    `json:"line_display_name"`
	ApptDate        time.Time `json:"appt_date"`
	ApptTime        string    `json:"appt_time"`
	Location        string    `json:"location"`
	ApptType        string    `json:"appt_type"`
}

// NewCronService creates a new cron service
func NewCronService(db *gorm.DB) *CronService {
	// Create cron with Bangkok timezone
	location, _ := time.LoadLocation("Asia/Bangkok")
	c := cron.New(cron.WithLocation(location))

	channelID := os.Getenv("LINE_CHANNEL_ID")
	channelSecret := os.Getenv("LINE_CHANNEL_SECRET")
	callbackURL := os.Getenv("LINE_CALLBACK_URL")

	return &CronService{
		db:          db,
		cron:        c,
		lineService: NewLINEService(db, channelID, channelSecret, callbackURL),
	}
}

// Start starts the cron scheduler
func (s *CronService) Start() {
	// Send appointment reminders at 08:30 every day
	_, err := s.cron.AddFunc("30 8 * * *", func() {
		log.Println("🔔 Running appointment reminder job...")
		s.SendAppointmentReminders()
	})
	if err != nil {
		log.Printf("❌ Failed to add cron job: %v", err)
		return
	}

	s.cron.Start()
	log.Println("✅ Cron scheduler started (Appointment reminders at 08:30)")
}

// Stop stops the cron scheduler
func (s *CronService) Stop() {
	s.cron.Stop()
	log.Println("🛑 Cron scheduler stopped")
}

// SendAppointmentReminders sends LINE reminders for tomorrow's appointments
func (s *CronService) SendAppointmentReminders() {
	// Get tomorrow's date
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	log.Printf("📅 Checking appointments for: %s", tomorrow)

	// Query appointments for tomorrow from mortgages table where:
	// 1. User has linked LINE account
	// 2. Has appointment date set
	var appointments []AppointmentReminder

	query := `
		SELECT 
			u.memb_no,
			COALESCE(f.full_name, u.username) as full_name,
			u.line_user_id,
			u.line_display_name,
			m.appt_date,
			m.appt_time,
			m.appt_location as location,
			COALESCE(la.name, 'นัดหมาย') as appt_type
		FROM mortgages m
		JOIN users u ON m.memb_no = u.memb_no
		LEFT JOIN flommast f ON u.memb_no = f.mast_memb_no
		LEFT JOIN loan_appts la ON m.current_appt_id = la.id
		WHERE DATE(m.appt_date) = ?
		AND m.deleted_at IS NULL
		AND u.line_user_id IS NOT NULL
		AND u.line_user_id != ''
	`

	result := s.db.Raw(query, tomorrow).Scan(&appointments)
	if result.Error != nil {
		log.Printf("❌ Failed to query appointments: %v", result.Error)
		return
	}

	log.Printf("📋 Found %d appointments with LINE linked", len(appointments))

	if len(appointments) == 0 {
		log.Println("✅ No appointments to remind")
		return
	}

	// Get Messaging API Channel Access Token
	channelAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken == "" {
		log.Println("❌ LINE_CHANNEL_ACCESS_TOKEN not set")
		return
	}

	webURL := os.Getenv("WEB_APP_URL")
	if webURL == "" {
		webURL = "https://loanspsc.com"
	}

	// Send reminders
	successCount := 0
	failCount := 0

	for _, appt := range appointments {
		// Format date in Thai
		apptDateStr := appt.ApptDate.Format("02/01/2006")
		apptTimeStr := appt.ApptTime
		if apptTimeStr == "" {
			apptTimeStr = "กรุณาตรวจสอบในระบบ"
		}

		// Create flex message
		flexContent := s.lineService.CreateAppointmentReminder(
			appt.FullName,
			apptDateStr,
			apptTimeStr,
			appt.Location,
			webURL,
		)

		// Send flex message
		err := s.lineService.SendFlexMessage(appt.LineUserID, flexContent, channelAccessToken)
		if err != nil {
			log.Printf("❌ Failed to send to %s (%s): %v", appt.MembNo, appt.LineDisplayName, err)
			failCount++

			// Fallback: send simple text message
			simpleMsg := fmt.Sprintf(
				"📅 แจ้งเตือนนัดหมาย\n\nสวัสดีคุณ %s\nคุณมีนัดหมายกับสหกรณ์ในวันพรุ่งนี้\n\n📆 วันที่: %s\n⏰ เวลา: %s\n\n🔗 เข้าดูรายละเอียดได้ที่: %s",
				appt.FullName,
				apptDateStr,
				apptTimeStr,
				webURL,
			)

			errSimple := s.lineService.SendPushMessage(appt.LineUserID, simpleMsg, channelAccessToken)
			if errSimple != nil {
				log.Printf("❌ Failed to send simple message: %v", errSimple)
			} else {
				log.Printf("✅ Sent simple message to %s", appt.MembNo)
				successCount++
				failCount--
			}
		} else {
			log.Printf("✅ Sent reminder to %s (%s)", appt.MembNo, appt.LineDisplayName)
			successCount++
		}
	}

	log.Printf("📊 Reminder summary: %d success, %d failed", successCount, failCount)
}

// SendTestReminder sends a test reminder to a specific LINE user (for testing)
func (s *CronService) SendTestReminder(lineUserID, memberName string) error {
	channelAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken == "" {
		return fmt.Errorf("LINE_CHANNEL_ACCESS_TOKEN not set")
	}

	webURL := os.Getenv("WEB_APP_URL")
	if webURL == "" {
		webURL = "https://loanspsc.com"
	}

	tomorrow := time.Now().AddDate(0, 0, 1).Format("02/01/2006")

	flexContent := s.lineService.CreateAppointmentReminder(
		memberName,
		tomorrow,
		"10:00 น.",
		"สำนักงานสหกรณ์",
		webURL,
	)

	return s.lineService.SendFlexMessage(lineUserID, flexContent, channelAccessToken)
}

// ManualTrigger manually triggers the appointment reminder (for testing)
func (s *CronService) ManualTrigger() {
	log.Println("🔔 Manual trigger: Running appointment reminder...")
	s.SendAppointmentReminders()
}
