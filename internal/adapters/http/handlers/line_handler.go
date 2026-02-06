package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"time"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LINEHandler handles LINE related requests
type LINEHandler struct {
	lineService     *services.LINEService
	db              *gorm.DB
	jwtSecret       string
	accessTokenExp  int // minutes
	refreshTokenExp int // days
}

// NewLINEHandler creates a new LINE handler
func NewLINEHandler(db *gorm.DB) *LINEHandler {
	channelID := os.Getenv("LINE_CHANNEL_ID")
	channelSecret := os.Getenv("LINE_CHANNEL_SECRET")
	callbackURL := os.Getenv("LINE_CALLBACK_URL")
	liffChannelID := os.Getenv("LIFF_CHANNEL_ID") // ✅ LIFF Channel ID
	jwtSecret := os.Getenv("PROD_JWT_SECRET")

	if callbackURL == "" {
		callbackURL = "https://api.loanspsc.com/api/v1/auth/line/callback"
	}

	// Token expiry settings
	accessTokenExp := 1440 // 24 hours in minutes
	if exp := os.Getenv("ACCESS_TOKEN_EXPIRY"); exp != "" {
		if val, err := strconv.Atoi(exp); err == nil {
			accessTokenExp = val
		}
	}

	refreshTokenExp := 7 // 7 days
	if exp := os.Getenv("REFRESH_TOKEN_EXPIRY"); exp != "" {
		if val, err := strconv.Atoi(exp); err == nil {
			refreshTokenExp = val
		}
	}

	return &LINEHandler{
		lineService:     services.NewLINEService(db, channelID, channelSecret, callbackURL, liffChannelID),
		db:              db,
		jwtSecret:       jwtSecret,
		accessTokenExp:  accessTokenExp,
		refreshTokenExp: refreshTokenExp,
	}
}

// GetLINELoginURL returns LINE Login URL (Public - for login with LINE)
// @Summary Get LINE Login URL for authentication
// @Description Get URL to redirect user for LINE Login (no auth required)
// @Tags LINE
// @Produce json
// @Param mode query string false "Mode: login or link" default(login)
// @Success 200 {object} response.Response
// @Router /auth/line/url [get]
func (h *LINEHandler) GetLINELoginURL(c *fiber.Ctx) error {
	mode := c.Query("mode", "login") // "login" or "link"

	state, _ := generateRandomState()

	// Store state and mode in cookie
	c.Cookie(&fiber.Cookie{
		Name:     "line_state",
		Value:    state,
		MaxAge:   300,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	c.Cookie(&fiber.Cookie{
		Name:     "line_mode",
		Value:    mode,
		MaxAge:   300,
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
	})

	// If linking, store user_id
	if mode == "link" {
		userID := c.Locals("user_id")
		if userID != nil {
			c.Cookie(&fiber.Cookie{
				Name:     "line_link_user",
				Value:    strconv.FormatUint(uint64(userID.(uint)), 10),
				MaxAge:   300,
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Lax",
			})
		}
	}

	loginURL := h.lineService.GetLoginURL(state)
	return response.Success(c, "LINE Login URL", fiber.Map{
		"url":  loginURL,
		"mode": mode,
	})
}

// LINECallback handles LINE callback
// @Summary LINE Login Callback
// @Description Handle callback from LINE after user authorization
// @Tags LINE
// @Produce json
// @Param code query string true "Authorization code from LINE"
// @Param state query string true "State for CSRF protection"
// @Success 200 {object} response.Response
// @Router /auth/line/callback [get]
func (h *LINEHandler) LINECallback(c *fiber.Ctx) error {
	code := c.Query("code")
	state := c.Query("state")
	errorParam := c.Query("error")

	// Get mode from cookie
	mode := c.Cookies("line_mode")
	if mode == "" {
		mode = "login"
	}

	// Frontend redirect URL
	frontendURL := os.Getenv("WEB_APP_URL")
	if frontendURL == "" {
		frontendURL = "https://loanspsc.com"
	}

	// Check for error from LINE
	if errorParam != "" {
		return c.Redirect(frontendURL + "/login?error=line_cancelled")
	}

	if code == "" {
		return c.Redirect(frontendURL + "/login?error=no_code")
	}

	// Verify state
	savedState := c.Cookies("line_state")
	if savedState == "" || savedState != state {
		return c.Redirect(frontendURL + "/login?error=invalid_state")
	}

	// Exchange code for token
	tokenResp, err := h.lineService.ExchangeToken(code)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=token_exchange_failed")
	}

	// Get LINE profile
	profile, err := h.lineService.GetProfile(tokenResp.AccessToken)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=profile_failed")
	}

	// Clear LINE cookies
	c.Cookie(&fiber.Cookie{Name: "line_state", Value: "", MaxAge: -1})
	c.Cookie(&fiber.Cookie{Name: "line_mode", Value: "", MaxAge: -1})
	c.Cookie(&fiber.Cookie{Name: "line_link_user", Value: "", MaxAge: -1})

	// Handle based on mode
	if mode == "link" {
		// Link mode - redirect back to profile with LINE data
		return c.Redirect(frontendURL + "/profile?line_user_id=" + profile.UserID + "&line_name=" + profile.DisplayName)
	}

	// Login mode - find user by LINE ID
	var user struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		MembNo   string `json:"memb_no"`
	}

	result := h.db.Raw(`
		SELECT id, username, full_name, email, role, memb_no 
		FROM users 
		WHERE line_user_id = ? AND deleted_at IS NULL
	`, profile.UserID).Scan(&user)

	if result.Error != nil || user.ID == 0 {
		// User not found - redirect to login with LINE info for registration
		return c.Redirect(frontendURL + "/login?error=not_registered&line_user_id=" + profile.UserID + "&line_name=" + profile.DisplayName)
	}

	// User found - generate JWT tokens
	// GenerateAccessToken(userID uint, membNo, username, role, secret string, expiryMinutes int)
	accessToken, err := jwt.GenerateAccessToken(
		user.ID,
		user.MembNo,
		user.Username,
		user.Role,
		h.jwtSecret,
		h.accessTokenExp,
	)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=token_generation_failed")
	}

	// Generate unique token ID for refresh token
	tokenID := uuid.New().String()

	// GenerateRefreshToken(userID uint, tokenID, secret string, expiryDays int)
	refreshToken, err := jwt.GenerateRefreshToken(
		user.ID,
		tokenID,
		h.jwtSecret,
		h.refreshTokenExp,
	)
	if err != nil {
		return c.Redirect(frontendURL + "/login?error=token_generation_failed")
	}

	// Save refresh token to database
	expiresAt := time.Now().AddDate(0, 0, h.refreshTokenExp)
	h.db.Exec(`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`, user.ID, refreshToken, expiresAt)

	// Update last login
	h.db.Exec(`UPDATE users SET last_login = NOW() WHERE id = ?`, user.ID)

	// Redirect to frontend with tokens
	return c.Redirect(frontendURL + "/login/line-callback?access_token=" + accessToken + "&refresh_token=" + refreshToken)
}

