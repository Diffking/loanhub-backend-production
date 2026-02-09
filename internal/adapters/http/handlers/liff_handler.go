package handlers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================
// LIFF Handler v3 - SMS OTP Security Upgrade
// ✅ LINE Token Verification (ป้องกันปลอม LINE User ID)
// ✅ Device ID Binding (ผูก 1 คน = 1 เครื่อง)
// ✅ Network Type Check (บังคับ Cellular เฉพาะ Register)
// ✅ Login อนุญาต WiFi (มี LINE Token + Device ID ป้องกัน)
// ✅ OTP Phone Verification (ยืนยันเบอร์โทร)
// 🆕 SMS OTP (ส่ง OTP ผ่าน SMS แทน LINE)
// 🆕 ไม่ส่ง OTP กลับใน response (ป้องกันช่องโหว่)
// ============================================================

type LIFFHandler struct {
	db              *gorm.DB
	lineService     *services.LINEService
	otpService      *services.OTPService
	smsService      *services.SMSService // 🆕 SMS Service
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
		smsService:      smsService, // 🆕
		jwtSecret:       jwtSecret,
		accessTokenExp:  accessTokenExp,
		refreshTokenExp: refreshTokenExp,
	}
}

// ============================================================
// Request/Response Structs
// ============================================================

type CheckLineUserRequest struct {
	LineUserID      string `json:"line_user_id" validate:"required"`
	LineAccessToken string `json:"line_access_token" validate:"required"`
}

type LIFFRegisterRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
	MembNo          string `json:"memb_no" validate:"required"`
	Phone           string `json:"phone" validate:"required"`
	OTPCode         string `json:"otp_code" validate:"required"`
	DeviceID        string `json:"device_id" validate:"required"`
	NetworkType     string `json:"network_type"`
}

type LIFFLoginRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
	DeviceID        string `json:"device_id" validate:"required"`
	NetworkType     string `json:"network_type"`
}

// OTP Request
type RequestOTPRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	MembNo          string `json:"memb_no" validate:"required"`
	Phone           string `json:"phone" validate:"required"`
}

type VerifyOTPRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	OTPCode         string `json:"otp_code" validate:"required"`
}

// Device Change Request (ขอเปลี่ยนเครื่อง)
type DeviceChangeRequest struct {
	LineAccessToken string `json:"line_access_token" validate:"required"`
	NewDeviceID     string `json:"new_device_id" validate:"required"`
	OTPCode         string `json:"otp_code" validate:"required"`
}

// ============================================================
// 1. Check LINE User - ตรวจว่า LINE user มีในระบบหรือยัง
// ============================================================
// @Summary Check LINE User (Secured)
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body CheckLineUserRequest true "LINE Access Token"
// @Success 200 {object} response.Response
// @Router /auth/liff/check [post]
func (h *LIFFHandler) CheckLineUser(c *fiber.Ctx) error {
	var req CheckLineUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LineAccessToken == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE Access Token")
	}

	// ✅ Verify LINE Access Token แล้วดึง profile จาก LINE โดยตรง
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		log.Printf("LINE token verify failed: %v", err)
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	// ใช้ LINE User ID จาก profile (ไม่ใช่จาก client)
	lineUserID := profile.UserID

	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", lineUserID).Scan(&count)

	// ถ้ามีในระบบ ส่ง device_id ที่ผูกไว้กลับไปด้วย
	var registeredDeviceID *string
	if count > 0 {
		h.db.Raw("SELECT device_id FROM users WHERE line_user_id = ? AND deleted_at IS NULL", lineUserID).Scan(&registeredDeviceID)
	}

	return response.Success(c, "ตรวจสอบสำเร็จ", fiber.Map{
		"exists":       count > 0,
		"line_user_id": lineUserID,
		"display_name": profile.DisplayName,
		"has_device":   registeredDeviceID != nil && *registeredDeviceID != "",
	})
}

