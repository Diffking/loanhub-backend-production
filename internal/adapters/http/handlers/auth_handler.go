package handlers

import (
	"errors"
	"strings"
	"time"

	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *services.AuthService
	cfg         *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *services.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cfg:         cfg,
	}
}

// RegisterRequest represents registration request body
type RegisterRequest struct {
	MembNo   string `json:"memb_no"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest represents login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Register handles user registration
// @Summary Register new user
// @Description Register a new user with member number validation
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration data"
// @Success 201 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 409 {object} response.Response
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate required fields
	if req.MembNo == "" {
		return response.BadRequest(c, "Member number is required")
	}
	if req.Username == "" {
		return response.BadRequest(c, "Username is required")
	}
	if req.Email == "" {
		return response.BadRequest(c, "Email is required")
	}
	if req.Password == "" {
		return response.BadRequest(c, "Password is required")
	}
	if len(req.Password) < 8 {
		return response.BadRequest(c, "Password must be at least 8 characters")
	}

	// Register user
	input := &services.RegisterInput{
		MembNo:   strings.TrimSpace(req.MembNo),
		Username: strings.TrimSpace(req.Username),
		Email:    strings.TrimSpace(req.Email),
		Password: req.Password,
	}

	result, err := h.authService.Register(c.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMemberNotFound):
			return response.NotFound(c, "Member number not found in system")
		case errors.Is(err, services.ErrMemberAlreadyUsed):
			return response.Conflict(c, "Member number already registered")
		case errors.Is(err, services.ErrUserAlreadyExists):
			return response.Conflict(c, "Username or email already exists")
		default:
			return response.InternalServerError(c, "Failed to register user")
		}
	}

	// Set cookies
	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)

	return response.Created(c, "User registered successfully", fiber.Map{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}

// Login handles user login
// @Summary Login user
// @Description Authenticate user and return tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login credentials"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate required fields
	if req.Username == "" {
		return response.BadRequest(c, "Username is required")
	}
	if req.Password == "" {
		return response.BadRequest(c, "Password is required")
	}

	// Login
	input := &services.LoginInput{
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
	}

	result, err := h.authService.Login(c.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			return response.Unauthorized(c, "Invalid username or password")
		case errors.Is(err, services.ErrUserInactive):
			return response.Forbidden(c, "User account is inactive")
		default:
			return response.InternalServerError(c, "Failed to login")
		}
	}

	// Set cookies
	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)

	return response.Success(c, "Login successful", fiber.Map{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}

// RefreshToken handles token refresh
// @Summary Refresh access token
// @Description Refresh access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	// Get refresh token from cookie
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return response.Unauthorized(c, "Refresh token not found")
	}

	// Refresh token
	result, err := h.authService.RefreshToken(c.Context(), refreshToken)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTokenExpired):
			h.clearAuthCookies(c)
			return response.Unauthorized(c, "Refresh token expired, please login again")
		case errors.Is(err, services.ErrTokenRevoked):
			h.clearAuthCookies(c)
			return response.Unauthorized(c, "Refresh token revoked, please login again")
		case errors.Is(err, services.ErrInvalidToken):
			h.clearAuthCookies(c)
			return response.Unauthorized(c, "Invalid refresh token")
		case errors.Is(err, services.ErrUserInactive):
			h.clearAuthCookies(c)
			return response.Forbidden(c, "User account is inactive")
		default:
			return response.InternalServerError(c, "Failed to refresh token")
		}
	}

	// Set new cookies
	h.setAuthCookies(c, result.AccessToken, result.RefreshToken)

	return response.Success(c, "Token refreshed successfully", fiber.Map{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}

// Logout handles user logout
// @Summary Logout user
// @Description Logout user and revoke refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Success 200 {object} response.Response
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	// Get refresh token from cookie
	refreshToken := c.Cookies("refresh_token")
	if refreshToken != "" {
		// Revoke refresh token
		_ = h.authService.Logout(c.Context(), refreshToken)
	}

	// Clear cookies
	h.clearAuthCookies(c)

	return response.Success(c, "Logged out successfully", nil)
}

// LogoutAll handles logout from all devices
// @Summary Logout from all devices
// @Description Revoke all refresh tokens for the user
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/logout-all [post]
func (h *AuthHandler) LogoutAll(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	// Revoke all tokens
	if err := h.authService.LogoutAll(c.Context(), userID); err != nil {
		return response.InternalServerError(c, "Failed to logout from all devices")
	}

	// Clear cookies
	h.clearAuthCookies(c)

	return response.Success(c, "Logged out from all devices", nil)
}

// Me returns the current user info
// @Summary Get current user
// @Description Get the currently authenticated user's information
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	// Get user
	user, err := h.authService.GetUserByID(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User not found")
	}

	return response.Success(c, "User retrieved successfully", fiber.Map{
		"user": user.ToResponse(),
	})
}

// setAuthCookies sets access and refresh token cookies
func (h *AuthHandler) setAuthCookies(c *fiber.Ctx, accessToken, refreshToken string) {
	// Access token cookie (shorter expiry)
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   h.cfg.JWT.AccessTokenMins * 60, // Convert minutes to seconds
		Secure:   h.cfg.Cookie.Secure,
		HTTPOnly: true,
		SameSite: h.cfg.Cookie.SameSite,
		Domain:   h.cfg.Cookie.Domain,
	})

	// Refresh token cookie (longer expiry)
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   h.cfg.JWT.RefreshTokenDays * 24 * 60 * 60, // Convert days to seconds
		Secure:   h.cfg.Cookie.Secure,
		HTTPOnly: true,
		SameSite: h.cfg.Cookie.SameSite,
		Domain:   h.cfg.Cookie.Domain,
	})
}

// clearAuthCookies clears auth cookies
func (h *AuthHandler) clearAuthCookies(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   h.cfg.Cookie.Secure,
		HTTPOnly: true,
		SameSite: h.cfg.Cookie.SameSite,
		Domain:   h.cfg.Cookie.Domain,
	})

	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
		Secure:   h.cfg.Cookie.Secure,
		HTTPOnly: true,
		SameSite: h.cfg.Cookie.SameSite,
		Domain:   h.cfg.Cookie.Domain,
	})
}
