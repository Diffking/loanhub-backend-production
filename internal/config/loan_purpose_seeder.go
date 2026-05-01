package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"spsc-loaneasy/internal/adapters/persistence/models"
	"spsc-loaneasy/internal/adapters/persistence/repositories"

	"gorm.io/gorm"
)

// SeedLoanPurposes loads FLOPRESN.txt → loan_purposes table.
// Idempotent: running multiple times only updates names of existing codes,
// inserts new codes. Does not delete codes that vanished from the file.
//
// File format: TSV with double-quoted fields, one record per line:
//
//	"001"	"รักษาพยาบาล"
//	"002"	"ใช้จ่ายส่วนตัว"
//
// Path: pass relative or absolute path. Common: "data/FLOPRESN.txt"
func SeedLoanPurposes(db *gorm.DB, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}
	defer file.Close()

	repo := repositories.NewLoanPurposeRepository(db)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	count := 0
	skipped := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		code, name, ok := parseFloprenLine(line)
		if !ok {
			skipped++
			log.Printf("⚠️  SeedLoanPurposes: skip malformed line: %q", line)
			continue
		}

		err := repo.UpsertByCode(nil, &models.LoanPurpose{
			Code:     code,
			Name:     name,
			IsActive: true,
		})
		if err != nil {
			return fmt.Errorf("upsert code %s: %w", code, err)
		}
		count++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", filePath, err)
	}

	log.Printf("✅ Loan purposes seeded: %d records (%d skipped)", count, skipped)
	return nil
}

// parseFloprenLine parses one line: "code"\t"name"
// returns code, name, ok
func parseFloprenLine(line string) (string, string, bool) {
	// Split on first tab
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	code := strings.Trim(strings.TrimSpace(parts[0]), `"`)
	name := strings.Trim(strings.TrimSpace(parts[1]), `"`)
	if code == "" || name == "" {
		return "", "", false
	}
	return code, name, true
}
