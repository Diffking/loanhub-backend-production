package services

import (
	"context"
	"errors"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/pkg/password"

	"gorm.io/gorm"
)

// User service errors
var (
	ErrUserNotFoundSvc    = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrOldPasswordWrong   = errors.New("old password is incorrect")
	ErrCannotDeleteSelf   = errors.New("cannot delete your own account")
	ErrCannotChangeOwnRole = errors.New("cannot change your own role")
)

// UserService handles user management business logic
type UserService struct {
	userRepo   repositories.UserRepository
	memberRepo repositories.MemberRepository
}

// NewUserService creates a new user service
func NewUserService(
	userRepo repositories.UserRepository,
	memberRepo repositories.MemberRepository,
) *UserService {
	return &UserService{
		userRepo:   userRepo,
		memberRepo: memberRepo,
	}
}

// ListUsersInput represents list users input
type ListUsersInput struct {
	Page   int
	Limit  int
	Search string
}

// ListUsersOutput represents list users output
type ListUsersOutput struct {
	Users      []*models.UserResponse `json:"users"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	Limit      int                    `json:"limit"`
	TotalPages int                    `json:"total_pages"`
}

// UpdateUserInput represents update user input (for admin)
type UpdateUserByAdminInput struct {
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	IsActive *bool   `json:"is_active"`
}

// UpdateProfileInput represents update profile input (for self)
type UpdateProfileInput struct {
	Email *string `json:"email"`
}

// ChangePasswordInput represents change password input
type ChangePasswordInput struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ListUsers lists all users with pagination
func (s *UserService) ListUsers(ctx context.Context, input *ListUsersInput) (*ListUsersOutput, error) {
	// Set defaults
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Limit < 1 {
		input.Limit = 10
	}
	if input.Limit > 100 {
		input.Limit = 100
	}

	offset := (input.Page - 1) * input.Limit

	users, total, err := s.userRepo.List(ctx, offset, input.Limit)
	if err != nil {
		return nil, err
	}

	// Convert to response format and add member info
	userResponses := make([]*models.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = user.ToResponse()
		
		// Get member info from flommast
		member, err := s.memberRepo.GetByMembNo(ctx, user.MembNo)
		if err == nil && member != nil {
			userResponses[i].FullName = member.FullName
			userResponses[i].DeptName = member.DeptName
		}
	}

	totalPages := int(total) / input.Limit
	if int(total)%input.Limit > 0 {
		totalPages++
	}

	return &ListUsersOutput{
		Users:      userResponses,
		Total:      total,
		Page:       input.Page,
		Limit:      input.Limit,
		TotalPages: totalPages,
	}, nil
}

// GetUserByID gets a user by ID
func (s *UserService) GetUserByID(ctx context.Context, id uint) (*models.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFoundSvc
		}
		return nil, err
	}

	response := user.ToResponse()

	// Get member info
	member, err := s.memberRepo.GetByMembNo(ctx, user.MembNo)
	if err == nil && member != nil {
		response.FullName = member.FullName
		response.DeptName = member.DeptName
	}

	return response, nil
}

// UpdateUserByAdmin updates a user by admin
func (s *UserService) UpdateUserByAdmin(ctx context.Context, id uint, adminID uint, input *UpdateUserByAdminInput) (*models.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFoundSvc
		}
		return nil, err
	}

	// Prevent admin from changing own role
	if id == adminID && input.Role != nil {
		return nil, ErrCannotChangeOwnRole
	}

	// Update fields
	if input.Email != nil && *input.Email != user.Email {
		// Check if email already exists
		exists, _ := s.userRepo.ExistsByEmail(ctx, *input.Email)
		if exists {
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *input.Email
	}

	if input.Role != nil {
		// Validate role
		if *input.Role != "USER" && *input.Role != "OFFICER" && *input.Role != "ADMIN" {
			return nil, errors.New("invalid role")
		}
		user.Role = *input.Role
	}

	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	response := user.ToResponse()

	// Get member info
	member, _ := s.memberRepo.GetByMembNo(ctx, user.MembNo)
	if member != nil {
		response.FullName = member.FullName
		response.DeptName = member.DeptName
	}

	return response, nil
}

// DeleteUser deletes a user (soft delete)
func (s *UserService) DeleteUser(ctx context.Context, id uint, adminID uint) error {
	// Prevent admin from deleting self
	if id == adminID {
		return ErrCannotDeleteSelf
	}

	// Check if user exists
	_, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFoundSvc
		}
		return err
	}

	return s.userRepo.Delete(ctx, id)
}

// GetProfile gets own profile
func (s *UserService) GetProfile(ctx context.Context, userID uint) (*models.UserResponse, error) {
	return s.GetUserByID(ctx, userID)
}

// UpdateProfile updates own profile
func (s *UserService) UpdateProfile(ctx context.Context, userID uint, input *UpdateProfileInput) (*models.UserResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFoundSvc
	}

	if input.Email != nil && *input.Email != user.Email {
		// Check if email already exists
		exists, _ := s.userRepo.ExistsByEmail(ctx, *input.Email)
		if exists {
			return nil, ErrEmailAlreadyExists
		}
		user.Email = *input.Email
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	response := user.ToResponse()

	// Get member info
	member, _ := s.memberRepo.GetByMembNo(ctx, user.MembNo)
	if member != nil {
		response.FullName = member.FullName
		response.DeptName = member.DeptName
	}

	return response, nil
}

// ChangePassword changes user's password
func (s *UserService) ChangePassword(ctx context.Context, userID uint, input *ChangePasswordInput) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFoundSvc
	}

	// Verify old password
	if !password.Verify(input.OldPassword, user.Password) {
		return ErrOldPasswordWrong
	}

	// Validate new password
	if len(input.NewPassword) < 8 {
		return errors.New("new password must be at least 8 characters")
	}

	// Hash new password
	hashedPassword, err := password.Hash(input.NewPassword)
	if err != nil {
		return err
	}

	user.Password = hashedPassword
	return s.userRepo.Update(ctx, user)
}

// SetUserRole sets user role (for admin seeding)
func (s *UserService) SetUserRole(ctx context.Context, userID uint, role string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	user.Role = role
	return s.userRepo.Update(ctx, user)
}
