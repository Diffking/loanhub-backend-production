-- ============================================================
-- Migration: เพิ่ม columns สำหรับ Device Binding + Phone Verification
-- Run this on your MySQL database
-- ============================================================

-- 1. เพิ่ม device_id (ผูกเครื่อง)
ALTER TABLE users ADD COLUMN device_id VARCHAR(255) DEFAULT NULL COMMENT 'Device ID ผูกเครื่อง (Android ID / identifierForVendor)' AFTER line_linked_at;

-- 2. เพิ่ม phone_verified (เบอร์โทรที่ verify แล้ว)
ALTER TABLE users ADD COLUMN phone_verified VARCHAR(20) DEFAULT NULL COMMENT 'เบอร์โทรที่ยืนยัน OTP แล้ว' AFTER device_id;

-- 3. เพิ่ม network_type (ประเภทเครือข่ายล่าสุด)
ALTER TABLE users ADD COLUMN network_type VARCHAR(20) DEFAULT NULL COMMENT 'ประเภทเครือข่ายล่าสุด: cellular/wifi' AFTER phone_verified;

-- 4. เพิ่ม last_login (เวลา login ล่าสุด) ถ้ายังไม่มี
-- ALTER TABLE users ADD COLUMN last_login DATETIME DEFAULT NULL AFTER network_type;

-- 5. Index สำหรับ device_id (เพื่อ lookup เร็ว)
ALTER TABLE users ADD INDEX idx_users_device_id (device_id);

-- 6. ตรวจว่ามี line_picture_url column หรือยัง (LIFF handler ใช้)
-- ถ้ายังไม่มีให้เพิ่ม
-- ALTER TABLE users ADD COLUMN line_picture_url VARCHAR(500) DEFAULT NULL AFTER line_display_name;

-- ============================================================
-- ตาราง OTP Logs (Optional - สำหรับ audit trail)
-- ถ้าต้องการเก็บ log การส่ง OTP
-- ============================================================
CREATE TABLE IF NOT EXISTS otp_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    line_user_id VARCHAR(50) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    otp_hash VARCHAR(255) NOT NULL COMMENT 'hashed OTP (ไม่เก็บ plain text)',
    status ENUM('sent', 'verified', 'expired', 'failed') DEFAULT 'sent',
    attempts INT DEFAULT 0,
    ip_address VARCHAR(50),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    verified_at DATETIME DEFAULT NULL,
    expires_at DATETIME NOT NULL,
    
    INDEX idx_otp_line_user (line_user_id),
    INDEX idx_otp_phone (phone),
    INDEX idx_otp_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='OTP request logs for audit';

-- ============================================================
-- ตรวจข้อมูลหลัง migration
-- ============================================================
-- SELECT COLUMN_NAME, DATA_TYPE, COLUMN_COMMENT 
-- FROM INFORMATION_SCHEMA.COLUMNS 
-- WHERE TABLE_NAME = 'users' 
-- AND COLUMN_NAME IN ('device_id', 'phone_verified', 'network_type');
