# Phase 6: Document Checklist Backend — Handoff

## สรุปการเปลี่ยนแปลง

### ไฟล์ใหม่ (4 ไฟล์)
| ไฟล์ | หน้าที่ |
|---|---|
| `handlers/doc_check_handler.go` | Handler: CRUD doc_items + Get/Update/Incomplete doc_checks |
| `repositories/doc_item_repository.go` | Repository: doc_items CRUD |
| `repositories/mortgage_doc_check_repository.go` | Repository: mortgage_doc_checks + InitChecklist + GetIncomplete |
| `services/doc_check_service.go` | Service: business logic (ไม่มี LINE — Frontend แสดง toast เอง) |

### ไฟล์แก้ไข (3 ไฟล์)
| ไฟล์ | สิ่งที่เปลี่ยน |
|---|---|
| `models/models.go` | เพิ่ม struct `DocItem`, `MortgageDocCheck`, `MortgageDocCheckResponse` + AutoMigrate |
| `config/master_seeder.go` | เพิ่ม `updateLoanTypeCodes()` + `seedDocItems()` 6 ชุด |
| `routes/routes.go` | เพิ่ม repos/service/handler init + routes ใหม่ |

---

## API Endpoints

### Master: Doc Items (Auth + Admin)
```
GET    /api/v1/master/doc-items              ?all=true&loan_type_id=2
GET    /api/v1/master/doc-items/:id
POST   /api/v1/master/doc-items
PUT    /api/v1/master/doc-items/:id
DELETE /api/v1/master/doc-items/:id
```

### Mortgage: Doc Checks (Auth + Officer/Admin)
```
GET    /api/v1/mortgages/:id/doc-checks/           ← ดึงเช็คลิสต์ (auto-create ถ้ายังไม่มี)
PUT    /api/v1/mortgages/:id/doc-checks/           ← อัพเดทเช็คลิสต์ (batch)
GET    /api/v1/mortgages/:id/doc-checks/incomplete ← ดึงรายการไม่ครบ (Frontend ใช้แสดง toast)
```

---

## API Request/Response

### GET /mortgages/5/doc-checks/
```json
{
  "status": "success",
  "data": {
    "doc_checks": [
      {
        "id": 1, "mortgage_id": 5, "doc_item_id": 10,
        "doc_item_code": "PT-01",
        "doc_item_name": "แบบฟอร์มคำขอกู้พิเศษ",
        "is_required": true, "is_checked": true, "is_recommended": false,
        "sort_order": 1, "checked_by": 3, "checked_at": "2026-02-25T10:00:00Z"
      }
    ]
  }
}
```

### PUT /mortgages/5/doc-checks/
```json
{
  "items": [
    { "id": 1, "is_checked": true, "is_recommended": false },
    { "id": 2, "is_checked": false, "is_recommended": true }
  ]
}
```

### GET /mortgages/5/doc-checks/incomplete
```json
{
  "status": "success",
  "data": {
    "result": {
      "mortgage_id": 5,
      "total_items": 13,
      "checked_items": 8,
      "missing_items": [
        {
          "id": 5, "doc_item_code": "PT-05",
          "doc_item_name": "สน.ทะเบียนสมรส / สน.ใบหย่า / สน.ใบมรณะบัตร 2 ใบ",
          "is_required": true, "is_checked": false, "is_recommended": true
        }
      ]
    }
  }
}
```
→ Frontend ใช้ `missing_items` แสดง toast แจ้งเตือน

---

## Seeder — สิ่งที่จะเกิดตอน compose up

1. **updateLoanTypeCodes():**
   - `ORDINARY` → `สบ`
   - `MULTIPURPOSE` → `อป`
   - เพิ่ม `FirstHome` (กู้พิเศษเพื่อการเคหะบ้านหลังแรก)

2. **seedDocItems():** — 6 ชุด
   - พท (13 รายการ), พด (11), พค (13), FirstHome (13), สบ (7), อป (7)
   - Pattern: เช็คก่อนสร้าง → **ไม่ overwrite, ไม่สร้างซ้ำ, ปลอดภัยตอน restart**

---

## วิธี Deploy

1. วางไฟล์ทับตาม path ใน `/var/www/loaneasy/internal/`
2. `docker-compose down && docker-compose up -d --build`
3. AutoMigrate สร้าง `doc_items` + `mortgage_doc_checks` อัตโนมัติ
4. Seeder seed ข้อมูลอัตโนมัติ

**⚠️ ห้ามใช้ `docker-compose down -v`** (ลบ volume = ลบ DB!)
