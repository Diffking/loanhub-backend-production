# 🔐 SMS OTP Upgrade Guide - EasyLoan v3

## สรุปการเปลี่ยนแปลง

### ❌ ระบบเดิม (v2)
```
OTP ส่งผ่าน LINE Push Message
OTP ถูกส่งกลับใน API response (ช่องโหว่!)
```

### ✅ ระบบใหม่ (v3)
```
OTP ส่งผ่าน SMS (SMSMKT) → ตรงถึง SIM card
OTP ไม่ถูกส่งกลับใน response (ปลอดภัย)
Dev mode → ส่ง OTP กลับ + fallback LINE (สะดวกทดสอบ)
Prod mode → SMS เท่านั้น, ไม่ส่ง OTP กลับ
```

---

## ไฟล์ที่เปลี่ยนแปลง

| ไฟล์ | ประเภท | ตำแหน่งในโปรเจค |
|---|---|---|
| `sms_service.go` | 🆕 สร้างใหม่ | `internal/core/services/sms_service.go` |
| `liff_handler.go` | ✏️ แก้ไข | `internal/adapters/http/handlers/liff_handler.go` |
| `routes.go` | ✏️ แก้ไข | `internal/adapters/http/routes/routes.go` |
| `.env` | ✏️ เพิ่มตัวแปร | root directory |

---

## วิธีติดตั้ง (Step by Step)

### Step 1: คัดลอกไฟล์ใหม่

```bash
# 1. สร้างไฟล์ sms_service.go (ไฟล์ใหม่)
cp sms_service.go  internal/core/services/sms_service.go

# 2. แทนที่ liff_handler.go
cp liff_handler.go  internal/adapters/http/handlers/liff_handler.go

# 3. แทนที่ routes.go
cp routes.go  internal/adapters/http/routes/routes.go
```

### Step 2: เพิ่มตัวแปรใน .env

```env
# เพิ่มใน .env ที่มีอยู่
SMS_PROVIDER=smsmkt
SMS_API_KEY=your_api_key_here
SMS_API_SECRET=your_secret_key_here
SMS_SENDER=SPSC
```

### Step 3: ทดสอบ (Dev Mode)

```bash
# ใน dev mode ไม่ต้องมี SMS API Key ก็ได้
# ระบบจะ fallback ส่งผ่าน LINE อัตโนมัติ
# + ส่ง OTP กลับใน response เพื่อทดสอบ

APP_MODE=dev go run cmd/main.go
```

### Step 4: ทดสอบ API

```bash
# Request OTP
curl -X POST http://localhost:3000/api/v1/auth/liff/otp/request \
  -H "Content-Type: application/json" \
  -d '{
    "line_access_token": "xxx",
    "memb_no": "12345",
    "phone": "0891234567"
  }'

# Dev response (มี otp_code):
# {
#   "phone_masked": "089XXXX567",
#   "expires_in": 300,
#   "provider": "line_fallback",
#   "otp_code": "123456",        ← เฉพาะ dev mode
#   "_dev_warning": "..."
# }

# Prod response (ไม่มี otp_code):
# {
#   "phone_masked": "089XXXX567",
#   "expires_in": 300,
#   "provider": "SMSMKT"
# }
```

### Step 5: Production Deployment

```bash
# ตั้งค่า .env บน VPS
APP_MODE=prod
SMS_PROVIDER=smsmkt
SMS_API_KEY=<key จาก SMSMKT>
SMS_API_SECRET=<secret จาก SMSMKT>
SMS_SENDER=SPSC

# Build & Run
go build -o server cmd/main.go
./server
```

---

## สมัคร SMSMKT

1. ไปที่ https://smsmkt.com
2. สมัครสมาชิก
3. Login เข้า Portal
4. ไปที่ **ตั้งค่า > API Key**
5. กด **สร้าง API Key** → จะได้ `API Key` + `Secret Key`
6. จดทะเบียน **Sender Name** (ชื่อผู้ส่ง เช่น `SPSC`)
7. นำ key มาใส่ใน `.env`

