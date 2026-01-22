package services

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"spsc-loaneasy/internal/adapters/persistence/models"
)

// NotificationService handles LINE notifications
type NotificationService struct {
	lineNotifyToken string
	enabled         bool
}

// NewNotificationService creates a new notification service
func NewNotificationService() *NotificationService {
	token := os.Getenv("LINE_NOTIFY_TOKEN")
	return &NotificationService{
		lineNotifyToken: token,
		enabled:         token != "",
	}
}

// IsEnabled checks if notification is enabled
func (s *NotificationService) IsEnabled() bool {
	return s.enabled
}

// sendLineNotify sends a message via LINE Notify
func (s *NotificationService) sendLineNotify(message string) error {
	if !s.enabled {
		return nil
	}

	data := url.Values{}
	data.Set("message", message)

	req, err := http.NewRequest("POST", "https://notify-api.line.me/api/notify", bytes.NewBufferString(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+s.lineNotifyToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// NotifyNewMortgage sends notification for new mortgage
func (s *NotificationService) NotifyNewMortgage(mortgage *models.Mortgage, memberName string) {
	message := fmt.Sprintf(`
🆕 คำขอสินเชื่อใหม่

📋 รหัส: #%d
👤 สมาชิก: %s (%s)
💰 จำนวนเงิน: %.2f บาท
📝 วัตถุประสงค์: %s

กรุณาตรวจสอบเอกสาร`,
		mortgage.ID,
		memberName,
		mortgage.MembNo,
		mortgage.Amount,
		mortgage.Purpose,
	)

	s.sendLineNotify(message)
}

// NotifyStatusChange sends notification for status change
func (s *NotificationService) NotifyStatusChange(mortgage *models.Mortgage, newStepName string) {
	message := fmt.Sprintf(`
🔄 เปลี่ยนสถานะ

📋 รหัส: #%d
👤 สมาชิก: %s
📊 สถานะใหม่: %s`,
		mortgage.ID,
		mortgage.MembNo,
		newStepName,
	)

	s.sendLineNotify(message)
}

// NotifyApproved sends notification for approved mortgage
func (s *NotificationService) NotifyApproved(mortgage *models.Mortgage) {
	contractNo := ""
	if mortgage.ContractNo != nil {
		contractNo = *mortgage.ContractNo
	}

	message := fmt.Sprintf(`
✅ อนุมัติสินเชื่อ

📋 เลขสัญญา: %s
👤 สมาชิก: %s
💰 จำนวนเงิน: %.2f บาท

กรุณานัดหมายรับเงิน`,
		contractNo,
		mortgage.MembNo,
		mortgage.Amount,
	)

	s.sendLineNotify(message)
}

// NotifyRejected sends notification for rejected mortgage
func (s *NotificationService) NotifyRejected(mortgage *models.Mortgage, reason string) {
	message := fmt.Sprintf(`
❌ ปฏิเสธสินเชื่อ

📋 รหัส: #%d
👤 สมาชิก: %s
📝 เหตุผล: %s`,
		mortgage.ID,
		mortgage.MembNo,
		reason,
	)

	s.sendLineNotify(message)
}

// NotifyNewAppointment sends notification for new appointment
func (s *NotificationService) NotifyNewAppointment(mortgage *models.Mortgage, apptType string, apptDate string) {
	message := fmt.Sprintf(`
📅 นัดหมายใหม่

📋 รหัส: #%d
👤 สมาชิก: %s
📌 ประเภท: %s
📆 วันที่: %s`,
		mortgage.ID,
		mortgage.MembNo,
		apptType,
		apptDate,
	)

	s.sendLineNotify(message)
}

// NotifyUpcomingAppointment sends notification for upcoming appointment
func (s *NotificationService) NotifyUpcomingAppointment(mortgage *models.Mortgage, apptType string, apptDate string, location string) {
	message := fmt.Sprintf(`
⏰ แจ้งเตือนนัดหมาย

📋 รหัส: #%d
👤 สมาชิก: %s
📌 ประเภท: %s
📆 วันที่: %s
📍 สถานที่: %s`,
		mortgage.ID,
		mortgage.MembNo,
		apptType,
		apptDate,
		location,
	)

	s.sendLineNotify(message)
}

// NotifyDocumentComplete sends notification when all documents are submitted
func (s *NotificationService) NotifyDocumentComplete(mortgage *models.Mortgage) {
	message := fmt.Sprintf(`
📄 เอกสารครบถ้วน

📋 รหัส: #%d
👤 สมาชิก: %s

พร้อมดำเนินการขั้นตอนถัดไป`,
		mortgage.ID,
		mortgage.MembNo,
	)

	s.sendLineNotify(message)
}
