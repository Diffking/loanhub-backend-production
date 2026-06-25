# Phase 6: Document Checklist Backend — Handoff

## สรุปการเปลี่ยนแปลง

### ไฟล์ใหม่ (4 ไฟล์)
| ไฟล์ | หน้าที่ |
|---|---|
| `handlers/doc_check_handler.go` | Handler: CRUD doc_items + checklist + toast + LINE |
| `repositories/doc_item_repository.go` | Repository: doc_items CRUD |
| `repositories/mortgage_doc_check_repository.go` | Repository: mortgage_doc_checks + InitChecklist + GetIncomplete |
| `services/doc_check_service.go` | Service: business logic + Toast data + LINE push |

### ไฟล์แก้ไข (3 ไฟล์)
| ไฟล์ | สิ่งที่เปลี่ยน |
|---|---|
| `models/models.go` | เพิ่ม `DocItem`, `MortgageDocCheck`, `MortgageDocCheckResponse` + AutoMigrate |
| `config/master_seeder.go` | เพิ่ม `updateLoanTypeCodes()` + `seedDocItems()` + `fixDocItemsRequired()` |
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
GET    /api/v1/mortgages/:id/doc-checks/              ← ดึงเช็คลิสต์ (auto-create)
PUT    /api/v1/mortgages/:id/doc-checks/              ← อัพเดทเช็คลิสต์ (batch)
GET    /api/v1/mortgages/:id/doc-checks/incomplete    ← ดึงรายการไม่ครบ (สำหรับ Toast)
POST   /api/v1/mortgages/:id/doc-checks/notify-line   ← ส่ง LINE แจ้งสมาชิก
```

---

## การทำงานของแต่ละ Endpoint

### `GET .../incomplete` → สำหรับ Toast (แสดงในหน้าเว็บ)
- Return ข้อมูลเอกสารไม่ครบ พร้อม total/checked/missing
- Frontend เอาไปแสดง toast เช่น "เอกสารครบ 8/13 — ยังขาด 5 รายการ"
- **ไม่ส่งอะไรออกไปข้างนอก**

### `POST .../notify-line` → ส่ง LINE ถึงสมาชิกจริง
- ดึงรายการไม่ครบ → สร้างข้อความ → ส่ง LINE push message
- ต้องมี `LINE_CHANNEL_ACCESS_TOKEN` ใน ENV
- สมาชิกต้องเชื่อม LINE แล้ว (มี line_user_id)

Error cases:
- `"สมาชิกยังไม่ได้เชื่อมต่อ LINE"` — ไม่มี line_user_id
- `"เอกสารครบถ้วนแล้ว ไม่มีรายการที่ต้องแจ้งเตือน"` — check หมดแล้ว

---

## วิธี Deploy

```bash
# Windows → Git
cd D:\File_Claude\SPSC-loaneasy\backend-vps\loaneasy
git add .
git commit -m "feat: Phase 6 doc checklist + fix is_required + LINE notify"
git push origin backend/Easyloan-Version1

# VPS
cd /var/www/loaneasy
git pull origin backend/Easyloan-Version1
docker-compose down && docker-compose up -d --build
```

**⚠️ ห้ามใช้ `docker-compose down -v`**
