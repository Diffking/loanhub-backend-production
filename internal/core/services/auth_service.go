package services

import (
	"context"
	"errors"
	"log"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"
	"spsc-loaneasy/internal/config"
	"spsc-loaneasy/internal/pkg/jwt"
	"spsc-loaneasy/internal/pkg/password"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Auth errors
var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrMemberNotFound     = errors.New("member not found in flommast")
	ErrMemberAlreadyUsed  = errors.New("member number already registered")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrUserInactive       = errors.New("user account is inactive")
)

// AuthService handles authentication business logic
type AuthService struct {
	userRepo         repositories.UserRepository
	refreshTokenRepo repositories.RefreshTokenRepository
	memberRepo       repositories.MemberRepository
	cfg              *config.Config
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo repositories.UserRepository,
	refreshTokenRepo repositories.RefreshTokenRepository,
	memberRepo repositories.MemberRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		memberRepo:       memberRepo,
		cfg:              cfg,
	}
}

// RegisterInput represents registration input
type RegisterInput struct {
	MembNo   string `json:"memb_no" validate:"required"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginInput represents login input
type LoginInput struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	User         *models.UserResponse `json:"user"`
	AccessToken  string               `json:"access_token"`
	RefreshToken string               `json:"refresh_token"`
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, input *RegisterInput) (*AuthResponse, error) {
	// 1. Validate member exists in flommast
	member, err := s.memberRepo.GetByMembNo(ctx, input.MembNo)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, err
	}

	// 2. Check if member number already registered
	exists, err := s.userRepo.ExistsByMembNo(ctx, input.MembNo)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrMemberAlreadyUsed
	}

	// 3. Check if username already exists
	exists, err = s.userRepo.ExistsByUsername(ctx, input.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	// 4. Check if email already exists
	exists, err = s.userRepo.ExistsByEmail(ctx, input.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	// 5. Hash password
	hashedPassword, err := password.Hash(input.Password)
	if err != nil {
		return nil, err
	}

	// 6. Create user
	user := &models.User{
		MembNo:   input.MembNo,
		Username: input.Username,
		Email:    input.Email,
		Password: hashedPassword,
		Role:     "USER",
		IsActive: true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 7. Generate tokens
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, err
	}

	// 8. Store refresh token
	if err := s.storeRefreshToken(ctx, user.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	// 9. Build response
	userResponse := user.ToResponse()
	userResponse.FullName = member.FullName
	userResponse.DeptName = member.DeptName

	log.Printf("✅ User registered: %s (MembNo: %s)", user.Username, user.MembNo)

	return &AuthResponse{
		User:         userResponse,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, input *LoginInput) (*AuthResponse, error) {
	// 1. Find user by username
	user, err := s.userRepo.GetByUsername(ctx, input.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 2. Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// 3. Verify password
	if !password.Verify(input.Password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	// 4. Get member info from flommast
	member, _ := s.memberRepo.GetByMembNo(ctx, user.MembNo)

	// 5. Generate tokens
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, err
	}

	// 6. Store refresh token
	if err := s.storeRefreshToken(ctx, user.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	// 7. Build response
	userResponse := user.ToResponse()
	if member != nil {
		userResponse.FullName = member.FullName
		userResponse.DeptName = member.DeptName
	}

	log.Printf("✅ User logged in: %s", user.Username)

	return &AuthResponse{
		User:         userResponse,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

// RefreshToken refreshes the access token using refresh token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// 1. Validate refresh token JWT
	claims, err := jwt.ValidateRefreshToken(refreshToken, s.cfg.JWT.RefreshSecret)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	// 2. Hash the token to find in DB
	tokenHash := password.HashToken(refreshToken)

	// 3. Find token in DB
	storedToken, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, err
	}

	// 4. Check if token is revoked
	if storedToken.IsRevoked() {
		return nil, ErrTokenRevoked
	}

	// 5. Check if token is expired
	if storedToken.IsExpired() {
		return nil, ErrTokenExpired
	}

	// 6. Get user
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// 7. Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// 8. Revoke old refresh token (Token Rotation)
	if err := s.refreshTokenRepo.Revoke(ctx, storedToken.ID); err != nil {
		return nil, err
	}

	// 9. Generate new tokens
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, err
	}

	// 10. Store new refresh token
	if err := s.storeRefreshToken(ctx, user.ID, tokens.RefreshToken); err != nil {
		return nil, err
	}

	// 11. Get member info
	member, _ := s.memberRepo.GetByMembNo(ctx, user.MembNo)

	// 12. Build response
	userResponse := user.ToResponse()
	if member != nil {
		userResponse.FullName = member.FullName
		userResponse.DeptName = member.DeptName
	}

	log.Printf("✅ Token refreshed for user: %s", user.Username)

	return &AuthResponse{
		User:         userResponse,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	}, nil
}

// Logout revokes the refresh token
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	// Hash the token
	tokenHash := password.HashToken(refreshToken)

	// Revoke the token
	if err := s.refreshTokenRepo.RevokeByTokenHash(ctx, tokenHash); err != nil {
		return err
	}

	log.Printf("✅ User logged out")
	return nil
}

// LogoutAll revokes all refresh tokens for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID uint) error {
	if err := s.refreshTokenRepo.RevokeAllByUserID(ctx, userID); err != nil {
		return err
	}

	log.Printf("✅ All sessions revoked for user ID: %d", userID)
	return nil
}

// ValidateAccessToken validates an access token
func (s *AuthService) ValidateAccessToken(accessToken string) (*jwt.Claims, error) {
	return jwt.ValidateAccessToken(accessToken, s.cfg.JWT.Secret)
}

// GetUserByID gets a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID uint) (*models.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

// generateTokens generates access and refresh tokens
func (s *AuthService) generateTokens(user *models.User) (*TokenPair, error) {
	// Generate access token
	accessToken, err := jwt.GenerateAccessToken(
		user.ID,
		user.MembNo,
		user.Username,
		user.Role,
		s.cfg.JWT.Secret,
		s.cfg.JWT.AccessTokenMins,
	)
	if err != nil {
		return nil, err
	}

	// Generate unique token ID
	tokenID := uuid.New().String()

	// Generate refresh token
	refreshToken, err := jwt.GenerateRefreshToken(
		user.ID,
		tokenID,
		s.cfg.JWT.RefreshSecret,
		s.cfg.JWT.RefreshTokenDays,
	)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// storeRefreshToken stores a refresh token in the database
func (s *AuthService) storeRefreshToken(ctx context.Context, userID uint, refreshToken string) error {
	tokenHash := password.HashToken(refreshToken)
	expiresAt := jwt.GetExpiryTime(s.cfg.JWT.RefreshTokenDays)

	token := &models.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}

	return s.refreshTokenRepo.Create(ctx, token)
}