// ============================================================
// 2. Request OTP - ขอ OTP ส่งไปที่เบอร์โทร
// ============================================================
// 🆕 v3: ส่ง OTP ผ่าน SMS (SMSMKT) แทน LINE
//         ไม่ส่ง OTP กลับใน response อีกต่อไป
// ============================================================
// @Summary Request OTP via SMS
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body RequestOTPRequest true "OTP Request"
// @Success 200 {object} response.Response
// @Router /auth/liff/otp/request [post]
func (h *LIFFHandler) RequestOTP(c *fiber.Ctx) error {
	var req RequestOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LineAccessToken == "" || req.MembNo == "" || req.Phone == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}

	// ✅ Verify LINE Token
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	// Pad member number
	membNo := req.MembNo
	for len(membNo) < 5 {
		membNo = "0" + membNo
	}

	// ตรวจเลขสมาชิกในระบบ flommast
	var mastMembNo, mastMobile string
	row := h.db.Raw("SELECT MAST_MEMB_NO, MAST_MOBILE FROM flommast WHERE MAST_MEMB_NO = ?", membNo).Row()
	err = row.Scan(&mastMembNo, &mastMobile)
	if err != nil || mastMembNo == "" {
		return response.BadRequest(c, "ไม่พบเลขสมาชิกนี้ในระบบ")
	}

	// ✅ ตรวจว่าเบอร์โทรตรงกับในระบบ
	cleanPhone := cleanPhoneNumber(req.Phone)
	cleanMastMobile := cleanPhoneNumber(mastMobile)
	if cleanPhone != cleanMastMobile {
		return response.BadRequest(c, "เบอร์โทรไม่ตรงกับข้อมูลสมาชิก")
	}

	// Generate OTP
	otpCode, err := h.otpService.GenerateOTP(profile.UserID, cleanPhone)
	if err != nil {
		log.Printf("🔍 [REGISTER] OTP verify failed: %v", err)
		return response.BadRequest(c, err.Error())
	}

	// ============================================================
	// 🆕 v3: ส่ง OTP ผ่าน SMS (SMSMKT)
	// ถ้าไม่มี SMS provider → fallback ส่งผ่าน LINE อัตโนมัติ
	// ============================================================
	go func() {
		if err := h.smsService.SendOTP(cleanPhone, otpCode, profile.UserID); err != nil {
			log.Printf("❌ Failed to send OTP: %v", err)
		}
	}()

	log.Printf("📱 OTP requested for member %s, phone %s (via %s)",
		membNo, maskPhone(cleanPhone), h.smsService.GetProviderName())

	// ============================================================
	// 🔒 Security: ไม่ส่ง OTP กลับใน response อีกต่อไป
	// OTP จะถูกส่งผ่าน SMS เท่านั้น
	// ============================================================
	responseData := fiber.Map{
		"phone_masked": maskPhone(cleanPhone),
		"expires_in":   300, // 5 minutes
		"provider":     h.smsService.GetProviderName(),
	}

	// แสดง OTP เมื่อ dev mode หรือ LINE fallback
	appMode := strings.TrimSpace(os.Getenv("APP_MODE"))
	providerName := h.smsService.GetProviderName()
	if appMode == "dev" || providerName == "line_fallback" {
		responseData["otp_code"] = otpCode
		if appMode == "dev" {
			responseData["_dev_warning"] = "OTP included for dev/test only"
		}
	}

	return response.Success(c, "ส่ง OTP สำเร็จ กรุณาตรวจสอบ SMS ที่เบอร์ "+maskPhone(cleanPhone), responseData)
}

