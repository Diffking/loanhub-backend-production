package handlers

import (
	"os"
	"strconv"
	"time"

	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LIFFHandler struct {
	db              *gorm.DB
	jwtSecret       string
	accessTokenExp  int
	refreshTokenExp int
}

func NewLIFFHandler(db *gorm.DB) *LIFFHandler {
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
		jwtSecret:       jwtSecret,
		accessTokenExp:  accessTokenExp,
		refreshTokenExp: refreshTokenExp,
	}
}

type CheckLineUserRequest struct {
	LineUserID string `json:"line_user_id" validate:"required"`
}

// @Summary Check LINE User
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body CheckLineUserRequest true "LINE User ID"
// @Success 200 {object} response.Response
// @Router /auth/liff/check [post]
func (h *LIFFHandler) CheckLineUser(c *fiber.Ctx) error {
	var req CheckLineUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineUserID == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE User ID")
	}
	var count int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", req.LineUserID).Scan(&count)
	return response.Success(c, "ตรวจสอบสำเร็จ", fiber.Map{
		"exists":       count > 0,
		"line_user_id": req.LineUserID,
	})
}

type LIFFRegisterRequest struct {
	LineUserID      string `json:"line_user_id" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
	MembNo          string `json:"memb_no" validate:"required"`
}

// @Summary Register with LIFF
// @Tags LIFF
// @Accept json
// @Produce json
// @Param request body LIFFRegisterRequest true "Registration Info"
// @Success 200 {object} response.Response
// @Router /auth/liff/register [post]
func (h *LIFFHandler) Register(c *fiber.Ctx) error {
	var req LIFFRegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}
	if req.LineUserID == "" || req.MembNo == "" {
		return response.BadRequest(c, "กรุณาระบุข้อมูลให้ครบ")
	}
	membNo := req.MembNo
	for len(membNo) < 5 {
		membNo = "0" + membNo
	}
	var existingCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE line_user_id = ? AND deleted_at IS NULL", req.LineUserID).Scan(&existingCount)
	if existingCount > 0 {
		return response.BadRequest(c, "LINE นี้ลงทะเบียนแล้ว")
	}
	var mastMembNo, fullName, deptName, stsTypeDesc, mastMobile string
	row := h.db.Raw("SELECT MAST_MEMB_NO, Full_Name, DEPT_NAME, STS_TYPE_DESC, MAST_MOBILE FROM flommast WHERE MAST_MEMB_NO = ?", membNo).Row()
	err := row.Scan(&mastMembNo, &fullName, &deptName, &stsTypeDesc, &mastMobile)
	if err != nil || mastMembNo == "" {
		return response.BadRequest(c, "ไม่พบเลขสมาชิกนี้ในระบบ")
	}
	var userCount int64
	h.db.Raw("SELECT COUNT(*) FROM users WHERE memb_no = ? AND deleted_at IS NULL", membNo).Scan(&userCount)
	if userCount > 0 {
		h.db.Exec("UPDATE users SET line_user_id = ?, line_display_name = ?, line_picture_url = ?, line_linked_at = NOW(), updated_at = NOW() WHERE memb_no = ? AND deleted_at IS NULL", req.LineUserID, req.LineDisplayName, req.LinePictureURL, membNo)
		return response.Success(c, "ผูก LINE กับบัญชีสำเร็จ", fiber.Map{
			"memb_no":   membNo,
			"full_name": fullName,
			"linked":    true,
		})
	}
	username := "M" + membNo
	h.db.Exec("INSERT INTO users (username, full_name, memb_no, role, dept_name, phone, line_user_id, line_display_name, line_picture_url, line_linked_at, email, password, created_at, updated_at) VALUES (?, ?, ?, 'USER', ?, ?, ?, ?, ?, NOW(), '', '', NOW(), NOW())", username, fullName, membNo, deptName, mastMobile, req.LineUserID, req.LineDisplayName, req.LinePictureURL)
	return response.Success(c, "ลงทะเบียนสำเร็จ", fiber.Map{
		"memb_no":   membNo,
		"full_name": fullName,
		"linked":    false,
	})
}

type LIFFLoginRequest struct {
	LineUserID      string `json:"line_user_id" validate:"required"`
	LineDisplayName string `json:"line_display_name"`
	LinePictureURL  string `json:"line_picture_url"`
}

// @Summary Login with LIFF
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
	if req.LineUserID == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE User ID")
	}
	var id uint
	var username, fullName, role, membNo string
	var email, deptName, phone, linePictureURL, lineDisplayName *string
	row := h.db.Raw("SELECT id, username, full_name, email, role, memb_no, dept_name, phone, line_picture_url, line_display_name FROM users WHERE line_user_id = ? AND deleted_at IS NULL", req.LineUserID).Row()
	err := row.Scan(&id, &username, &fullName, &email, &role, &membNo, &deptName, &phone, &linePictureURL, &lineDisplayName)
	if err != nil || id == 0 {
		return response.NotFound(c, "ไม่พบผู้ใช้ในระบบ กรุณาลงทะเบียน")
	}
	h.db.Exec("UPDATE users SET line_display_name = ?, line_picture_url = ?, updated_at = NOW() WHERE id = ?", req.LineDisplayName, req.LinePictureURL, id)
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
	h.db.Exec("INSERT INTO refresh_tokens (user_id, token, expires_at, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())", id, refreshToken, expiresAt)
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
