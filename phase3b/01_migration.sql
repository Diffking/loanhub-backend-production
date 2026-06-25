-- ============================================
-- Phase 3b Migration
-- 1. Add columns to flommast: mast_share_value, member_type_code
-- 2. Create table savings_accounts
-- ============================================
SET NAMES utf8mb4;

-- Add columns to flommast (idempotent — guarded)
ALTER TABLE flommast
  ADD COLUMN IF NOT EXISTS mast_share_value DECIMAL(13,2) NOT NULL DEFAULT 0.00 COMMENT 'มูลค่าทุนเรือนหุ้น';

ALTER TABLE flommast
  ADD COLUMN IF NOT EXISTS member_type_code VARCHAR(10) NOT NULL DEFAULT '' COMMENT 'รหัสประเภทสมาชิก เช่น 1000, 0110';

-- Add index for filtering by type_code
ALTER TABLE flommast
  ADD INDEX IF NOT EXISTS idx_member_type_code (member_type_code);

-- Create savings_accounts table
CREATE TABLE IF NOT EXISTS savings_accounts (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  mast_memb_no VARCHAR(20) NOT NULL,
  full_name VARCHAR(255) NOT NULL DEFAULT '',
  account_no VARCHAR(20) NOT NULL,
  balance DECIMAL(15,4) NOT NULL DEFAULT 0,
  created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uk_member_account (mast_memb_no, account_no),
  KEY idx_memb_no (mast_memb_no)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='บัญชีเงินฝากของสมาชิก ใช้สำหรับค้ำประกันเงินกู้';
