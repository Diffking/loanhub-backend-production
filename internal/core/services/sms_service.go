package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ============================================================
// SMS Service - ส่ง OTP ผ่าน SMS (SMSMKT)
// ============================================================
// Provider หลัก: SMSMKT (https://smsmkt.com)
// Fallback: LINE Push Message (ถ้า SMS พัง หรือ dev mode)
//
// SMSMKT API Docs: https://developers.smsmkt.com/en/api-reference
// Base URL: https://portal-otp.smsmkt.com/api/
// Auth: Headers → api_key + secret_key
//
// Environment Variables ที่ต้องตั้ง:
//   SMS_PROVIDER=smsmkt
//   SMS_API_KEY=your_api_key
//   SMS_API_SECRET=your_secret_key
//   SMS_SENDER=SPSC (ชื่อผู้ส่งที่จดทะเบียนกับ SMSMKT)
// ============================================================

// SMSProvider interface สำหรับส่ง SMS
type SMSProvider interface {
	SendSMS(phone, message string) error
	Name() string
}

// SMSService จัดการการส่ง SMS
type SMSService struct {
	provider    SMSProvider
	devMode     bool
	lineService *LINEService // สำหรับ fallback ส่งผ่าน LINE
}

// NewSMSService creates a new SMS service
func NewSMSService(lineService *LINEService) *SMSService {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("SMS_PROVIDER")))
	apiKey := os.Getenv("SMS_API_KEY")
	apiSecret := os.Getenv("SMS_API_SECRET")
	sender := os.Getenv("SMS_SENDER")
	appMode := strings.TrimSpace(os.Getenv("APP_MODE"))

	if sender == "" {
		sender = "SPSC" // default sender name
	}

	svc := &SMSService{
		devMode:     appMode != "prod",
		lineService: lineService,
	}

	// เลือก SMS Provider ตาม config
	switch provider {
	case "smsmkt":
		if apiKey != "" && apiSecret != "" {
			svc.provider = NewSMSMKTProvider(apiKey, apiSecret, sender)
			log.Printf("📱 SMS Service: SMSMKT (sender: %s)", sender)
		} else {
			log.Printf("⚠️ SMS Service: SMSMKT API key not set - will use LINE fallback")
		}

	default:
		log.Printf("⚠️ SMS Service: No provider configured (SMS_PROVIDER=%s) - will use LINE fallback", provider)
	}

	return svc
}

// SendOTP ส่ง OTP ไปยังเบอร์โทร
func (s *SMSService) SendOTP(phone, otpCode, lineUserID string) error {
	message := fmt.Sprintf("รหัส OTP: %s (หมดอายุ 5 นาที) - สหกรณ์ SPSC อย่าบอกรหัสนี้แก่ผู้อื่น", otpCode)

	// 1. ถ้ามี SMS Provider → ส่ง SMS จริง
	if s.provider != nil {
		smsPhone := formatPhoneForSMS(phone)
		err := s.provider.SendSMS(smsPhone, message)
		if err != nil {
			log.Printf("❌ SMS send failed via %s: %v", s.provider.Name(), err)

			// Fallback ไปส่ง LINE ถ้า SMS พัง
			if s.lineService != nil && lineUserID != "" {
				log.Printf("🔄 Falling back to LINE push message")
				return s.sendViaLINE(lineUserID, message)
			}
			return fmt.Errorf("ไม่สามารถส่ง OTP ได้: %w", err)
		}

		log.Printf("✅ OTP sent via SMS (%s) to %s", s.provider.Name(), maskPhoneLog(phone))
		return nil
	}

	// 2. Dev mode หรือไม่มี provider → ส่งผ่าน LINE
	if s.lineService != nil && lineUserID != "" {
		if s.devMode {
			log.Printf("🧪 [DEV MODE] Sending OTP via LINE (no SMS provider configured)")
		} else {
			log.Printf("⚠️ [PROD] No SMS provider - falling back to LINE")
		}
		return s.sendViaLINE(lineUserID, message)
	}

	// 3. ไม่มีทางส่งเลย
	return fmt.Errorf("ไม่สามารถส่ง OTP ได้: ไม่มี SMS provider และ LINE")
}

// sendViaLINE ส่ง OTP ผ่าน LINE Push Message (fallback)
func (s *SMSService) sendViaLINE(lineUserID, message string) error {
	channelAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken == "" {
		return fmt.Errorf("LINE_CHANNEL_ACCESS_TOKEN not configured")
	}

	err := s.lineService.SendPushMessage(lineUserID, message, channelAccessToken)
	if err != nil {
		log.Printf("❌ LINE push failed: %v", err)
		return fmt.Errorf("ส่ง OTP ผ่าน LINE ไม่สำเร็จ: %w", err)
	}

	log.Printf("✅ OTP sent via LINE to user %s", lineUserID[:8]+"...")
	return nil
}

// IsReady ตรวจว่า SMS service พร้อมใช้งานหรือไม่
func (s *SMSService) IsReady() bool {
	return s.provider != nil
}

// GetProviderName ชื่อ provider ที่ใช้อยู่
func (s *SMSService) GetProviderName() string {
	if s.provider != nil {
		return s.provider.Name()
	}
	return "line_fallback"
}

