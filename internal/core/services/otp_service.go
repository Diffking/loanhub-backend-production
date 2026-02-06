package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"gorm.io/gorm"
)

// ============================================================
// OTP Service - ระบบ OTP ยืนยันเบอร์โทร
// ============================================================

// OTPEntry represents a single OTP record in memory
type OTPEntry struct {
	Code      string
	Phone     string
	ExpiresAt time.Time
	Attempts  int // จำนวนครั้งที่ใส่ผิด
	Verified  bool
}

// OTPService handles OTP generation and verification
type OTPService struct {
	db    *gorm.DB
	store map[string]*OTPEntry // key = line_user_id
	mu    sync.RWMutex
}

// NewOTPService creates a new OTP service
func NewOTPService(db *gorm.DB) *OTPService {
	svc := &OTPService{
		db:    db,
		store: make(map[string]*OTPEntry),
	}
	// Cleanup expired OTPs every 5 minutes
	go svc.cleanupLoop()
	return svc
}

// GenerateOTP creates a new 6-digit OTP for a LINE user
// Returns the OTP code (to be sent via SMS)
func (s *OTPService) GenerateOTP(lineUserID, phone string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check rate limit - ไม่ให้ขอ OTP บ่อยเกินไป (1 นาที)
	if existing, ok := s.store[lineUserID]; ok {
		timeSinceCreated := time.Until(existing.ExpiresAt) // remaining time
		// OTP มีอายุ 5 นาที ถ้ายังเหลือ > 4 นาที แสดงว่าเพิ่งขอไป
		if timeSinceCreated > 4*time.Minute {
			return "", fmt.Errorf("กรุณารอ 1 นาที ก่อนขอ OTP ใหม่")
		}
	}

	// Generate 6-digit random OTP
	code, err := generateSecureOTP(6)
	if err != nil {
		return "", fmt.Errorf("ไม่สามารถสร้าง OTP ได้: %w", err)
	}

	// Store OTP (expires in 5 minutes)
	s.store[lineUserID] = &OTPEntry{
		Code:      code,
		Phone:     phone,
		ExpiresAt: time.Now().Add(5 * time.Minute),
		Attempts:  0,
		Verified:  false,
	}

	return code, nil
}

// VerifyOTP checks if the provided OTP is valid
func (s *OTPService) VerifyOTP(lineUserID, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.store[lineUserID]
	if !ok {
		return fmt.Errorf("ไม่พบ OTP กรุณาขอ OTP ใหม่")
	}

	// Check expiry
	if time.Now().After(entry.ExpiresAt) {
		delete(s.store, lineUserID)
		return fmt.Errorf("OTP หมดอายุ กรุณาขอ OTP ใหม่")
	}

	// Check attempts (max 5)
	if entry.Attempts >= 5 {
		delete(s.store, lineUserID)
		return fmt.Errorf("ใส่ OTP ผิดเกินจำนวนครั้ง กรุณาขอ OTP ใหม่")
	}

	// Verify code
	entry.Attempts++
	if entry.Code != code {
		return fmt.Errorf("OTP ไม่ถูกต้อง (เหลืออีก %d ครั้ง)", 5-entry.Attempts)
	}

	// Success - mark as verified
	entry.Verified = true
	return nil
}

// IsVerified checks if OTP was verified for a LINE user
func (s *OTPService) IsVerified(lineUserID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.store[lineUserID]
	if !ok {
		return false
	}
	return entry.Verified && time.Now().Before(entry.ExpiresAt)
}

// GetVerifiedPhone returns the phone that was verified
func (s *OTPService) GetVerifiedPhone(lineUserID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.store[lineUserID]
	if !ok {
		return ""
	}
	if entry.Verified {
		return entry.Phone
	}
	return ""
}

// ClearOTP removes OTP after successful registration/login
func (s *OTPService) ClearOTP(lineUserID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, lineUserID)
}

// cleanupLoop periodically removes expired OTPs
func (s *OTPService) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		for key, entry := range s.store {
			if time.Now().After(entry.ExpiresAt) {
				delete(s.store, key)
			}
		}
		s.mu.Unlock()
	}
}

// generateSecureOTP generates a cryptographically secure random OTP
func generateSecureOTP(length int) (string, error) {
	result := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		result += fmt.Sprintf("%d", n.Int64())
	}
	return result, nil
}
