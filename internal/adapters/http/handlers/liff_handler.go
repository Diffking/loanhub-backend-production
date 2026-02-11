package handlers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================
// LIFF Handler v3 - ลบ Device ID ออกทั้งหมด
// ✅ LINE Token Verification (ป้องกันปลอม LINE User ID)
// ✅ OTP Phone Verification (ยืนยันเบอร์โทร)
// ✅ Brute-force Protection (ล็อคหลังใส่ผิด 5 ครั้ง, 30 นาที)
// ============================================================

const (
	maxCardAttempts = 5
	lockDuration    = 30 * time.Minute
)

type cardAttemptInfo struct {
	count    int
	lockedAt time.Time
}

var (
	cardAttempts   = make(map[string]*cardAttemptInfo)
	cardAttemptsMu sync.Mutex
)

func checkCardLock(membNo string) time.Duration {
	cardAttemptsMu.Lock()
	defer cardAttemptsMu.Unlock()
	info, exists := cardAttempts[membNo]
	if !exists {
		return 0
	}
	if info.count >= maxCardAttempts && !info.lockedAt.IsZero() {
		remaining := lockDuration - time.Since(info.lockedAt)
		if remaining > 0 {
			return remaining
		}
		delete(cardAttempts, membNo)
	}
	return 0
}

func recordCardFail(membNo string) (locked bool, remaining time.Duration) {
	cardAttemptsMu.Lock()
	defer cardAttemptsMu.Unlock()
	info, exists := cardAttempts[membNo]
	if !exists {
		info = &cardAttemptInfo{}
		cardAttempts[membNo] = info
	}
	info.count++
	if info.count >= maxCardAttempts {
		info.lockedAt = time.Now()
		log.Printf("🔒 [SECURITY] Member %s locked after %d failed card attempts", membNo, info.count)
		return true, lockDuration
	}
	return false, 0
}

func resetCardAttempts(membNo string) {
	cardAttemptsMu.Lock()
	defer cardAttemptsMu.Unlock()
	delete(cardAttempts, membNo)
}

type LIFFHandler struct {
	db              *gorm.DB
	lineService     *services.LINEService
	otpService      *services.OTPService
	smsService      *services.SMSService
	jwtSecret       string
	accessTokenExp  int
	refreshTokenExp int
}

func NewLIFFHandler(db *gorm.DB, lineService *services.LINEService, otpService *services.OTPService, smsService *services.SMSService) *LIFFHandler {
	jwtSecret := os.Getenv("PROD_JWT_SECRET")
	accessTokenExp := 1440
	if exp := os.Getenv("ACCESS_TOKEN_EXPIRY"); exp != "" {
		if val, err := strconv.Atoi(exp); err == nil {
			accessTokenExp = val
		}
	}
	refreshTokenExp := 7
	if exp := os.Getenv("REFRESH_TOKEN_EXPIRY"); exp != "" {
		if val, err := strconv.Atoi(exp); err == nil {
			refreshTokenExp = val
		}
	}
	return &LIFFHandler{
		db:              db,
		lineService:     lineService,
		otpService:      otpService,
		smsService:      smsService,
		jwtSecret:       jwtSecret,
		accessTokenExp:  accessTokenExp,
		refreshTokenExp: refreshTokenExp,
	}
}

// ============================================================
// Request/Response Structs
// ============================================================

type CheckLineUserRequest struct {
	LineUserID      string `json:"line_user_id"`
	LineAccessToken string `json:"line_access_token" validate:"required"`
}

type LIFFRegisterRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
	MembNo          string `json:"memb_no" validate:"required"`
	Phone           string `json:"phone" validate:"required"`
	CardLast4       string `json:"card_last4" validate:"required"`
	OTPCode         string `json:"otp_code" validate:"required"`
	NetworkType     string `json:"network_type"`
}

type LIFFLoginRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
	NetworkType     string `json:"network_type"`
}

type RequestOTPRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	MembNo          string `json:"memb_no" validate:"required"`
	Phone           string `json:"phone" validate:"required"`
	CardLast4       string `json:"card_last4" validate:"required"`
}

type VerifyOTPRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	OTPCode         string `json:"otp_code" validate:"required"`
}

// ============================================================
// 1. Check LINE User
// ============================================================
func (h *LIFFHandler) CheckLineUser(c *fiber.Ctx) error {
	var req CheckLineUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineAccessToken == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE Access Token")
	}

	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", profile.UserID).Scan(&count)

	return response.Success(c, "ตรวจสอบสำเร็จ", fiber.Map{
		"exists":       count > 0,
		"line_user_id": profile.UserID,
		"display_name": profile.DisplayName,
	})
}

// ============================================================
// 2. Request OTP
// ============================================================
func (h *LIFFHandler) RequestOTP(c *fiber.Ctx) error {
	var req RequestOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineAccessToken == "" || req.MembNo == "" || req.Phone == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}
	if req.CardLast4 == "" || len(req.CardLast4) != 4 {
		return response.BadRequest(c, "กรุณาระบุเลขบัตรประชาชน 4 หลักสุดท้าย")
	}

	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	membNo := req.MembNo
	for len(membNo) < 5 {
		membNo = "0" + membNo
	}

	var mastMembNo, mastMobile, mastCardID string
	row := h.db.Raw("SELECT MAST_MEMB_NO, MAST_MOBILE, COALESCE(mast_card_id, '') FROM flommast WHERE MAST_MEMB_NO = ?", membNo).Row()
	err = row.Scan(&mastMembNo, &mastMobile, &mastCardID)
	if err != nil || mastMembNo == "" {
		return response.BadRequest(c, "ไม่พบเลขสมาชิกนี้ในระบบ")
	}

	cleanPhone := cleanPhoneNumber(req.Phone)
	cleanMastMobile := cleanPhoneNumber(mastMobile)
	if cleanPhone != cleanMastMobile {
		return response.BadRequest(c, "เบอร์โทรไม่ตรงกับข้อมูลสมาชิก")
	}

	if remaining := checkCardLock(membNo); remaining > 0 {
		mins := int(remaining.Minutes()) + 1
		return response.BadRequest(c, fmt.Sprintf("ใส่เลขบัตรผิดเกินกำหนด กรุณารอ %d นาที", mins))
	}

	cleanCardID := ""
	for _, ch := range mastCardID {
		if ch >= '0' && ch <= '9' {
			cleanCardID += string(ch)
		}
	}
	if len(cleanCardID) < 4 {
		return response.BadRequest(c, "ข้อมูลบัตรประชาชนในระบบไม่สมบูรณ์ กรุณาติดต่อสหกรณ์")
	}
	dbCardLast4 := cleanCardID[len(cleanCardID)-4:]
	if req.CardLast4 != dbCardLast4 {
		locked, _ := recordCardFail(membNo)
		if locked {
			return response.BadRequest(c, fmt.Sprintf("ใส่เลขบัตรผิดเกินกำหนด กรุณารอ %d นาที", int(lockDuration.Minutes())))
		}
		return response.BadRequest(c, "เลขบัตรประชาชน 4 หลักสุดท้ายไม่ตรงกับข้อมูลสมาชิก")
	}
	resetCardAttempts(membNo)

	otpCode, err := h.otpService.GenerateOTP(profile.UserID, cleanPhone)
	if err != nil {
		return response.BadRequest(c, err.Error())
	}

	smsMessage := fmt.Sprintf("รหัส OTP ของคุณคือ: %s (หมดอายุใน 5 นาที) - สหกรณ์ SPSC", otpCode)
	channelAccessToken := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if channelAccessToken != "" {
		go func() {
			if err := h.lineService.SendPushMessage(profile.UserID, smsMessage, channelAccessToken); err != nil {
				log.Printf("Failed to send OTP via LINE: %v", err)
			}
		}()
	}

	log.Printf("📱 OTP Generated for member %s, phone %s: %s", membNo, cleanPhone, otpCode)

	return response.Success(c, "ส่ง OTP สำเร็จ", fiber.Map{
		"phone_masked": maskPhone(cleanPhone),
		"otp_code":     otpCode,
		"expires_in":   300,
	})
}

