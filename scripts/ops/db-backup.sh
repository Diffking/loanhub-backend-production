#!/usr/bin/env bash
# ============================================================
# สำรองฐานข้อมูล production → บีบอัด → เข้ารหัส → ตรวจไฟล์ → ลบของเก่า
# ผิดพลาดเมื่อไหร่แจ้ง LINE ทันที (ใช้ ALERT_LINE_TO เดียวกับ uptime-check.sh)
#
# ต้องมีใน .env:
#   PROD_DB_*               (มีอยู่แล้ว)
#   BACKUP_PASSPHRASE=...   รหัสถอดไฟล์ backup — ถ้าหายไฟล์ backup จะกู้ไม่ได้!
#   LINE_CHANNEL_ACCESS_TOKEN, ALERT_LINE_TO
#
# ใช้งาน (root บน VPS):
#   bash /var/www/loaneasy/scripts/ops/db-backup.sh          สำรอง 1 ครั้ง
#   bash /var/www/loaneasy/scripts/ops/db-backup.sh --test   สำรอง + แจ้ง LINE แม้สำเร็จ
#
# กู้คืน (ระวัง: ทับข้อมูลปัจจุบัน):
#   openssl enc -d -aes-256-cbc -pbkdf2 -in <ไฟล์.sql.gz.enc> -pass env:BACKUP_PASSPHRASE \
#     | gunzip | mysql -h<host> -u<user> -p <dbname>
#
# หมายเหตุ: ไฟล์เก็บบน VPS เครื่องเดียวกับฐานข้อมูล — ควรดาวน์โหลดเก็บไว้นอกเครื่องเป็นระยะ
#   (จากเครื่องตัวเอง: scp -i ~/.ssh/loaneasy_vps root@<vps-ip>:/root/db-backups/<ไฟล์> .)
# ============================================================
set -uo pipefail

ENV_FILE="${ENV_FILE:-/var/www/loaneasy/.env}"
BACKUP_DIR="${BACKUP_DIR:-/root/db-backups}"
KEEP="${KEEP:-14}"

# shellcheck source=/dev/null
. "$(dirname "$0")/../lib/line-notify.sh"

g() { line_env_get "$1"; }

fail() {
  local msg="$1"
  echo "ERROR: $msg" >&2
  line_notify "🔴 [loanEasy] สำรองฐานข้อมูลล้มเหลว ($(TZ=Asia/Bangkok date '+%Y-%m-%d %H:%M'))
$msg"
  exit 1
}

DB_NAME="$(g PROD_DB_NAME)"
PASSPHRASE="$(g BACKUP_PASSPHRASE)"
[ -n "$DB_NAME" ] || fail "อ่าน PROD_DB_NAME จาก $ENV_FILE ไม่ได้"
[ -n "$PASSPHRASE" ] || fail "ยังไม่ได้ตั้ง BACKUP_PASSPHRASE ใน $ENV_FILE"

mkdir -p "$BACKUP_DIR" && chmod 700 "$BACKUP_DIR"
STAMP="$(TZ=Asia/Bangkok date +%Y%m%d-%H%M%S)"
OUT="$BACKUP_DIR/$DB_NAME-$STAMP.sql.gz.enc"

export MYSQL_PWD="$(g PROD_DB_PASS)"
export BACKUP_PASSPHRASE="$PASSPHRASE"

# dump → gzip → encrypt (ไม่เขียน plaintext ลงดิสก์เลย)
if ! timeout 900 mysqldump --single-transaction --no-tablespaces --quick \
      -h"$(g PROD_DB_HOST)" -P"$(g PROD_DB_PORT)" -u"$(g PROD_DB_USER)" "$DB_NAME" 2>/tmp/dumperr \
      | gzip -9 \
      | openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass env:BACKUP_PASSPHRASE -out "$OUT"; then
  rm -f "$OUT"
  fail "mysqldump/encrypt ล้มเหลว: $(tail -3 /tmp/dumperr 2>/dev/null)"
fi
chmod 600 "$OUT"

SIZE=$(stat -c %s "$OUT" 2>/dev/null || echo 0)
[ "$SIZE" -gt 10240 ] || fail "ไฟล์ backup เล็กผิดปกติ ($SIZE bytes) — $OUT"

# ตรวจว่าถอดรหัสและคลายซิปได้จริง และมีคำสั่ง SQL ครบท้ายไฟล์
if ! openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -in "$OUT" -pass env:BACKUP_PASSPHRASE 2>/dev/null \
     | gunzip 2>/dev/null | tail -5 | grep -q "Dump completed"; then
  fail "ไฟล์ backup ตรวจสอบไม่ผ่าน (ถอดรหัส/คลายซิปไม่ได้ หรือ dump ไม่สมบูรณ์) — $OUT"
fi

# ลบของเก่า เหลือ $KEEP ชุดล่าสุด
DELETED=0
while IFS= read -r old; do
  rm -f "$old" && DELETED=$((DELETED + 1))
done < <(ls -1t "$BACKUP_DIR"/*.sql.gz.enc 2>/dev/null | tail -n +$((KEEP + 1)))

HUMAN=$(numfmt --to=iec --suffix=B "$SIZE" 2>/dev/null || echo "${SIZE}B")
COUNT=$(ls -1 "$BACKUP_DIR"/*.sql.gz.enc 2>/dev/null | wc -l)
echo "OK: $OUT ($HUMAN) — เก็บไว้ $COUNT ชุด, ลบเก่า $DELETED ชุด"

if [ "${1:-}" = "--test" ]; then
  line_notify "🧪 [loanEasy] ทดสอบสำรองฐานข้อมูล ($(TZ=Asia/Bangkok date '+%Y-%m-%d %H:%M'))
สำเร็จ: $HUMAN — เก็บไว้ $COUNT ชุด (ลบเก่า $DELETED ชุด)
ตรวจแล้วว่าถอดรหัสและคลายซิปได้จริง"
fi
