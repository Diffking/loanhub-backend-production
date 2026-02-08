package config

import (
	"log"

	"spsc-loaneasy/internal/adapters/persistence/models"

	"gorm.io/gorm"
)

func SeedQueueData(db *gorm.DB) error {
	if err := seedBranches(db); err != nil {
		return err
	}
	if err := seedServiceTypes(db); err != nil {
		return err
	}
	if err := seedServiceCounters(db); err != nil {
		return err
	}
	if err := seedQueueConfig(db); err != nil {
		return err
	}
	log.Println("✅ Queue data seeded successfully")
	return nil
}

func seedBranches(db *gorm.DB) error {
	addr := "สหกรณ์ออมทรัพย์สาธารณสุขสงขลา"
	openTime := "08:30"
	closeTime := "16:30"
	schedNote := "ให้บริการตามตารางที่แจ้ง"

	branches := []models.Branch{
		{Code: "HQ", Name: "สำนักงานสหกรณ์ออมทรัพย์สาธารณสุขสงขลา", BranchType: "OFFICE", Address: &addr, OpenTime: &openTime, CloseTime: &closeTime, IsActive: true},
		{Code: "MOB01", Name: "รถตู้โมบาย 1", BranchType: "MOBILE", ScheduleNote: &schedNote, IsActive: true},
	}

	for _, b := range branches {
		var existing models.Branch
		if err := db.Where("code = ?", b.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&b).Error; err != nil {
					return err
				}
				log.Printf("   Created branch: %s", b.Code)
			}
		}
	}
	return nil
}

func seedServiceTypes(db *gorm.DB) error {
	serviceTypes := []models.ServiceType{
		{Code: "LOAN", Name: "บริการกู้เงิน", Description: "บริการสินเชื่อและการกู้เงิน", Icon: "banknotes", Color: "#0d9488", DisplayOrder: 1, IsActive: true},
		{Code: "DEPOSIT", Name: "บริการฝากเงิน", Description: "บริการฝากเงินและถอนเงิน", Icon: "piggy-bank", Color: "#2563eb", DisplayOrder: 2, IsActive: true},
		{Code: "GENERAL", Name: "บริการทั่วไป", Description: "บริการทั่วไปอื่นๆ", Icon: "clipboard", Color: "#7c3aed", DisplayOrder: 3, IsActive: true},
	}

	for _, st := range serviceTypes {
		var existing models.ServiceType
		if err := db.Where("code = ?", st.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&st).Error; err != nil {
					return err
				}
				log.Printf("   Created service_type: %s", st.Code)
			}
		}
	}
	return nil
}

func seedServiceCounters(db *gorm.DB) error {
	var hq models.Branch
	if err := db.Where("code = ?", "HQ").First(&hq).Error; err != nil {
		log.Printf("⚠️ Skipping counter seed: branch HQ not found")
		return nil
	}

	var loanST, depositST, generalST models.ServiceType
	db.Where("code = ?", "LOAN").First(&loanST)
	db.Where("code = ?", "DEPOSIT").First(&depositST)
	db.Where("code = ?", "GENERAL").First(&generalST)

	type counterDef struct {
		Number        int
		Name          string
		ServiceTypeID uint
	}

	counters := []counterDef{
		{1, "ช่อง 1 — กู้เงิน", loanST.ID},
		{2, "ช่อง 2 — กู้เงิน", loanST.ID},
		{3, "ช่อง 3 — ฝากเงิน", depositST.ID},
		{4, "ช่อง 4 — ฝากเงิน", depositST.ID},
		{5, "ช่อง 5 — ทั่วไป", generalST.ID},
	}

	for _, c := range counters {
		var existing models.ServiceCounter
		if err := db.Where("branch_id = ? AND counter_number = ?", hq.ID, c.Number).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				counter := models.ServiceCounter{BranchID: hq.ID, ServiceTypeID: c.ServiceTypeID, CounterNumber: c.Number, CounterName: c.Name, Status: "CLOSED", IsActive: true}
				if err := db.Create(&counter).Error; err != nil {
					return err
				}
				log.Printf("   Created counter: HQ #%d", c.Number)
			}
		}
	}
	return nil
}

func seedQueueConfig(db *gorm.DB) error {
	var hq models.Branch
	if err := db.Where("code = ?", "HQ").First(&hq).Error; err != nil {
		log.Printf("⚠️ Skipping config seed: branch HQ not found")
		return nil
	}

	type configDef struct {
		Key   string
		Value string
		Desc  string
	}

	configs := []configDef{
		{"walkin_prefix", "Q", "อักษรนำหน้าคิว Walk-in"},
		{"booking_prefix", "B", "อักษรนำหน้าคิว Booking"},
		{"avg_service_minutes", "15", "เวลาเฉลี่ยต่อคิว (นาที)"},
		{"max_skip_count", "3", "ข้ามได้สูงสุดกี่ครั้ง"},
		{"notify_before_queue", "3", "แจ้งเตือนก่อนถึงคิวกี่คิว"},
		{"booking_advance_days", "7", "จองล่วงหน้าได้กี่วัน"},
		{"slot_duration_min", "30", "ช่วงเวลาต่อ slot (นาที)"},
		{"auto_cancel_minutes", "30", "ยกเลิก Booking ถ้าไม่มาภายในกี่นาที"},
		{"notify_day_before", "18:00", "เวลาส่งแจ้งเตือนล่วงหน้า 1 วัน"},
	}

	for _, c := range configs {
		var existing models.QueueConfig
		if err := db.Where("branch_id = ? AND config_key = ?", hq.ID, c.Key).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				cfg := models.QueueConfig{BranchID: hq.ID, ConfigKey: c.Key, ConfigValue: c.Value, Description: c.Desc}
				if err := db.Create(&cfg).Error; err != nil {
					return err
				}
				log.Printf("   Created config: %s = %s", c.Key, c.Value)
			}
		}
	}
	return nil
}
