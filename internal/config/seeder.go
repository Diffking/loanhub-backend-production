package config

import (
	"log"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/pkg/password"

	"gorm.io/gorm"
)

// Seeder handles database seeding
type Seeder struct {
	db *gorm.DB
}

// NewSeeder creates a new seeder instance
func NewSeeder(db *gorm.DB) *Seeder {
	return &Seeder{db: db}
}

// Run executes all seeders
func (s *Seeder) Run() error {
	log.Println("🌱 Running database seeders...")

	if err := s.seedAdminUser(); err != nil {
		log.Printf("⚠️ Admin seeder skipped: %v", err)
	}

	log.Println("✅ Database seeding completed")
	return nil
}

// seedAdminUser seeds default admin user
// This is for development/testing only
// In production, create admin through secure process
func (s *Seeder) seedAdminUser() error {
	// Check if admin already exists
	var count int64
	s.db.Model(&models.User{}).Where("role = ?", "ADMIN").Count(&count)
	if count > 0 {
		return nil // Admin already exists
	}

	// Note: In real scenario, admin should have a valid MAST_MEMB_NO from flommast
	// This is just a placeholder for development
	hashedPassword, err := password.Hash("admin123456")
	if err != nil {
		return err
	}

	admin := &models.User{
		MembNo:   "ADMIN001", // Placeholder - should be valid member number
		Username: "admin",
		Email:    "admin@spsc.or.th",
		Password: hashedPassword,
		Role:     "ADMIN",
		IsActive: true,
	}

	// Only create if there's a matching flommast record
	// For now, skip if no flommast record (will need manual setup)
	var memberExists int64
	s.db.Table("flommast").Where("MAST_MEMB_NO = ?", admin.MembNo).Count(&memberExists)
	if memberExists == 0 {
		log.Println("⚠️ Skipping admin seed: No matching flommast record")
		log.Println("   Create admin user manually with valid member number")
		return nil
	}

	if err := s.db.Create(admin).Error; err != nil {
		return err
	}

	log.Printf("✅ Admin user created: %s", admin.Username)
	return nil
}
