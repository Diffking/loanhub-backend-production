# 📋 EasyLoan (SPSC LoanEasy) — Project Summary

## 🖥️ Server & Infrastructure

| รายการ | รายละเอียด |
|---|---|
| VPS | Hostinger — srv1178613.hstgr.cloud (72.62.67.47) |
| OS | Ubuntu |
| Backend Path | `/var/www/loaneasy` |
| Branch ปัจจุบัน | `feature/queue-phase456` |
| GitHub | `github.com/Diffking/loanhub-backend-production` |
| Docker | `loaneasy-api` (Go app) — MySQL รันบน host ไม่ใช่ Docker |
| Domain API | `https://api.loanspsc.com` |
| Domain Frontend User | `https://user.loanspsc.com` |
| Domain Frontend Admin | (ระบุเพิ่มเมื่อ deploy) |
| DB | MySQL — `spsc_loaneasy` (port 3306 บน host) |

## 🏗️ Tech Stack

### Backend (Go)
- **Framework:** Fiber v2.52.0
- **ORM:** GORM
- **Auth:** JWT (Cookie + Bearer Header + Query Param)
- **Docs:** Swagger
- **Container:** Docker (Alpine + tzdata)
- **Cron:** robfig/cron/v3

### Frontend User (React — LIFF)
- **Framework:** React + Vite
- **CSS:** Tailwind CSS
- **Auth:** LIFF SDK (LINE Login) + JWT stored in context
- **State:** React Context (AuthContext)
- **API Client:** Axios
- **Path บน Windows:** `D:\File_Claude\SPSC-loaneasy\backend-vps` (มี frontend อยู่ด้วย)

### Frontend Admin (React)
- **Framework:** React + Vite
- **CSS:** Tailwind CSS
- **Auth:** JWT (username/password login)
- **ใช้สำหรับ:** Officer / Admin จัดการคิว, สินเชื่อ, dashboard

---

## 📁 Backend Structure

```
/var/www/loaneasy/
├── cmd/                           # Main entry point
├── internal/
│   ├── adapters/
│   │   ├── http/
│   │   │   ├── handlers/          # HTTP Handlers
│   │   │   │   ├── queue_handler.go        # User queue endpoints
│   │   │   │   ├── queue_admin_handler.go  # Officer/Admin queue
│   │   │   │   ├── queue_display_handler.go# TV display (public)
│   │   │   │   ├── auth_handler.go
│   │   │   │   ├── liff_handler.go         # LIFF login/OTP
│   │   │   │   ├── line_handler.go         # LINE OAuth
│   │   │   │   ├── mortgage_handler.go
│   │   │   │   ├── dashboard_handler.go
│   │   │   │   ├── master_handler.go
│   │   │   │   ├── mobile_handler.go       # API v2 mobile
│   │   │   │   └── health_handler.go
│   │   │   ├── middleware/
│   │   │   │   └── auth_middleware.go       # JWT auth (cookie/header/query)
│   │   │   └── routes/
│   │   │       └── routes.go               # All route definitions
│   │   └── persistence/
│   │       ├── models/                     # GORM models
│   │       └── repositories/
│   │           ├── queue_repository.go     # Queue DB queries
│   │           └── ...other repos
│   ├── core/services/
│   │   ├── queue_service.go               # Queue business logic
│   │   ├── queue_notify_service.go        # SSE hub + notifications
│   │   ├── queue_auto_service.go          # Auto-cancel + reminders
│   │   ├── auth_service.go
│   │   ├── notification_service.go        # LINE Notify
│   │   ├── otp_service.go                 # OTP for LIFF
│   │   └── ...other services
│   ├── config/
│   │   └── master_seeder.go              # Seed branches/services/counters
│   └── pkg/
│       ├── jwt/                           # JWT utilities
│       └── response/                      # Standard API response
├── docker-compose.yml
├── Dockerfile
├── go.mod / go.sum
└── docs/                                  # Swagger docs
```

## 📁 Frontend User Structure

```
src/
├── api/
│   └── axios.js              # API client + interceptors
├── context/
│   └── AuthContext.jsx        # Auth state (LIFF + JWT)
├── pages/
│   ├── Login.jsx              # LIFF login page
│   ├── Register.jsx           # Member registration
│   ├── Queue.jsx              # กดคิว Walk-in + จองล่วงหน้า
│   ├── QueueStatus.jsx        # สถานะคิว + ประวัติวันนี้
│   ├── MyLoans.jsx            # ดูสินเชื่อของฉัน
│   └── Profile.jsx            # โปรไฟล์สมาชิก
├── index.html
├── vite.config.js
├── tailwind.config.js
└── package.json
```

