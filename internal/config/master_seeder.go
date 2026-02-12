package config

import (
	"log"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

// SeedMasterData seeds initial master data
func SeedMasterData(db *gorm.DB) error {
	// Seed Loan Types
	if err := seedLoanTypes(db); err != nil {
		return err
	}

	// Seed Loan Steps
	if err := seedLoanSteps(db); err != nil {
		return err
	}

	// Seed Loan Docs
	if err := seedLoanDocs(db); err != nil {
		return err
	}

	// Seed Loan Appts
	if err := seedLoanAppts(db); err != nil {
		return err
	}

	log.Println("✅ Master data seeded successfully")
	return nil
}

func seedLoanTypes(db *gorm.DB) error {
	loanTypes := []models.LoanType{
		{
			Code:         "NORMAL",
			Name:         "สินเชื่อสามัญ",
			Description:  "สินเชื่อทั่วไปสำหรับสมาชิก",
			InterestRate: 6.50,
			IsActive:     true,
		},
		{
			Code:         "EMERGENCY",
			Name:         "สินเชื่อฉุกเฉิน",
			Description:  "สินเชื่อสำหรับกรณีฉุกเฉิน วงเงินไม่เกิน 100,000 บาท",
			InterestRate: 6.00,
			IsActive:     true,
		},
		{
			Code:         "SPECIAL",
			Name:         "สินเชื่อพิเศษ",
			Description:  "สินเชื่อพิเศษสำหรับสมาชิกที่มีหลักประกัน",
			InterestRate: 5.50,
			IsActive:     true,
		},
	}

	for _, lt := range loanTypes {
		var existing models.LoanType
		if err := db.Unscoped().Where("code = ?", lt.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&lt).Error; err != nil {
					return err
				}
				log.Printf("   Created loan_type: %s", lt.Name)
			}
		}
	}
	return nil
}

func seedLoanSteps(db *gorm.DB) error {
	loanSteps := []models.LoanStep{
		{
			Code:        "RECEIVED",
			Name:        "รับคำขอ",
			Description: "รับคำขอสินเชื่อจากสมาชิก",
			StepOrder:   1,
			Color:       "#2196F3",
			IsFinal:     false,
			IsActive:    true,
		},
		{
			Code:        "SURVEY",
			Name:        "รอลงพื้นที่ประเมิน",
			Description: "รอเจ้าหน้าที่ลงพื้นที่ประเมินหลักประกัน",
			StepOrder:   2,
			Color:       "#FF9800",
			IsFinal:     false,
			IsActive:    true,
		},
		{
			Code:        "PENDING_APPROVE",
			Name:        "รออนุมัติ",
			Description: "รอผู้มีอำนาจอนุมัติ",
			StepOrder:   3,
			Color:       "#9C27B0",
			IsFinal:     false,
			IsActive:    true,
		},
		{
			Code:        "APPROVED",
			Name:        "อนุมัติแล้ว/นัดจดจำนองทำสัญญา",
			Description: "อนุมัติแล้ว รอนัดจำนองและทำสัญญา",
			StepOrder:   4,
			Color:       "#4CAF50",
			IsFinal:     false,
			IsActive:    true,
		},
		{
			Code:        "REJECTED",
			Name:        "ปฏิเสธ/ยกเลิก",
			Description: "คำขอถูกปฏิเสธหรือยกเลิก",
			StepOrder:   5,
			Color:       "#F44336",
			IsFinal:     true,
			IsActive:    true,
		},
		{
			Code:        "COMPLETED",
			Name:        "สิ้นสุด",
			Description: "ดำเนินการเสร็จสิ้น ไม่แสดงในรายการติดตาม",
			StepOrder:   6,
			Color:       "#9E9E9E",
			IsFinal:     true,
			IsActive:    true,
		},
	}

	for _, ls := range loanSteps {
		var existing models.LoanStep
		if err := db.Unscoped().Where("code = ?", ls.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&ls).Error; err != nil {
					return err
				}
				log.Printf("   Created loan_step: %s", ls.Name)
			}
		}
	}
	return nil
}

func seedLoanDocs(db *gorm.DB) error {
	loanDocs := []models.LoanDoc{
		{
			Code:        "ID_CARD",
			Name:        "สำเนาบัตรประชาชน",
			Description: "สำเนาบัตรประชาชน รับรองสำเนาถูกต้อง",
			IsActive:    true,
		},
		{
			Code:        "HOUSE_REG",
			Name:        "สำเนาทะเบียนบ้าน",
			Description: "สำเนาทะเบียนบ้าน รับรองสำเนาถูกต้อง",
			IsActive:    true,
		},
		{
			Code:        "SALARY_SLIP",
			Name:        "สลิปเงินเดือน",
			Description: "สลิปเงินเดือน 3 เดือนล่าสุด",
			IsActive:    true,
		},
		{
			Code:        "BANK_STATEMENT",
			Name:        "Statement บัญชี",
			Description: "รายการเดินบัญชีย้อนหลัง 6 เดือน",
			IsActive:    true,
		},
		{
			Code:        "LAND_TITLE",
			Name:        "โฉนดที่ดิน",
			Description: "สำเนาโฉนดที่ดิน (กรณีใช้เป็นหลักประกัน)",
			IsActive:    true,
		},
		{
			Code:        "GUARANTOR_ID",
			Name:        "บัตรผู้ค้ำประกัน",
			Description: "สำเนาบัตรประชาชนผู้ค้ำประกัน",
			IsActive:    true,
		},
	}

	for _, ld := range loanDocs {
		var existing models.LoanDoc
		if err := db.Unscoped().Where("code = ?", ld.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&ld).Error; err != nil {
					return err
				}
				log.Printf("   Created loan_doc: %s", ld.Name)
			}
		}
	}
	return nil
}

func seedLoanAppts(db *gorm.DB) error {
	loanAppts := []models.LoanAppt{
		{
			Code:            "SUBMIT_DOC",
			Name:            "นัดส่งเอกสาร",
			Description:     "นัดสมาชิกมาส่งเอกสารเพิ่มเติม",
			DefaultLocation: "เคาน์เตอร์บริการ สหกรณ์ฯ",
			IsActive:        true,
		},
		{
			Code:            "SIGN_CONTRACT",
			Name:            "นัดเซ็นสัญญา",
			Description:     "นัดสมาชิกมาเซ็นสัญญากู้ยืม",
			DefaultLocation: "ห้องประชุม สหกรณ์ฯ",
			IsActive:        true,
		},
		{
			Code:            "CHECK_COLLATERAL",
			Name:            "นัดตรวจหลักประกัน",
			Description:     "นัดตรวจสอบหลักประกัน ณ สถานที่จริง",
			DefaultLocation: "ตามที่อยู่หลักประกัน",
			IsActive:        true,
		},
		{
			Code:            "RECEIVE_MONEY",
			Name:            "นัดรับเงิน",
			Description:     "นัดสมาชิกมารับเงินกู้",
			DefaultLocation: "ห้องการเงิน สหกรณ์ฯ",
			IsActive:        true,
		},
	}

	for _, la := range loanAppts {
		var existing models.LoanAppt
		if err := db.Unscoped().Where("code = ?", la.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&la).Error; err != nil {
					return err
				}
				log.Printf("   Created loan_appt: %s", la.Name)
			}
		}
	}
	return nil
}
