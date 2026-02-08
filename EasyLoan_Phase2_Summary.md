# EasyLoan Queue — Phase 2 Summary

> **วันที่:** 8 กุมภาพันธ์ 2026
> **Phase:** 2 — Backend API Walk-in (กดคิว/เรียก/สถานะ)
> **สถานะ:** 📝 โค้ดพร้อม — รอ deploy

---

## 1. ไฟล์ใหม่ 4 ไฟล์

| ไฟล์ | ตำแหน่ง | หมายเหตุ |
|---|---|---|
| `queue_repository.go` | `internal/adapters/persistence/repositories/` | DB queries ทั้งหมดของระบบคิว |
| `queue_service.go` | `internal/core/services/` | Business logic: Walk-in, Call, Serve, Skip, Transfer |
| `queue_handler.go` | `internal/adapters/http/handlers/` | USER endpoints: กดคิว/ดูสถานะ |
| `queue_admin_handler.go` | `internal/adapters/http/handlers/` | OFFICER/ADMIN: เรียกคิว/จัดการช่อง |

## 2. ไฟล์แก้ไข 1 ไฟล์

| ไฟล์ | สิ่งที่แก้ |
|---|---|
| `routes.go` | เพิ่ม `queueRepo`, `queueService`, `queueHandler`, `queueAdminHandler` + 2 route groups |

---

## 3. API Endpoints ที่เพิ่ม

### USER (ต้อง JWT) — `/api/v1/queue/*`

| Method | Endpoint | หมายเหตุ |
|---|---|---|
| `GET` | `/queue/branches` | ดูจุดบริการทั้งหมด |
| `GET` | `/queue/branches/:id/services` | ดูบริการ + ช่องที่เปิด |
| `GET` | `/queue/branches/:id/status` | สถานะคิวปัจจุบัน (จำนวน waiting/serving/done) |
| `POST` | `/queue/walkin` | กดคิว Walk-in `{ branch_id, service_type_id }` |
| `GET` | `/queue/my-tickets` | คิวของฉันวันนี้ |
| `GET` | `/queue/my-tickets/:id` | รายละเอียดคิว |
| `GET` | `/queue/track/:ticket_number` | ติดตามจากเลขคิว (เช่น Q-001) |

### OFFICER/ADMIN (ต้อง JWT + role) — `/api/v1/admin/queue/*`

| Method | Endpoint | Body | หมายเหตุ |
|---|---|---|---|
| `POST` | `/admin/queue/counter/open` | `{ counter_id }` | เปิดช่อง (assign ตัวเองเป็น staff) |
| `POST` | `/admin/queue/counter/close` | `{ counter_id }` | ปิดช่อง |
| `POST` | `/admin/queue/counter/break` | `{ counter_id }` | พักช่อง |
| `POST` | `/admin/queue/call-next` | `{ counter_id }` | เรียกคิวถัดไป (auto-select ตาม priority) |
| `POST` | `/admin/queue/call/:id` | `{ counter_id }` | เรียกคิวเจาะจง |
| `POST` | `/admin/queue/recall/:id` | — | เรียกซ้ำ (update called_at) |
| `POST` | `/admin/queue/serve/:id` | — | เริ่มให้บริการ |
| `POST` | `/admin/queue/complete/:id` | — | เสร็จสิ้น |
| `POST` | `/admin/queue/skip/:id` | — | ข้ามคิว (ข้าม 3 ครั้ง = auto cancel) |
| `POST` | `/admin/queue/transfer/:id` | `{ new_counter_id }` | โอนคิวไปช่องอื่น |
| `GET` | `/admin/queue/dashboard?branch_id=X` | — | สรุปคิว + waiting list + counters |
| `GET` | `/admin/queue/history?branch_id=X` | — | ประวัติคิววันนี้ |
| `GET` | `/admin/queue/config?branch_id=X` | — | ดูค่าตั้ง |
| `PUT` | `/admin/queue/config` | `{ branch_id, key, value }` | แก้ค่าตั้ง |