---

## 🗄️ Database — ตาราง Queue ที่เกี่ยวข้อง

### branches
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| code | string | HQ, MOB01, BR01 |
| name | string | ชื่อสาขา |
| is_active | bool | เปิด/ปิดสาขา |
| open_time | string | 08:30 |
| close_time | string | 16:30 |

**Data:** 3 สาขา — สำนักงาน HQ, รถตู้โมบาย 1, สาขา รพ.สงขลา

### service_types
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| code | string | LOAN, DEPOSIT, GENERAL |
| name | string | สินเชื่อ, การเงิน, สมาชิกสัมพันธ์ |
| is_active | bool | |
| display_order | int | ลำดับแสดง |

**Data:** 3 บริการ — สินเชื่อ, การเงิน, สมาชิกสัมพันธ์

### service_counters
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| branch_id | uint FK | |
| service_type_id | uint FK | |
| counter_name | string | ช่อง 1, ช่อง 2 |
| counter_number | int | |
| status | string | OPEN/CLOSED/BREAK |
| staff_user_id | *uint FK | พนักงานที่เปิดช่อง |
| is_active | bool | |

**Logic:** HQ มี 3 ช่อง (3 services), MOB01 มี 2 ช่อง, BR01 มี 2 ช่อง

### queue_tickets
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| ticket_number | string | Q-001, B-001 |
| queue_type | string | WALKIN / BOOKING |
| queue_date | date | วันที่คิว |
| branch_id | uint FK | |
| service_type_id | uint FK | |
| user_id | uint FK | |
| status | string | WAITING/CALLING/SERVING/COMPLETED/SKIPPED/CANCELLED |
| priority | int | 0=walk-in, 10=booking checked-in |
| counter_id | *uint FK | ช่องที่เรียก |
| issued_at | timestamp | เวลากดคิว |
| called_at | *timestamp | เวลาเรียก |
| serving_at | *timestamp | เวลาเริ่มบริการ |
| completed_at | *timestamp | เวลาเสร็จ |
| skip_count | int | จำนวนครั้งที่ข้าม (3=auto cancel) |
| booking_date | *date | วันที่จอง |
| booking_slot | string | เวลาจอง HH:MM |
| booking_note | string | หมายเหตุ |

### booking_slots
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| branch_id | uint FK | |
| service_type_id | uint FK | |
| slot_date | date | |
| slot_time | string | HH:MM |
| max_bookings | int | จำนวนจองได้สูงสุด |
| current_bookings | int | จำนวนที่จองแล้ว |
| is_available | bool | |

### queue_configs
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| branch_id | uint FK | |
| config_key | string | avg_service_min, max_booking_per_slot |
| config_value | string | |

### users
| Column | Type | Description |
|---|---|---|
| id | uint PK | |
| username | string | M07337 (เลขสมาชิก) |
| role | string | MEMBER / OFFICER / ADMIN |
| line_user_id | *string | LINE UID |
| liff_device_id | *string | Device fingerprint |

---

## 🔌 API Endpoints (183 handlers)

### Queue — User (Auth required)
| Method | Path | Description |
|---|---|---|
| GET | /api/v1/queue/branches | ดูสาขาทั้งหมด |
| GET | /api/v1/queue/branches/:id/services | ดูบริการ+ช่องของสาขา |
| GET | /api/v1/queue/branches/:id/status | สถานะคิวปัจจุบัน |
| POST | /api/v1/queue/walkin | กดคิว Walk-in |
| POST | /api/v1/queue/ticket | กดคิว Walk-in (alias) |
| GET | /api/v1/queue/my-tickets | คิวของฉันวันนี้ |
| GET | /api/v1/queue/my-tickets/:id | รายละเอียดคิว |
| DELETE | /api/v1/queue/ticket/:id | ยกเลิกคิว Walk-in |
| GET | /api/v1/queue/track/:ticket_number | ติดตามจากเลขคิว |
| GET | /api/v1/queue/events?branch_id=X&token=X | SSE real-time |
| GET | /api/v1/queue/booking/slots | ดู slot จอง |
| POST | /api/v1/queue/booking | จองล่วงหน้า |
| DELETE | /api/v1/queue/booking/:id | ยกเลิกจอง |

