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

	// Phase 6: Update loan type codes + add FirstHome
	if err := updateLoanTypeCodes(db); err != nil {
		log.Printf("⚠️ Update loan type codes: %v", err)
	}

	// Phase 6: Seed Doc Items
	if err := seedDocItems(db); err != nil {
		log.Printf("⚠️ Seed doc items: %v", err)
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

// ============================================================
// Phase 6: Update Loan Type Codes + Add FirstHome
// ============================================================

func updateLoanTypeCodes(db *gorm.DB) error {
	// Update ORDINARY → สบ
	db.Model(&models.LoanType{}).Where("code = ?", "ORDINARY").Update("code", "สบ")

	// Update MULTIPURPOSE → อป
	db.Model(&models.LoanType{}).Where("code = ?", "MULTIPURPOSE").Update("code", "อป")

	// Add FirstHome loan type (ถ้ายังไม่มี)
	var existing models.LoanType
	if err := db.Unscoped().Where("code = ?", "FirstHome").First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			firstHome := models.LoanType{
				Code:         "FirstHome",
				Name:         "กู้พิเศษเพื่อการเคหะบ้านหลังแรก",
				Description:  "ประเภทเงินกู้พิเศษ - เพื่อการเคหะ(บ้านหลังแรก)",
				InterestRate: 0.00,
				IsActive:     true,
			}
			if err := db.Create(&firstHome).Error; err != nil {
				return err
			}
			log.Printf("   Created loan_type: %s", firstHome.Name)
		}
	}

	log.Println("   ✅ Loan type codes updated")
	return nil
}

// ============================================================
// Phase 6: Seed Doc Items (รายการเอกสารแต่ละตัว ผูก loan_type)
// ============================================================