// LinkLINERequest represents request to link LINE
type LinkLINERequest struct {
	LINEUserID  string `json:"line_user_id" validate:"required"`
	DisplayName string `json:"display_name"`
}

// LinkLINE links LINE account to user (requires auth)
// @Summary Link LINE account to user
// @Description Link LINE account to currently logged in user
// @Tags LINE
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body LinkLINERequest true "LINE User Info"
// @Success 200 {object} response.Response
// @Router /auth/line/link [post]
func (h *LINEHandler) LinkLINE(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "กรุณาเข้าสู่ระบบก่อน")
	}

	var req LinkLINERequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "ข้อมูลไม่ถูกต้อง")
	}

	if req.LINEUserID == "" {
		return response.BadRequest(c, "กรุณาระบุ LINE User ID")
	}

	// Check if LINE already linked to another user
	existingUserID, _ := h.lineService.GetUserByLINEID(req.LINEUserID)
	if existingUserID != 0 && existingUserID != userID.(uint) {
		return response.BadRequest(c, "LINE นี้เชื่อมต่อกับบัญชีอื่นแล้ว")
	}

	// Link LINE to user
	if err := h.lineService.LinkUserLINE(userID.(uint), req.LINEUserID, req.DisplayName); err != nil {
		return response.InternalServerError(c, "ไม่สามารถเชื่อมต่อ LINE ได้")
	}

	return response.Success(c, "เชื่อมต่อ LINE สำเร็จ", fiber.Map{
		"line_user_id": req.LINEUserID,
		"display_name": req.DisplayName,
	})
}

// UnlinkLINE unlinks LINE account (requires auth)
// @Summary Unlink LINE account from user
// @Tags LINE
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /auth/line/unlink [post]
func (h *LINEHandler) UnlinkLINE(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "กรุณาเข้าสู่ระบบก่อน")
	}

	if err := h.lineService.UnlinkUserLINE(userID.(uint)); err != nil {
		return response.InternalServerError(c, "ไม่สามารถยกเลิกการเชื่อมต่อได้")
	}

	return response.Success(c, "ยกเลิกการเชื่อมต่อ LINE สำเร็จ", nil)
}

// GetLINEStatus returns LINE link status (requires auth)
// @Summary Get LINE link status
// @Tags LINE
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Router /auth/line/status [get]
func (h *LINEHandler) GetLINEStatus(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return response.Unauthorized(c, "กรุณาเข้าสู่ระบบก่อน")
	}

	var result struct {
		LineUserID      *string `json:"line_user_id"`
		LineDisplayName *string `json:"line_display_name"`
		LineLinkedAt    *string `json:"line_linked_at"`
	}

	h.db.Raw("SELECT line_user_id, line_display_name, line_linked_at FROM users WHERE id = ?", userID).Scan(&result)

	isLinked := result.LineUserID != nil && *result.LineUserID != ""

	return response.Success(c, "LINE Status", fiber.Map{
		"is_linked":    isLinked,
		"line_user_id": result.LineUserID,
		"display_name": result.LineDisplayName,
		"linked_at":    result.LineLinkedAt,
	})
}

// GetLINEService returns the LINE service instance
// ใช้ใน routes.go เพื่อส่งต่อให้ LIFFHandler
func (h *LINEHandler) GetLINEService() *services.LINEService {
	return h.lineService
}

func generateRandomState() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