// ============================================================
// 3. Verify OTP
// ============================================================
func (h *LIFFHandler) VerifyOTP(c *fiber.Ctx) error {
	var req VerifyOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineAccessToken == "" || req.OTPCode == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}

	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	if err := h.otpService.VerifyOTP(profile.UserID, req.OTPCode); err != nil {
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "ยืนยัน OTP สำเร็จ", fiber.Map{
		"verified": true,
	})
}

// ============================================================
// 4. Register with LIFF
// ============================================================
func (h *LIFFHandler) Register(c *fiber.Ctx) error {
	var req LIFFRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineAccessToken == "" || req.MembNo == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}
	if req.OTPCode == "" {
		return response.BadRequest(c, "กรุณาระบุรหัส OTP")
	}
	if req.CardLast4 == "" || len(req.CardLast4) != 4 {
		return response.BadRequest(c, "กรุณาระบุเลขบัตรประชาชน 4 หลักสุดท้าย")
	}

	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	lineUserID := profile.UserID

	if err := h.otpService.VerifyOTP(lineUserID, req.OTPCode); err != nil {
		return response.BadRequest(c, err.Error())
	}

	membNo := req.MembNo
	for len(membNo) < 5 {
		membNo = "0" + membNo
	}

	var existingCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", lineUserID).Scan(&existingCount)
	if existingCount > 0 {
		return response.BadRequest(c, "LINE นี้ลงทะเบียนแล้ว")
	}

	var mastMembNo, fullName, deptName, stsTypeDesc, mastMobile, mastCardID string
	row := h.db.Raw("SELECT MAST_MEMB_NO, Full_Name, DEPT_NAME, STS_TYPE_DESC, MAST_MOBILE, COALESCE(mast_card_id, '') FROM flommast WHERE MAST_MEMB_NO = ?", membNo).Row()
	err = row.Scan(&mastMembNo, &fullName, &deptName, &stsTypeDesc, &mastMobile, &mastCardID)
	if err != nil || mastMembNo == "" {
		return response.BadRequest(c, "ไม่พบเลขสมาชิกนี้ในระบบ")
	}

	if remaining := checkCardLock(membNo); remaining > 0 {
		mins := int(remaining.Minutes()) + 1
		return response.BadRequest(c, fmt.Sprintf("ใส่เลขบัตรผิดเกินกำหนด กรุณารอ %d นาที", mins))
	}

	cleanCardID := ""
	for _, ch := range mastCardID {
		if ch >= '0' && ch <= '9' {
			cleanCardID += string(ch)
		}
	}
	if len(cleanCardID) < 4 {
		return response.BadRequest(c, "ข้อมูลบัตรประชาชนในระบบไม่สมบูรณ์ กรุณาติดต่อสหกรณ์")
	}
	dbCardLast4 := cleanCardID[len(cleanCardID)-4:]
	if req.CardLast4 != dbCardLast4 {
		locked, _ := recordCardFail(membNo)
		if locked {
			return response.BadRequest(c, fmt.Sprintf("ใส่เลขบัตรผิดเกินกำหนด กรุณารอ %d นาที", int(lockDuration.Minutes())))
		}
		return response.BadRequest(c, "เลขบัตรประชาชน 4 หลักสุดท้ายไม่ตรงกับข้อมูลสมาชิก")
	}
	resetCardAttempts(membNo)

	verifiedPhone := h.otpService.GetVerifiedPhone(lineUserID)
	if verifiedPhone == "" {
		verifiedPhone = cleanPhoneNumber(mastMobile)
	}

	var userCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE memb_no = ? AND deleted_at IS NULL", membNo).Scan(&userCount)
	if userCount > 0 {
		h.db.Exec(`UPDATE users SET 
			line_user_id = ?, line_display_name = ?, line_picture_url = ?, 
			line_linked_at = NOW(), phone_verified = ?, 
			network_type = ?, updated_at = NOW() 
			WHERE memb_no = ? AND deleted_at IS NULL`,
			lineUserID, req.LineDisplayName, req.LinePictureURL,
			verifiedPhone, req.NetworkType, membNo)

		h.otpService.ClearOTP(lineUserID)
		return response.Success(c, "ผูก LINE กับบัญชีสำเร็จ", fiber.Map{
			"memb_no":   membNo,
			"full_name": fullName,
			"linked":    true,
		})
	}

	username := "M" + membNo
	h.db.Exec(`INSERT INTO users (
		username, full_name, memb_no, role, dept_name, phone, 
		line_user_id, line_display_name, line_picture_url, line_linked_at, 
		phone_verified, network_type,
		email, password, created_at, updated_at
	) VALUES (?, ?, ?, 'USER', ?, ?, ?, ?, ?, NOW(), ?, ?, NULL, '', NOW(), NOW())`,
		username, fullName, membNo, deptName, verifiedPhone,
		lineUserID, req.LineDisplayName, req.LinePictureURL,
		verifiedPhone, req.NetworkType)

	h.otpService.ClearOTP(lineUserID)
	return response.Success(c, "ลงทะเบียนสำเร็จ", fiber.Map{
		"memb_no":   membNo,
		"full_name": fullName,
		"linked":    false,
	})
}

