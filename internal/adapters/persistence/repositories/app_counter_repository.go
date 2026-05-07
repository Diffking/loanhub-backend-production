package repositories

import (
	"context"
	"fmt"
	"time"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AppCounterRepository handles auto-numbering for loan application requests.
// Pattern: one row per (kind, year). When a new year starts, a new row is created.
//
// Usage:
//   - PeekNext(ctx, "loan_print")   // เลขที่ "ถัดไป" สำหรับ preview (ไม่ commit)
//   - IssueNext(ctx, "loan_print")  // รัน +1 และ return เลขใหม่ (atomic)
type AppCounterRepository struct {
	db *gorm.DB
}

// NewAppCounterRepository creates a new app counter repository
func NewAppCounterRepository(db *gorm.DB) *AppCounterRepository {
	return &AppCounterRepository{db: db}
}

// thaiYear converts current Gregorian year → Buddhist year
func thaiYear() int {
	return time.Now().Year() + 543
}

// FormatRequestNo returns the formatted request number string
// Example: kind="loan_print", year=2569, seq=1 → "00001/2569"
func FormatRequestNo(seq int, year int) string {
	return fmt.Sprintf("%05d/%d", seq, year)
}

// PeekNext returns the NEXT request number WITHOUT incrementing.
// Used for preview/display in form before user clicks "Print".
//
// Important: this is a non-binding hint; another officer might issue the same
// number between PeekNext and IssueNext. The frontend should re-display the
// committed number after IssueNext.
func (r *AppCounterRepository) PeekNext(ctx context.Context, kind string) (string, int, int, error) {
	year := thaiYear()
	var counter models.AppCounter
	err := r.db.WithContext(ctx).
		Where("kind = ? AND year = ?", kind, year).
		First(&counter).Error

	nextSeq := 1
	if err == nil {
		nextSeq = counter.LastSeq + 1
	} else if err != gorm.ErrRecordNotFound {
		return "", 0, 0, err
	}
	// if not found → nextSeq stays 1 (fresh start for the year)

	return FormatRequestNo(nextSeq, year), nextSeq, year, nil
}

// IssueNext atomically increments the counter and returns the issued number.
// Uses a transaction with FOR UPDATE locking to prevent race conditions
// when two officers click Print simultaneously.
//
// Returns: (formatted "00001/2569", seq=1, year=2569, error)
func (r *AppCounterRepository) IssueNext(ctx context.Context, kind string) (string, int, int, error) {
	year := thaiYear()
	var issuedSeq int

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter models.AppCounter

		// Lock the row for the (kind, year) pair
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("kind = ? AND year = ?", kind, year).
			First(&counter).Error

		if err == gorm.ErrRecordNotFound {
			// First request of the year → create row with seq=1
			counter = models.AppCounter{
				Kind:    kind,
				Year:    year,
				LastSeq: 1,
			}
			if err := tx.Create(&counter).Error; err != nil {
				return err
			}
			issuedSeq = 1
			return nil
		}
		if err != nil {
			return err
		}

		// Existing row → increment
		counter.LastSeq++
		issuedSeq = counter.LastSeq
		return tx.Save(&counter).Error
	})

	if err != nil {
		return "", 0, 0, err
	}

	return FormatRequestNo(issuedSeq, year), issuedSeq, year, nil
}

// Reset is for testing/admin purposes — resets the counter for a (kind, year).
// USE WITH CAUTION: this will cause duplicate request numbers if any have
// already been issued for that year.
func (r *AppCounterRepository) Reset(ctx context.Context, kind string, year int) error {
	return r.db.WithContext(ctx).
		Where("kind = ? AND year = ?", kind, year).
		Delete(&models.AppCounter{}).Error
}

// GetCurrent returns the current state for monitoring/display.
// Returns nil if no counter exists yet for this year.
func (r *AppCounterRepository) GetCurrent(ctx context.Context, kind string) (*models.AppCounter, error) {
	year := thaiYear()
	var counter models.AppCounter
	err := r.db.WithContext(ctx).
		Where("kind = ? AND year = ?", kind, year).
		First(&counter).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &counter, err
}