// ============================================================
// SMSMKT Provider
// ============================================================
// API Docs: https://developers.smsmkt.com/en/api-reference
// Base URL: https://portal-otp.smsmkt.com/api/
//
// Authentication: HTTP Headers
//   api_key:    API Key จาก SMSMKT Console
//   secret_key: Secret Key จาก SMSMKT Console
//
// Endpoints:
//   POST /api/send-message   → ส่ง SMS ด้วยข้อความที่กำหนดเอง
//   POST /api/otp-send       → OTP สำเร็จรูป (SMSMKT สร้าง OTP ให้)
//   POST /api/otp-validate   → Verify OTP สำเร็จรูป
//   GET  /api/get-credit     → เช็คเครดิตคงเหลือ
//   GET  /api/get-estimate   → เช็คจำนวน SMS ที่ส่งได้
//
// Send Message Parameters (JSON body):
//   message  (required) - ข้อความ SMS (รองรับ UTF-8 / ภาษาไทย)
//   phone    (required) - เบอร์ปลายทาง 08xxxxxxxx (คั่นด้วย , ถ้าหลายเบอร์)
//   sender   (required) - ชื่อผู้ส่งที่จดทะเบียนไว้
//
// Response:
//   { "code": "000", "detail": "success", "result": { ... } }
//
// วิธีสมัคร:
//   1. สมัครที่ https://smsmkt.com
//   2. Login เข้า Portal
//   3. ไปที่ ตั้งค่า → API Key → สร้าง API Key + Secret Key
//   4. จดทะเบียน Sender Name
// ============================================================

type SMSMKTProvider struct {
	apiKey    string
	secretKey string
	sender    string
	client    *http.Client
	baseURL   string
}

func NewSMSMKTProvider(apiKey, secretKey, sender string) *SMSMKTProvider {
	return &SMSMKTProvider{
		apiKey:    apiKey,
		secretKey: secretKey,
		sender:    sender,
		client:    &http.Client{Timeout: 30 * time.Second},
		baseURL:   "https://portal-otp.smsmkt.com/api",
	}
}

func (p *SMSMKTProvider) Name() string {
	return "SMSMKT"
}

// SMSMKT Send Message Request (JSON body)
type smsmktSendRequest struct {
	Message string `json:"message"`
	Phone   string `json:"phone"`
	Sender  string `json:"sender"`
}

// SMSMKT API Response
type smsmktResponse struct {
	Code   string      `json:"code"`
	Detail string      `json:"detail"`
	Result interface{} `json:"result"`
}

// SendSMS ส่ง SMS ผ่าน SMSMKT Send Message API
// POST https://portal-otp.smsmkt.com/api/send-message
// Headers: api_key, secret_key
// Body: { "message", "phone", "sender" }
func (p *SMSMKTProvider) SendSMS(phone, message string) error {
	apiURL := p.baseURL + "/send-message"

	reqBody := smsmktSendRequest{
		Message: message,
		Phone:   phone,
		Sender:  p.sender,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// ============================================================
	// SMSMKT Authentication: ส่ง api_key + secret_key ผ่าน Headers
	// (ไม่ใช่ Basic Auth, ไม่ใช่ใส่ใน body)
	// ============================================================
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api_key", p.apiKey)
	req.Header.Set("secret_key", p.secretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	// Log response for debugging
	log.Printf("📱 SMSMKT response [%d]: %s", resp.StatusCode, string(body))

	// ตรวจ HTTP status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SMSMKT HTTP error [%d]: %s", resp.StatusCode, string(body))
	}

	// Parse response JSON
	var smsResp smsmktResponse
	if err := json.Unmarshal(body, &smsResp); err != nil {
		// ถ้า parse ไม่ได้ แต่ status 200 ก็ถือว่าสำเร็จ
		log.Printf("⚠️ SMSMKT response parse warning: %v", err)
		return nil
	}

	// ตรวจ code จาก response body
	// SMSMKT คืน code "000" = success
	if smsResp.Code != "" && smsResp.Code != "000" && smsResp.Code != "200" {
		return fmt.Errorf("SMSMKT API error [%s]: %s", smsResp.Code, smsResp.Detail)
	}

	return nil
}

// CheckCredit เช็คเครดิตคงเหลือ
// GET https://portal-otp.smsmkt.com/api/get-credit
func (p *SMSMKTProvider) CheckCredit() (string, error) {
	apiURL := p.baseURL + "/get-credit"

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("api_key", p.apiKey)
	req.Header.Set("secret_key", p.secretKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SMSMKT error [%d]: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// ============================================================
// Helper Functions
// ============================================================

// formatPhoneForSMS แปลงเบอร์ไทยเป็นรูปแบบที่ SMSMKT รับ
// SMSMKT รับเบอร์รูปแบบ 08xxxxxxxx (เบอร์ไทยปกติ 10 หลัก)
func formatPhoneForSMS(phone string) string {
	// ลบ characters ที่ไม่ใช่ตัวเลข
	cleaned := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			cleaned += string(ch)
		}
	}

	// 66XXXXXXXXX → 0XXXXXXXXX (SMSMKT ใช้เบอร์ไทย 0x format)
	if strings.HasPrefix(cleaned, "66") && len(cleaned) == 11 {
		return "0" + cleaned[2:]
	}

	// ถ้าเป็น 0XXXXXXXXX อยู่แล้ว → ใช้ได้เลย
	if strings.HasPrefix(cleaned, "0") && len(cleaned) == 10 {
		return cleaned
	}

	// คืนค่าเดิมถ้าแปลงไม่ได้
	return cleaned
}

// maskPhoneLog ซ่อนเบอร์สำหรับ log (ความปลอดภัย)
func maskPhoneLog(phone string) string {
	if len(phone) < 6 {
		return "***"
	}
	return phone[:3] + "****" + phone[len(phone)-2:]
}