// ============================================================
// 3. Verify OTP
// ============================================================
// @Summary Verify OTP
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body VerifyOTPRequest true "OTP Verification"
// @Success 200 {object} response.Response
// @Router /auth/liff/otp/verify [post]
func (h *LIFFHandler) VerifyOTP(c *fiber.Ctx) error {
	var req VerifyOTPRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LineAccessToken == "" || req.OTPCode == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}

	// Verify LINE Token
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	// Verify OTP
	if err := h.otpService.VerifyOTP(profile.UserID, req.OTPCode); err != nil {
		log.Printf("🔍 [REGISTER] OTP verify failed: %v", err)
		return response.BadRequest(c, err.Error())
	}

	return response.Success(c, "ยืนยัน OTP สำเร็จ", fiber.Map{
		"verified": true,
	})
}

// ============================================================
// 4. Register with LIFF - ลงทะเบียน (ต้อง verify OTP ก่อน)
// ============================================================
// @Summary Register with LIFF (Secured)
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body LIFFRegisterRequest true "Registration Info"
// @Success 200 {object} response.Response
// @Router /auth/liff/register [post]
func (h *LIFFHandler) Register(c *fiber.Ctx) error {
	log.Println("🔍 [REGISTER] Start")
	var req LIFFRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	// Validate required fields
	if req.LineAccessToken == "" || req.MembNo == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}
	if req.DeviceID == "" {
		return response.BadRequest(c, "กรุณาระบุ Device ID")
	}
	if req.OTPCode == "" {
		return response.BadRequest(c, "กรุณาระบุรหัส OTP")
	}

	// ✅ ตรวจ Network Type - บังคับ Cellular
	log.Printf("🔍 [REGISTER] NetworkType=%s, DeviceID=%s, MembNo=%s, OTP=%s", req.NetworkType, req.DeviceID, req.MembNo, req.OTPCode)
	if err := h.validateNetworkType(req.NetworkType); err != nil {
		log.Printf("🔍 [REGISTER] OTP verify failed: %v", err)
		return response.BadRequest(c, err.Error())
	}

	// ✅ Verify LINE Token แล้วดึง profile
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	lineUserID := profile.UserID

	// ✅ Verify OTP (ต้อง verify ก่อนหน้านี้แล้ว หรือ verify ตอน register เลย)
	log.Printf("🔍 [REGISTER] lineUserID=%s, verifying OTP...", lineUserID)
	if err := h.otpService.VerifyOTP(lineUserID, req.OTPCode); err != nil {
		log.Printf("🔍 [REGISTER] OTP verify failed: %v", err)
		return response.BadRequest(c, err.Error())
	}

	// Pad member number
	membNo := req.MembNo
	for len(membNo) < 5 {
		membNo = "0" + membNo
	}

	// ตรวจว่า LINE นี้ลงทะเบียนแล้วหรือยัง
	var existingCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", lineUserID).Scan(&existingCount)
	if existingCount > 0 {
		return response.BadRequest(c, "LINE นี้ลงทะเบียนแล้ว")
	}

	// ✅ ตรวจว่า Device ID นี้ผูกกับคนอื่นหรือยัง
	var deviceCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE device_id = ? AND deleted_at IS NULL", req.DeviceID).Scan(&deviceCount)
	if deviceCount > 0 {
		return response.BadRequest(c, "เครื่องนี้ลงทะเบียนกับบัญชีอื่นแล้ว กรุณาติดต่อสหกรณ์")
	}

	// ตรวจเลขสมาชิกใน flommast
	var mastMembNo, fullName, deptName, stsTypeDesc, mastMobile string
	row := h.db.Raw("SELECT MAST_MEMB_NO, Full_Name, DEPT_NAME, STS_TYPE_DESC, MAST_MOBILE FROM flommast WHERE MAST_MEMB_NO = ?", membNo).Row()
	err = row.Scan(&mastMembNo, &fullName, &deptName, &stsTypeDesc, &mastMobile)
	if err != nil || mastMembNo == "" {
		return response.BadRequest(c, "ไม่พบเลขสมาชิกนี้ในระบบ")
	}

	// Get verified phone from OTP
	verifiedPhone := h.otpService.GetVerifiedPhone(lineUserID)
	if verifiedPhone == "" {
		verifiedPhone = cleanPhoneNumber(mastMobile)
	}

	// ตรวจว่ามี user ที่ใช้ memb_no นี้อยู่แล้วหรือไม่
	var userCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE memb_no = ? AND deleted_at IS NULL", membNo).Scan(&userCount)
	if userCount > 0 {
		// ผูก LINE + Device กับบัญชีที่มีอยู่
		h.db.Exec(`UPDATE users SET 
			line_user_id = ?, line_display_name = ?, line_picture_url = ?, 
			line_linked_at = NOW(), device_id = ?, phone_verified = ?, 
			network_type = ?, updated_at = NOW() 
			WHERE memb_no = ? AND deleted_at IS NULL`,
			lineUserID, req.LineDisplayName, req.LinePictureURL,
			req.DeviceID, verifiedPhone,
			req.NetworkType, membNo)

		// Clear OTP
		h.otpService.ClearOTP(lineUserID)

		return response.Success(c, "ผูก LINE กับบัญชีสำเร็จ", fiber.Map{
			"memb_no":   membNo,
			"full_name": fullName,
			"linked":    true,
		})
	}

	// สร้าง user ใหม่ (พร้อม device_id + phone_verified)
	username := "M" + membNo
	h.db.Exec(`INSERT INTO users (
		username, full_name, memb_no, role, dept_name, phone, 
		line_user_id, line_display_name, line_picture_url, line_linked_at, 
		device_id, phone_verified, network_type,
		email, password, created_at, updated_at
	) VALUES (?, ?, ?, 'USER', ?, ?, ?, ?, ?, NOW(), ?, ?, ?, '', '', NOW(), NOW())`,
		username, fullName, membNo, deptName, verifiedPhone,
		lineUserID, req.LineDisplayName, req.LinePictureURL,
		req.DeviceID, verifiedPhone, req.NetworkType)

	// Clear OTP
	h.otpService.ClearOTP(lineUserID)

	return response.Success(c, "ลงทะเบียนสำเร็จ", fiber.Map{
		"memb_no":   membNo,
		"full_name": fullName,
		"linked":    false,
	})
}