---

## 4. Business Logic สำคัญ

### กดคิว Walk-in (CreateWalkin)
1. ตรวจ branch active
2. ตรวจ service_type มีอยู่
3. ตรวจ user ไม่มีคิวซ้ำ (same branch + service + วันนี้ + status WAITING/CALLING/SERVING)
4. สร้างเลขคิว `Q-001`, `Q-002`, ...
5. Return ticket + จำนวนคิวรอ + เวลาประมาณ

### เรียกคิว (CallNext)
1. ตรวจ counter เปิดอยู่
2. ตรวจ counter ไม่มีคิว active อยู่
3. หาคิวถัดไป: `priority DESC, issued_at ASC` (ผู้สูงอายุก่อน → FIFO)
4. อัพเดทสถานะ → CALLING + assign counter + called_at

### ข้ามคิว (Skip)
- ข้าม 1-2 ครั้ง → กลับไปรอ (WAITING) + skip_count+1
- ข้าม 3 ครั้ง → auto cancel (CANCELLED)

### โอนคิว (Transfer)
- ย้าย ticket ไปช่องบริการอื่น
- อัพเดท service_type_id ตาม counter ใหม่
- reset สถานะ → WAITING

---

## 5. วิธี Deploy

### สร้างไฟล์ใหม่บน VPS

```bash
ssh root@72.62.67.47
cd /var/www/loaneasy

# 1. สร้าง queue_repository.go
cat > internal/adapters/persistence/repositories/queue_repository.go << 'GOEOF'
# (วางโค้ดจากไฟล์ queue_repository.go)
GOEOF

# 2. สร้าง queue_service.go
cat > internal/core/services/queue_service.go << 'GOEOF'
# (วางโค้ดจากไฟล์ queue_service.go)
GOEOF

# 3. สร้าง queue_handler.go
cat > internal/adapters/http/handlers/queue_handler.go << 'GOEOF'
# (วางโค้ดจากไฟล์ queue_handler.go)
GOEOF

# 4. สร้าง queue_admin_handler.go
cat > internal/adapters/http/handlers/queue_admin_handler.go << 'GOEOF'
# (วางโค้ดจากไฟล์ queue_admin_handler.go)
GOEOF

# 5. แทนที่ routes.go (backup ก่อน)
cp internal/adapters/http/routes/routes.go internal/adapters/http/routes/routes.go.bak
cat > internal/adapters/http/routes/routes.go << 'GOEOF'
# (วางโค้ดจากไฟล์ routes.go ฉบับเต็ม)
GOEOF
```

### Build & Test

```bash
# Build ทดสอบก่อน
docker-compose down
docker-compose up -d --build
docker-compose logs -f --tail=50

# ทดสอบ endpoint
curl -s http://localhost:3000/health
curl -s http://localhost:3000/api/v1/queue/branches -H "Authorization: Bearer <TOKEN>"
```

### ถ้าพัง → ย้อนกลับ

```bash
# คืน routes.go เดิม
cp internal/adapters/http/routes/routes.go.bak internal/adapters/http/routes/routes.go
# ลบไฟล์ใหม่
rm internal/adapters/persistence/repositories/queue_repository.go
rm internal/core/services/queue_service.go
rm internal/adapters/http/handlers/queue_handler.go
rm internal/adapters/http/handlers/queue_admin_handler.go
# Rebuild
docker-compose down && docker-compose up -d --build
```

### Git Commit

```bash
git add .
git commit -m "feat: Phase 2 - Queue Walk-in API (handler/service/repository/routes)"
git push origin feature/queue-phase1
```

---

## 6. Phase 3 จะทำอะไร

- **Frontend** หน้ากดคิว Walk-in (Queue.jsx)
- **Frontend** หน้าติดตามคิว real-time (QueueStatus.jsx)
- **Frontend** หน้า OFFICER เรียกคิว (admin/QueueDashboard.jsx)
- เพิ่ม route ใน React App.jsx
- เพิ่ม menu ใน Layout.jsx