**API Docs:** https://developers.smsmkt.com/en/api-reference

---

## Security Flow Diagram

```
┌─────────────────────────────────────────────────────────┐
│  FRONTEND (LIFF App)                                     │
│                                                           │
│  1. ผู้ใช้กรอกเลขสมาชิก + เบอร์โทร                        │
│  2. กดปุ่ม "ขอรหัส OTP"                                   │
│     → POST /auth/liff/otp/request                        │
│  3. รอ SMS เข้าเครื่อง (ไม่มี OTP ใน response)             │
│  4. กรอก OTP 6 หลัก                                       │
│     → POST /auth/liff/otp/verify                         │
│  5. กดลงทะเบียน                                           │
│     → POST /auth/liff/register                           │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│  BACKEND (Go/Fiber)                                      │
│                                                           │
│  /otp/request:                                           │
│  ├── Verify LINE Token ✓                                 │
│  ├── ตรวจเลขสมาชิกใน flommast ✓                          │
│  ├── ตรวจเบอร์โทรตรงกับ DB ✓                              │
│  ├── สร้าง OTP 6 หลัก (เก็บใน memory, TTL 5 นาที) ✓      │
│  └── ส่ง OTP ผ่าน SMS Service:                           │
│      ├── SMS Provider พร้อม? → ส่ง SMSMKT                │
│      └── ไม่พร้อม? → fallback ส่ง LINE                   │
│                                                           │
│  /otp/verify:                                            │
│  ├── ตรวจ OTP ตรงไหม ✓                                   │
│  ├── ตรวจหมดอายุ ✓                                       │
│  └── จำกัด 5 ครั้ง ✓                                     │
└─────────────────────┬───────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────┐
│  SMS GATEWAY (SMSMKT)                                    │
│                                                           │
│  POST https://portal-otp.smsmkt.com/api/send-message     │
│  Headers: api_key, secret_key                            │
│  Body: { "message", "phone": "08xxxxxxxx", "sender" }   │
│                                                           │
│  → SMS ส่งตรงถึง SIM card ของผู้ใช้                       │
│  → พิสูจน์ว่าเครื่องนี้มี SIM ที่รับ SMS ได้จริง           │
└─────────────────────────────────────────────────────────┘
```

---

## จุดเปลี่ยนแปลงสำคัญในโค้ด

### 1. `liff_handler.go` - NewLIFFHandler (เพิ่ม parameter)

```go
// เดิม (v2):
func NewLIFFHandler(db *gorm.DB, lineService *services.LINEService, otpService *services.OTPService) *LIFFHandler

// ใหม่ (v3):
func NewLIFFHandler(db *gorm.DB, lineService *services.LINEService, otpService *services.OTPService, smsService *services.SMSService) *LIFFHandler
```

### 2. `liff_handler.go` - RequestOTP (เปลี่ยนวิธีส่ง)

```go
// เดิม: ส่งผ่าน LINE + ส่ง OTP กลับใน response
go func() {
    h.lineService.SendPushMessage(profile.UserID, smsMessage, channelAccessToken)
}()
return response.Success(c, "...", fiber.Map{
    "otp_code": otpCode,  // ❌ ช่องโหว่!
})

// ใหม่: ส่งผ่าน SMS Service (SMSMKT) + ไม่ส่ง OTP กลับ
go func() {
    h.smsService.SendOTP(cleanPhone, otpCode, profile.UserID)
}()
return response.Success(c, "...", fiber.Map{
    // ✅ ไม่มี otp_code ใน prod mode
})
```

### 3. `routes.go` - Wire SMS Service

```go
// เดิม:
liffHandler := handlers.NewLIFFHandler(db, lineService, otpService)

// ใหม่:
smsService := services.NewSMSService(lineService)
liffHandler := handlers.NewLIFFHandler(db, lineService, otpService, smsService)
```