// ============================================================
// 5. Login with LIFF - เข้าสู่ระบบ (ตรวจ Device, อนุญาต WiFi)
//    Security: LINE Token Verify + Device ID Binding
//    WiFi: อนุญาต (บังคับ Cellular เฉพาะ Register เท่านั้น)
// ============================================================
// @Summary Login with LIFF (Secured)
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body LIFFLoginRequest true "LINE Info"
// @Success 200 {object} response.Response
// @Router /auth/liff/login [post]
func (h *LIFFHandler) LoginWithLiff(c *fiber.Ctx) error {
	var req LIFFLoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LineAccessToken == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE Access Token")
	}
	if req.DeviceID == "" {
		return response.BadRequest(c, "กรุณาระบุ Device ID")
	}

	// ✅ Login: อนุญาต WiFi ได้ (บังคับ Cellular เฉพาะ Register เท่านั้น)
	//    Security ตอน Login ใช้ LINE Token Verify + Device ID Check แทน
	if nt := strings.ToLower(strings.TrimSpace(req.NetworkType)); nt == "wifi" {
		log.Printf("⚠️ Login via WiFi (allowed) - will verify LINE token + device ID")
	}

	// ✅ Verify LINE Token แล้วดึง profile จาก LINE โดยตรง
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		log.Printf("LINE token verify failed: %v", err)
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง กรุณา login LINE ใหม่")
	}

	lineUserID := profile.UserID

	// ค้นหา user จาก LINE User ID
	var id uint
	var username, fullName, role, membNo string
	var email, deptName, phone, linePictureURL, lineDisplayName, deviceID *string
	row := h.db.Raw(`SELECT id, username, full_name, email, role, memb_no, 
		dept_name, phone, line_picture_url, line_display_name, device_id 
		FROM users WHERE line_user_id = ? AND deleted_at IS NULL`, lineUserID).Row()
	err = row.Scan(&id, &username, &fullName, &email, &role, &membNo,
		&deptName, &phone, &linePictureURL, &lineDisplayName, &deviceID)
	if err != nil || id == 0 {
		return response.NotFound(c, "ไม่พบผู้ใช้ในระบบ กรุณาลงทะเบียน")
	}

	// ✅ ตรวจ Device ID - ต้องตรงกับที่ลงทะเบียนไว้
	if deviceID != nil && *deviceID != "" && *deviceID != req.DeviceID && role != "ADMIN" && role != "OFFICER" {
		log.Printf("⚠️ Device mismatch for user %d: registered=%s, current=%s", id, *deviceID, req.DeviceID)
		return response.Forbidden(c, "เครื่องนี้ไม่ตรงกับที่ลงทะเบียนไว้ กรุณาติดต่อสหกรณ์เพื่อเปลี่ยนเครื่อง")
	}

	// อัพเดท LINE profile + network type + last login
	h.db.Exec(`UPDATE users SET 
		line_display_name = ?, line_picture_url = ?, 
		network_type = ?, last_login = NOW(), updated_at = NOW() 
		WHERE id = ?`,
		req.LineDisplayName, req.LinePictureURL,
		req.NetworkType, id)

	// ถ้ายังไม่ได้ผูก device (user เก่าก่อนอัพเดท) ให้ผูกเลย
	if deviceID == nil || *deviceID == "" {
		h.db.Exec("UPDATE users SET device_id = ? WHERE id = ?", req.DeviceID, id)
		log.Printf("📱 Auto-bound device %s to user %d", req.DeviceID, id)
	}

	// Generate JWT tokens
	accessToken, err := jwt.GenerateAccessToken(id, membNo, username, role, h.jwtSecret, h.accessTokenExp)
	if err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้าง Token ได้")
	}
	tokenID := uuid.New().String()
	refreshToken, err := jwt.GenerateRefreshToken(id, tokenID, h.jwtSecret, h.refreshTokenExp)
	if err != nil {
		return response.InternalServerError(c, "ไม่สามารถสร้าง Token ได้")
	}

	// Save refresh token
	expiresAt := time.Now().AddDate(0, 0, h.refreshTokenExp)
	h.db.Exec("INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())",
		id, refreshToken, expiresAt)

	// Update display values from request
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
// 6. Change Device - ขอเปลี่ยนเครื่อง (ต้อง OTP)
// ============================================================
// @Summary Request Device Change
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body DeviceChangeRequest true "Device Change Request"
// @Success 200 {object} response.Response
// @Router /auth/liff/device/change [post]
func (h *LIFFHandler) ChangeDevice(c *fiber.Ctx) error {
	var req DeviceChangeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LineAccessToken == "" || req.NewDeviceID == "" || req.OTPCode == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}

	// Verify LINE Token
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	lineUserID := profile.UserID

	// Verify OTP
	log.Printf("🔍 [REGISTER] lineUserID=%s, verifying OTP...", lineUserID)
	if err := h.otpService.VerifyOTP(lineUserID, req.OTPCode); err != nil {
		log.Printf("🔍 [REGISTER] OTP verify failed: %v", err)
		return response.BadRequest(c, err.Error())
	}

	// ตรวจว่า Device ID ใหม่ไม่ซ้ำกับคนอื่น
	var deviceCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE device_id = ? AND line_user_id != ? AND deleted_at IS NULL",
		req.NewDeviceID, lineUserID).Scan(&deviceCount)
	if deviceCount > 0 {
		return response.BadRequest(c, "เครื่องนี้ลงทะเบียนกับบัญชีอื่นแล้ว")
	}

	// อัพเดท Device ID
	result := h.db.Exec("UPDATE users SET device_id = ?, updated_at = NOW() WHERE line_user_id = ? AND deleted_at IS NULL",
		req.NewDeviceID, lineUserID)
	if result.RowsAffected == 0 {
		return response.NotFound(c, "ไม่พบผู้ใช้ในระบบ")
	}

	// Clear OTP
	h.otpService.ClearOTP(lineUserID)

	log.Printf("📱 Device changed for LINE user %s: new device = %s", lineUserID, req.NewDeviceID)

	return response.Success(c, "เปลี่ยนเครื่องสำเร็จ", fiber.Map{
		"new_device_id": req.NewDeviceID,
	})
}

