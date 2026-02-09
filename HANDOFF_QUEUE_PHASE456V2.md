# 🚀 EasyLoan - Handoff สำหรับแชทใหม่

## โปรเจค
- **ระบบ:** SPSC LoanEasy - ระบบสินเชื่อสหกรณ์
- **Backend:** Go / Fiber v2.52.0
- **Database:** MySQL (spsc_loaneasy)
- **Deploy:** Docker Compose on VPS

## Paths
| ที่ | Path |
|-----|------|
| VPS | `/var/www/loaneasy` |
| Windows | `C:\Users\rahan\Downloads\loaneasy` |
| GitHub | `Diffking/loanhub-backend-production` |
| Branch | `feature/queue-phase456v2` |

## สถานะปัจจุบัน
- ✅ Server รันปกติบน Docker (port 3000)
- ✅ LIFF Login/Register/OTP ทำงานปกติ
- ✅ OTP ส่งผ่าน **LINE** (ฟรี) + แสดง OTP บนหน้าเว็บ (ไม่ต้องสลับไปดู LINE message)
- ✅ SMS OTP via SMSMKT **โค้ดพร้อม** แต่ปิดไว้ (มีค่าใช้จ่าย) — เปิดได้ทันทีโดยเพิ่ม config ใน docker-compose.yml
- ✅ Fallback อัตโนมัติ: SMS ล้มเหลว → ส่ง LINE แทน

## โครงสร้างไฟล์สำคัญ
```
internal/
├── adapters/http/
│   ├── handlers/
│   │   └── liff_handler.go      # LIFF Login/Register/OTP/Check
│   └── routes/
│       └── routes.go            # API routing
├── core/services/
│   ├── sms_service.go           # SMS OTP (SMSMKT provider)
│   ├── otp_service.go           # OTP generate/verify (in-memory)
│   └── line_service.go          # LINE API service
└── internal/config/
    └── config.go                # App configuration
```

## API Endpoints (LIFF)
| Method | Path | Description |
|--------|------|-------------|
| POST | /api/v1/auth/liff/check | เช็ค LINE user + สถานะ |
| POST | /api/v1/auth/liff/otp/request | ขอ OTP (ส่ง LINE/SMS) |
| POST | /api/v1/auth/liff/otp/verify | ยืนยัน OTP |
| POST | /api/v1/auth/liff/register | ลงทะเบียน (ต้อง cellular) |
| POST | /api/v1/auth/liff/login | เข้าสู่ระบบ |

## OTP Flow
```
ขอ OTP → GenerateOTP (6 หลัก, 5 นาที, max 5 attempts)
       → ส่ง LINE (ปัจจุบัน) / SMS (เมื่อเปิด)
       → แสดง OTP บนหน้าเว็บ (เมื่อใช้ LINE fallback)
       → User กรอก OTP → VerifyOTP → Register
```

## Register Flow
1. Validate fields (line_access_token, memb_no, device_id, otp_code)
2. ตรวจ Network Type → **บังคับ Cellular** (WiFi ไม่ผ่าน)
3. Verify LINE Token → ดึง profile
4. Verify OTP
5. ตรวจ LINE ซ้ำ + Device ซ้ำ
6. เช็ค flommast (ข้อมูลสมาชิก)
7. สร้าง user ใหม่ หรือ ผูก LINE กับบัญชีที่มีอยู่

## Docker Compose
- Config ทั้งหมดอยู่ใน `docker-compose.yml` ส่วน `environment:`
- **ไม่ได้**อ่านจาก `.env` file
- Network mode: host (port 3000)
- DB: 127.0.0.1:3306/spsc_loaneasy

## Deploy Commands
```bash
cd /var/www/loaneasy
git pull origin feature/queue-phase456v2
docker-compose down
docker-compose up -d --build
docker-compose logs -f
```

## เปิด SMS OTP (เมื่อพร้อม)
ดูคู่มือ: `SMS_OTP_SWITCH_GUIDE.md`
- ซื้อ credit SMSMKT
- สมัคร Sender Name "SPSC"
- เพิ่ม SMS config ใน docker-compose.yml
- โทร AIS 1175 ปลดบล็อก SMS

## ⚠️ สิ่งที่ต้องระวัง
- `docker-compose.yml` มี secret keys → **ห้าม commit ขึ้น git**
- `.gitignore` มี `server` แล้ว (ป้องกัน binary)
- OTP เก็บใน memory → restart server = OTP หาย
- Register บังคับ Cellular → WiFi จะ 400 error
- Debug log "🔍[REGISTER]" ยังเปิดอยู่ → ลบทิ้งเมื่อ stable