func seedDocItems(db *gorm.DB) error {
	// Helper: ดึง loan_type ID จาก code
	getLoanTypeID := func(code string) uint {
		var lt models.LoanType
		if err := db.Where("code = ?", code).First(&lt).Error; err != nil {
			log.Printf("   ⚠️ loan_type not found: %s", code)
			return 0
		}
		return lt.ID
	}

	// รายการเอกสาร — กู้พิเศษเพื่อการทั่วไป (พท)
	ptID := getLoanTypeID("พท")
	if ptID > 0 {
		seedDocItemsForType(db, ptID, []docItemSeed{
			{Code: "PT-01", Name: "แบบฟอร์มคำขอกู้พิเศษ", IsRequired: true, Sort: 1},
			{Code: "PT-02", Name: "แบบฟอร์มให้ความยินยอมหักเงินชำระหนี้สหกรณ์", IsRequired: true, Sort: 2},
			{Code: "PT-03", Name: "สน.บัตรประชาชนผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 3},
			{Code: "PT-04", Name: "สน.ทะเบียนบ้านผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 4},
			{Code: "PT-05", Name: "สน.ทะเบียนสมรส / สน.ใบหย่า / สน.ใบมรณะบัตร 2 ใบ", IsRequired: false, Sort: 5},
			{Code: "PT-06", Name: "สน.บัตรประชาชนผู้ขาย เจ้าของหลักทรัพย์ และคู่สมรส (ถ้ามี) 2 ใบ", IsRequired: false, Sort: 6},
			{Code: "PT-07", Name: "สลิปเงินเดือน และ กพ.7", IsRequired: true, Sort: 7},
			{Code: "PT-08", Name: "สำเนาโฉนดเท่าฉบับจริง (หน้า-หลัง)", IsRequired: true, Sort: 8},
			{Code: "PT-09", Name: "ใบระวางที่ดิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 9},
			{Code: "PT-10", Name: "หนังสือรับรองราคาประเมิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 10},
			{Code: "PT-11", Name: "สัญญาจะซื้อจะขาย", IsRequired: true, Sort: 11},
			{Code: "PT-12", Name: "แปลนบ้าน (กรณีสร้างบ้าน)", IsRequired: false, Sort: 12},
			{Code: "PT-13", Name: "สัญญาก่อสร้าง/เบิกงวด", IsRequired: false, Sort: 13},
		})
	}

	// รายการเอกสาร — กู้พิเศษเพื่อซื้อที่ดิน (พด)
	pdID := getLoanTypeID("พด")
	if pdID > 0 {
		seedDocItemsForType(db, pdID, []docItemSeed{
			{Code: "PD-01", Name: "แบบฟอร์มคำขอกู้พิเศษ", IsRequired: true, Sort: 1},
			{Code: "PD-02", Name: "แบบฟอร์มให้ความยินยอมหักเงินชำระหนี้สหกรณ์", IsRequired: true, Sort: 2},
			{Code: "PD-03", Name: "สน.บัตรประชาชนผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 3},
			{Code: "PD-04", Name: "สน.ทะเบียนบ้านผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 4},
			{Code: "PD-05", Name: "สน.ทะเบียนสมรส / สน.ใบหย่า / สน.ใบมรณะบัตร 2 ใบ", IsRequired: false, Sort: 5},
			{Code: "PD-06", Name: "สน.บัตรประชาชนผู้ขาย เจ้าของหลักทรัพย์ และคู่สมรส (ถ้ามี) 2 ใบ", IsRequired: false, Sort: 6},
			{Code: "PD-07", Name: "สลิปเงินเดือน และ กพ.7", IsRequired: true, Sort: 7},
			{Code: "PD-08", Name: "สำเนาโฉนดเท่าฉบับจริง (หน้า-หลัง)", IsRequired: true, Sort: 8},
			{Code: "PD-09", Name: "ใบระวางที่ดิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 9},
			{Code: "PD-10", Name: "หนังสือรับรองราคาประเมิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 10},
			{Code: "PD-11", Name: "สัญญาจะซื้อจะขายที่ดิน", IsRequired: true, Sort: 11},
		})
	}

	// รายการเอกสาร — กู้พิเศษเพื่อการเคหะ (พค)
	pkID := getLoanTypeID("พค")
	if pkID > 0 {
		seedDocItemsForType(db, pkID, []docItemSeed{
			{Code: "PK-01", Name: "แบบฟอร์มคำขอกู้พิเศษ", IsRequired: true, Sort: 1},
			{Code: "PK-02", Name: "แบบฟอร์มให้ความยินยอมหักเงินชำระหนี้สหกรณ์", IsRequired: true, Sort: 2},
			{Code: "PK-03", Name: "สน.บัตรประชาชนผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 3},
			{Code: "PK-04", Name: "สน.ทะเบียนบ้านผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 4},
			{Code: "PK-05", Name: "สน.ทะเบียนสมรส / สน.ใบหย่า / สน.ใบมรณะบัตร 2 ใบ", IsRequired: false, Sort: 5},
			{Code: "PK-06", Name: "สน.บัตรประชาชนผู้ขาย เจ้าของหลักทรัพย์ และคู่สมรส (ถ้ามี) 2 ใบ", IsRequired: false, Sort: 6},
			{Code: "PK-07", Name: "สลิปเงินเดือน และ กพ.7", IsRequired: true, Sort: 7},
			{Code: "PK-08", Name: "สำเนาโฉนดเท่าฉบับจริง (หน้า-หลัง)", IsRequired: true, Sort: 8},
			{Code: "PK-09", Name: "ใบระวางที่ดิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 9},
			{Code: "PK-10", Name: "หนังสือรับรองราคาประเมิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 10},
			{Code: "PK-11", Name: "สัญญาจะซื้อจะขาย", IsRequired: true, Sort: 11},
			{Code: "PK-12", Name: "แปลนบ้าน (กรณีสร้างบ้าน)", IsRequired: false, Sort: 12},
			{Code: "PK-13", Name: "สัญญาก่อสร้าง/เบิกงวด", IsRequired: false, Sort: 13},
		})
	}

	// รายการเอกสาร — กู้พิเศษเพื่อการเคหะบ้านหลังแรก (FirstHome)
	fhID := getLoanTypeID("FirstHome")
	if fhID > 0 {
		seedDocItemsForType(db, fhID, []docItemSeed{
			{Code: "FH-01", Name: "แบบฟอร์มคำขอกู้พิเศษ", IsRequired: true, Sort: 1},
			{Code: "FH-02", Name: "แบบฟอร์มให้ความยินยอมหักเงินชำระหนี้สหกรณ์", IsRequired: true, Sort: 2},
			{Code: "FH-03", Name: "สน.บัตรประชาชนผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 3},
			{Code: "FH-04", Name: "สน.ทะเบียนบ้านผู้กู้และคู่สมรส 2 ใบ", IsRequired: true, Sort: 4},
			{Code: "FH-05", Name: "สน.ทะเบียนสมรส / สน.ใบหย่า / สน.ใบมรณะบัตร 2 ใบ", IsRequired: false, Sort: 5},
			{Code: "FH-06", Name: "สลิปเงินเดือน และ กพ.7", IsRequired: true, Sort: 6},
			{Code: "FH-07", Name: "ใบระวางที่ดิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 7},
			{Code: "FH-08", Name: "หนังสือรับรองราคาประเมิน (จากสำนักงานที่ดิน)", IsRequired: true, Sort: 8},
			{Code: "FH-09", Name: "แปลนบ้าน", IsRequired: true, Sort: 9},
			{Code: "FH-10", Name: "สัญญาจ้างก่อสร้าง / การเบิกงวดงาน", IsRequired: false, Sort: 10},
			{Code: "FH-11", Name: "ใบขออนุญาตก่อสร้าง", IsRequired: true, Sort: 11},
			{Code: "FH-12", Name: "สัญญาจะซื้อจะขาย", IsRequired: true, Sort: 12},
			{Code: "FH-13", Name: "สำเนาทะเบียนบ้านหลังที่ต้องการซื้อ", IsRequired: true, Sort: 13},
		})
	}

	// รายการเอกสาร — กู้สามัญ (สบ)
	sbID := getLoanTypeID("สบ")
	if sbID > 0 {
		seedDocItemsForType(db, sbID, []docItemSeed{
			{Code: "SB-01", Name: "เอกสารคำขอกู้สามัญ", IsRequired: true, Sort: 1},
			{Code: "SB-02", Name: "สำเนาบัตรประชาชนผู้กู้/คู่สมรส", IsRequired: true, Sort: 2},
			{Code: "SB-03", Name: "สลิปเงินเดือนเดือนล่าสุด", IsRequired: true, Sort: 3},
			{Code: "SB-04", Name: "สำเนาโฉนดเท่าฉบับจริง", IsRequired: true, Sort: 4},
			{Code: "SB-05", Name: "สำเนาบัตรประชาชนและทะเบียนบ้านของเจ้าของหลักทรัพย์", IsRequired: true, Sort: 5},
			{Code: "SB-06", Name: "ใบทะเบียนสมรส/หย่า", IsRequired: false, Sort: 6},
			{Code: "SB-07", Name: "หลักฐานประกอบวัตถุประสงค์การกู้", IsRequired: true, Sort: 7},
		})
	}

	// รายการเอกสาร — กู้สามัญเอนกประสงค์ (อป)
	apID := getLoanTypeID("อป")
	if apID > 0 {
		seedDocItemsForType(db, apID, []docItemSeed{
			{Code: "AP-01", Name: "เอกสารคำขอกู้เอนกประสงค์", IsRequired: true, Sort: 1},
			{Code: "AP-02", Name: "สำเนาบัตรประชาชนผู้กู้/คู่สมรส", IsRequired: true, Sort: 2},
			{Code: "AP-03", Name: "สลิปเงินเดือนเดือนล่าสุด", IsRequired: true, Sort: 3},
			{Code: "AP-04", Name: "สำเนาโฉนดเท่าฉบับจริง", IsRequired: true, Sort: 4},
			{Code: "AP-05", Name: "สำเนาบัตรประชาชนและทะเบียนบ้านของเจ้าของหลักทรัพย์", IsRequired: true, Sort: 5},
			{Code: "AP-06", Name: "ใบทะเบียนสมรส/หย่า", IsRequired: false, Sort: 6},
			{Code: "AP-07", Name: "หลักฐานประกอบวัตถุประสงค์การกู้", IsRequired: true, Sort: 7},
		})
	}

	log.Println("   ✅ Doc items seeded")
	return nil
}

// docItemSeed helper struct
type docItemSeed struct {
	Code       string
	Name       string
	IsRequired bool
	Sort       int
}

// seedDocItemsForType seeds doc items for a specific loan type
func seedDocItemsForType(db *gorm.DB, loanTypeID uint, items []docItemSeed) {
	for _, item := range items {
		var existing models.DocItem
		if err := db.Unscoped().Where("loan_type_id = ? AND code = ?", loanTypeID, item.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				docItem := models.DocItem{
					LoanTypeID: loanTypeID,
					Code:       item.Code,
					Name:       item.Name,
					IsRequired: item.IsRequired,
					SortOrder:  item.Sort,
					IsActive:   true,
				}
				if err := db.Create(&docItem).Error; err != nil {
					log.Printf("   ⚠️ Failed to create doc_item %s: %v", item.Code, err)
				}
			}
		}
	}
}