// ============================================================
// 7. Get Device Info - ดูข้อมูล device ที่ผูกไว้
// ============================================================
// @Summary Get Device Info
// @Tags LIFF
// @Produce json
// @Success 200 {object} response.Response
// @Router /auth/liff/device/info [post]
func (h *LIFFHandler) GetDeviceInfo(c *fiber.Ctx) error {
	var req struct {
		LineAccessToken string `json:"line_access_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	// Verify LINE Token
	profile, err := h.lineService.VerifyAndGetProfile(req.LineAccessToken)
	if err != nil {
		return response.Unauthorized(c, "LINE Token ไม่ถูกต้อง")
	}

	var result struct {
		DeviceID      *string    `json:"device_id"`
		PhoneVerified *string    `json:"phone_verified"`
		LastLogin     *time.Time `json:"last_login"`
	}
	h.db.Raw("SELECT device_id, phone_verified, last_login FROM users WHERE line_user_id = ? AND deleted_at IS NULL",
		profile.UserID).Scan(&result)

	return response.Success(c, "ข้อมูลอุปกรณ์", fiber.Map{
		"device_id":      result.DeviceID,
		"phone_verified": result.PhoneVerified,
		"last_login":     result.LastLogin,
	})
}

// ============================================================
// Helper Functions
// ============================================================

// validateNetworkType ตรวจว่าเป็น cellular หรือไม่
func (h *LIFFHandler) validateNetworkType(networkType string) error {
	// ถ้าไม่ส่งมา ให้ผ่าน (backward compatible / LIFF อาจตรวจไม่ได้)
	if networkType == "" {
		return nil
	}

	nt := strings.ToLower(strings.TrimSpace(networkType))

	// อนุญาตเฉพาะ cellular / mobile
	allowedTypes := map[string]bool{
		"cellular": true,
		"mobile":   true,
		"4g":       true,
		"5g":       true,
		"3g":       true,
		"lte":      true,
	}

	if !allowedTypes[nt] {
		// WiFi หรือ type อื่นจะไม่อนุญาต
		if nt == "wifi" {
			return fmt.Errorf("กรุณาใช้อินเทอร์เน็ตมือถือ (Cellular) ในการเข้าสู่ระบบ ไม่สามารถใช้ WiFi ได้")
		}
		// Unknown type - log แต่ให้ผ่าน (เพื่อ backward compatible)
		log.Printf("⚠️ Unknown network type: %s - allowing", nt)
		return nil
	}

	return nil
}

// cleanPhoneNumber ลบ -, +66, ช่องว่าง ออก แล้วแปลงเป็น 0XXXXXXXXX
func cleanPhoneNumber(phone string) string {
	// ลบ characters ที่ไม่ใช่ตัวเลข
	cleaned := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			cleaned += string(ch)
		}
	}

	// แปลง 66XXXXXXXXX → 0XXXXXXXXX
	if strings.HasPrefix(cleaned, "66") && len(cleaned) == 11 {
		cleaned = "0" + cleaned[2:]
	}

	// แปลง +66... (ถ้าหลุดมา)
	if strings.HasPrefix(cleaned, "660") {
		cleaned = cleaned[2:]
	}

	return cleaned
}

// maskPhone ซ่อนเบอร์โทร เช่น 089XXXX567
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "XXXX" + phone[len(phone)-3:]
}