// ============================================================
// 5. Login with LIFF
// ============================================================
func (h *LIFFHandler) LoginWithLiff(c *fiber.Ctx) error {
	var req LIFFLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineAccessToken == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE Access Token")
	}

	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		log.Printf("LINE token verify failed: %v", err)
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	lineUserID := profile.UserID

	var id uint
	var username, fullName, role, membNo string
	var email, deptName, phone, linePictureURL, lineDisplayName *string
	row := h.db.Raw(`SELECT id, username, full_name, email, role, memb_no, 
		dept_name, phone, line_picture_url, line_display_name 
		FROM users WHERE line_user_id = ? AND deleted_at IS NULL`, lineUserID).Row()
	err = row.Scan(&id, &username, &fullName, &email, &role, &membNo,
		&deptName, &phone, &linePictureURL, &lineDisplayName)
	if err != nil || id == 0 {
		return response.NotFound(c, "ไม่พบผู้ใช้ในระบบ กรุณาลงทะเบียน")
	}

	h.db.Exec(`UPDATE users SET 
		line_display_name = ?, line_picture_url = ?, 
		network_type = ?, last_login = NOW(), updated_at = NOW() 
		WHERE id = ?`,
		req.LineDisplayName, req.LinePictureURL,
		req.NetworkType, id)

	accessToken, err := jwt.GenerateAccessToken(id, membNo, username, role, h.jwtSecret, h.accessTokenExp)
	if err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้าง Token ได้")
	}
	tokenID := uuid.New().String()
	refreshToken, err := jwt.GenerateRefreshToken(id, tokenID, h.jwtSecret, h.refreshTokenExp)
	if err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้าง Token ได้")
	}

	expiresAt := time.Now().AddDate(0, 0, h.refreshTokenExp)
	h.db.Exec("INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())",
		id, refreshToken, expiresAt)

	if req.LinePictureURL != "" {
		linePictureURL = &req.LinePictureURL
	}
	if req.LineDisplayName != "" {
		lineDisplayName = &req.LineDisplayName
	}

	return response.Success(c, "เข้าสู่ระบบสำเร็จ", fiber.Map{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": fiber.Map{
			"id":                id,
			"username":          username,
			"full_name":         fullName,
			"email":             email,
			"role":              role,
			"memb_no":           membNo,
			"dept_name":         deptName,
			"phone":             phone,
			"line_picture_url":  linePictureURL,
			"line_display_name": lineDisplayName,
		},
	})
}

// ============================================================
// Helper Functions
// ============================================================

func cleanPhoneNumber(phone string) string {
	cleaned := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			cleaned += string(ch)
		}
	}
	if strings.HasPrefix(cleaned, "66") && len(cleaned) == 11 {
		cleaned = "0" + cleaned[2:]
	}
	if strings.HasPrefix(cleaned, "660") {
		cleaned = cleaned[2:]
	}
	return cleaned
}

func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "XXXX" + phone[len(phone)-3:]
}