### Queue — Admin (Officer/Admin)
| Method | Path | Description |
|---|---|---|
| POST | /api/v1/admin/queue/counter/open | เปิดช่อง |
| POST | /api/v1/admin/queue/counter/close | ปิดช่อง |
| POST | /api/v1/admin/queue/counter/break | พักช่อง |
| POST | /api/v1/admin/queue/call-next | เรียกคิวถัดไป |
| POST | /api/v1/admin/queue/call/:id | เรียกคิวเฉพาะ |
| POST | /api/v1/admin/queue/recall/:id | เรียกซ้ำ |
| POST | /api/v1/admin/queue/serve/:id | เริ่มบริการ |
| POST | /api/v1/admin/queue/complete/:id | เสร็จสิ้น |
| POST | /api/v1/admin/queue/skip/:id | ข้ามคิว |
| POST | /api/v1/admin/queue/transfer/:id | โอนคิว |
| GET | /api/v1/admin/queue/dashboard | Dashboard คิว |
| GET | /api/v1/admin/queue/history | ประวัติวันนี้ |
| GET | /api/v1/admin/queue/bookings | รายการจอง |
| POST | /api/v1/admin/queue/booking/:id/checkin | Check-in จอง |
| POST | /api/v1/admin/queue/slots/generate | สร้าง slot |

### Queue — Display (Public, no auth)
| Method | Path | Description |
|---|---|---|
| GET | /api/v1/queue/display/:branch_id | ข้อมูลจอ TV |
| GET | /api/v1/queue/display/:branch_id/events | SSE จอ TV |

### Auth & LIFF
| Method | Path | Description |
|---|---|---|
| POST | /api/v1/auth/login | Login (username/password) |
| POST | /api/v1/auth/register | Register |
| POST | /api/v1/auth/refresh | Refresh token |
| POST | /api/v1/auth/liff/check | เช็ค LINE user |
| POST | /api/v1/auth/liff/otp/request | ขอ OTP |
| POST | /api/v1/auth/liff/otp/verify | ยืนยัน OTP |
| POST | /api/v1/auth/liff/register | ลงทะเบียน LIFF |
| POST | /api/v1/auth/liff/login | Login ด้วย LIFF |

---

## ✅ แก้ไขล่าสุด (9 ก.พ. 2026)

### Bug Fixes ที่ deploy แล้ว:
1. ✅ **Services กรองตาม branch** — GetServiceTypesByBranch จาก service_counters
2. ✅ **ยกเลิกคิว Walk-in** — เพิ่ม DELETE /queue/ticket/:id + CancelWalkin
3. ✅ **Timezone Bangkok** — bangkokToday() แทน time.Now().Truncate(UTC)
4. ✅ **SSE auth query param** — auth_middleware อ่าน ?token= ได้

### ไฟล์ Backend ที่แก้:
- `internal/adapters/http/middleware/auth_middleware.go`
- `internal/adapters/persistence/repositories/queue_repository.go`
- `internal/core/services/queue_service.go`
- `internal/adapters/http/handlers/queue_handler.go`
- `internal/adapters/http/routes/routes.go`

### ค้างไว้:
- LINE แจ้งเตือนคิว (ใกล้ถึงคิว + เรียกคิว) — ต้องอัพโหลด queue_notify_service.go + notification_service.go
- ฟีเจอร์ OTP ใหม่

---

## 🔐 Test Credentials

| User | Password | Role |
|---|---|---|
| M07337 | Test1234 | MEMBER |

## 📦 Deploy Commands

```bash
# Windows → push
cd D:\File_Claude\SPSC-loaneasy\backend-vps
git add .
git commit -m "message"
git push origin feature/queue-phase456

# VPS → pull & rebuild
ssh root@72.62.67.47
cd /var/www/loaneasy
git stash && git pull origin feature/queue-phase456
docker-compose down && docker-compose up -d --build
docker-compose logs -f --tail=50

# DB Backup (MySQL on host, not Docker)
mysqldump -u root -p spsc_loaneasy > /var/www/backups/db_prod_$(date +%Y%m%d_%H%M%S).sql
```
