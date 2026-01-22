package handlers

import (
	"errors"
	"strconv"

	"spsc-loaneasy/internal/core/services"
	"spsc-loaneasy/internal/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// UserHandler handles user management endpoints
type UserHandler struct {
	userService *services.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// ListUsers handles listing all users (Admin only)
// @Summary List all users
// @Description Get a paginated list of all users (Admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(10)
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /users [get]
func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	input := &services.ListUsersInput{
		Page:  page,
		Limit: limit,
	}

	result, err := h.userService.ListUsers(c.Context(), input)
	if err != nil {
		return response.InternalServerError(c, "Failed to list users")
	}

	return response.Success(c, "Users retrieved successfully", result)
}

// GetUser handles getting a user by ID (Admin only)
// @Summary Get user by ID
// @Description Get a specific user by ID (Admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	user, err := h.userService.GetUserByID(c.Context(), uint(id))
	if err != nil {
		if errors.Is(err, services.ErrUserNotFoundSvc) {
			return response.NotFound(c, "User not found")
		}
		return response.InternalServerError(c, "Failed to get user")
	}

	return response.Success(c, "User retrieved successfully", fiber.Map{
		"user": user,
	})
}

// UpdateUserRequest represents update user request body
type UpdateUserRequest struct {
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

// UpdateUser handles updating a user (Admin only)
// @Summary Update user
// @Description Update a user's information (Admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param body body UpdateUserRequest true "Update data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Get admin ID from context
	adminID, _ := c.Locals("userID").(uint)

	input := &services.UpdateUserByAdminInput{
		Email:    req.Email,
		Role:     req.Role,
		IsActive: req.IsActive,
	}

	user, err := h.userService.UpdateUserByAdmin(c.Context(), uint(id), adminID, input)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserNotFoundSvc):
			return response.NotFound(c, "User not found")
		case errors.Is(err, services.ErrEmailAlreadyExists):
			return response.Conflict(c, "Email already exists")
		case errors.Is(err, services.ErrCannotChangeOwnRole):
			return response.BadRequest(c, "Cannot change your own role")
		default:
			return response.InternalServerError(c, "Failed to update user")
		}
	}

	return response.Success(c, "User updated successfully", fiber.Map{
		"user": user,
	})
}

// DeleteUser handles deleting a user (Admin only)
// @Summary Delete user
// @Description Delete a user (soft delete) (Admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /users/{id} [delete]
func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	// Get admin ID from context
	adminID, _ := c.Locals("userID").(uint)

	err = h.userService.DeleteUser(c.Context(), uint(id), adminID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserNotFoundSvc):
			return response.NotFound(c, "User not found")
		case errors.Is(err, services.ErrCannotDeleteSelf):
			return response.BadRequest(c, "Cannot delete your own account")
		default:
			return response.InternalServerError(c, "Failed to delete user")
		}
	}

	return response.Success(c, "User deleted successfully", nil)
}

// GetProfile handles getting own profile
// @Summary Get own profile
// @Description Get the current user's profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /profile [get]
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	user, err := h.userService.GetProfile(c.Context(), userID)
	if err != nil {
		return response.InternalServerError(c, "Failed to get profile")
	}

	return response.Success(c, "Profile retrieved successfully", fiber.Map{
		"user": user,
	})
}

// UpdateProfileRequest represents update profile request body
type UpdateProfileRequest struct {
	Email *string `json:"email"`
}

// UpdateProfile handles updating own profile
// @Summary Update own profile
// @Description Update the current user's profile
// @Tags Profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body UpdateProfileRequest true "Update data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 409 {object} response.Response
// @Router /profile [put]
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	input := &services.UpdateProfileInput{
		Email: req.Email,
	}

	user, err := h.userService.UpdateProfile(c.Context(), userID, input)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyExists) {
			return response.Conflict(c, "Email already exists")
		}
		return response.InternalServerError(c, "Failed to update profile")
	}

	return response.Success(c, "Profile updated successfully", fiber.Map{
		"user": user,
	})
}

// ChangePasswordRequest represents change password request body
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword handles changing password
// @Summary Change password
// @Description Change the current user's password
// @Tags Profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ChangePasswordRequest true "Password data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Router /profile/password [put]
func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok {
		return response.Unauthorized(c, "Unauthorized")
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate
	if req.OldPassword == "" {
		return response.BadRequest(c, "Old password is required")
	}
	if req.NewPassword == "" {
		return response.BadRequest(c, "New password is required")
	}
	if len(req.NewPassword) < 8 {
		return response.BadRequest(c, "New password must be at least 8 characters")
	}

	input := &services.ChangePasswordInput{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}

	err := h.userService.ChangePassword(c.Context(), userID, input)
	if err != nil {
		if errors.Is(err, services.ErrOldPasswordWrong) {
			return response.BadRequest(c, "Old password is incorrect")
		}
		return response.InternalServerError(c, "Failed to change password")
	}

	return response.Success(c, "Password changed successfully", nil)
}

// SetUserRoleRequest represents set user role request
type SetUserRoleRequest struct {
	Role string `json:"role"`
}

// SetUserRole handles setting user role (Admin only)
// @Summary Set user role
// @Description Set a user's role (Admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param body body SetUserRoleRequest true "Role data"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /users/{id}/role [put]
func (h *UserHandler) SetUserRole(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid user ID")
	}

	var req SetUserRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "Invalid request body")
	}

	// Validate role
	if req.Role != "USER" && req.Role != "OFFICER" && req.Role != "ADMIN" {
		return response.BadRequest(c, "Invalid role. Must be USER, OFFICER, or ADMIN")
	}

	// Prevent changing own role
	adminID, _ := c.Locals("userID").(uint)
	if uint(id) == adminID {
		return response.BadRequest(c, "Cannot change your own role")
	}

	err = h.userService.SetUserRole(c.Context(), uint(id), req.Role)
	if err != nil {
		return response.InternalServerError(c, "Failed to set user role")
	}

	return response.Success(c, "User role updated successfully", nil)
}
