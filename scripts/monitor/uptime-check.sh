#!/usr/bin/env bash
# ============================================================
# loanEasy uptime watchdog — แจ้งเตือนทาง LINE
#
# ตรวจทุกครั้งที่รัน (cron ทุก 5 นาที): admin / user / api ต้องตอบ HTTP 200
# ถ้าเว็บบน Hostinger ล่ม จะยิงทดสอบไป origin ตรง ๆ (ข้าม DNS/CDN) เพื่อบอกได้ว่า
# ปัญหาอยู่ที่ CDN/DNS (origin ยังปกติ) หรือที่ Hosting เอง
# — ใช้ได้ทั้งตอนเปิดและปิด Hostinger CDN (CDN ขัดข้องเคยทำเว็บล่ม 2026-09-22)
#
# แจ้งเตือนเฉพาะตอน "สถานะเปลี่ยน" (ปกติ → มีปัญหา, มีปัญหา → กลับมาปกติ)
# จะไม่ส่งซ้ำทุก 5 นาทีระหว่างที่ยังล่มอยู่
#
# ต้องมีใน .env:
#   LINE_CHANNEL_ACCESS_TOKEN=...   (มีอยู่แล้ว)
#   ALERT_LINE_TO=Uxxxxxxxx...      (LINE user ID ผู้รับแจ้งเตือน, คั่นด้วย , ได้หลายคน)
#
# ใช้งาน:
#   scripts/monitor/uptime-check.sh          ตรวจ + แจ้งเตือนถ้าสถานะเปลี่ยน
#   scripts/monitor/uptime-check.sh --test   ส่งข้อความทดสอบ + แสดงผลตรวจ
# ============================================================
set -u

ENV_FILE="${ENV_FILE:-/var/www/loaneasy/.env}"
STATE_DIR="${STATE_DIR:-/var/lib/loaneasy-monitor}"
ORIGIN_IP="${ORIGIN_IP:-145.223.109.15}"

# เว็บที่ฝากไว้กับ Hostinger Hosting (ทดสอบย้อนไป origin ได้)
HOSTINGER_HOSTS="admin.loanspsc.com user.loanspsc.com"
HTTP_URLS="https://admin.loanspsc.com/ https://user.loanspsc.com/ https://api.loanspsc.com/health"

NL="
"

# อ่านค่าเฉพาะ key ที่ต้องใช้ (ไม่ source .env ทั้งไฟล์)
env_get() {
  grep -E "^$1=" "$ENV_FILE" 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '\r' | sed -e 's/^["'\'']//' -e 's/["'\'']$//'
}

TOKEN="$(env_get LINE_CHANNEL_ACCESS_TOKEN)"
ALERT_TO="$(env_get ALERT_LINE_TO)"

send_line() {
  local text="$1" to json
  if [ -z "$TOKEN" ] || [ -z "$ALERT_TO" ]; then
    echo "WARN: LINE_CHANNEL_ACCESS_TOKEN / ALERT_LINE_TO not set in $ENV_FILE — skip LINE" >&2
    return 1
  fi
  # escape สำหรับ JSON: \ " และขึ้นบรรทัดใหม่
  local bs='\' q='"' cr=$'\r'
  text="${text//"$bs"/"$bs$bs"}"
  text="${text//"$q"/"$bs$q"}"
  text="${text//"$cr"/}"
  text="${text//"$NL"/"${bs}n"}"
  for to in $(echo "$ALERT_TO" | tr ',' ' '); do
    json="{\"to\":\"$to\",\"messages\":[{\"type\":\"text\",\"text\":\"$text\"}]}"
    curl -sS -m 15 -o /dev/null -w "LINE push -> %{http_code}\n" \
      -X POST https://api.line.me/v2/bot/message/push \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "$json"
  done
}

# ยิงตรงไปที่ origin (ข้าม DNS/CDN) เพื่อแยกว่าใครพัง
origin_ok() {
  local host="$1"
  [ "$(curl -sS -o /dev/null -w '%{http_code}' -m 15 \
      --resolve "$host:443:$ORIGIN_IP" "https://$host/" 2>/dev/null)" = "200" ]
}

# IP ที่ DNS ตอบตอนนี้ (แนบในข้อความแจ้งเตือนไว้วิเคราะห์)
dns_ips() {
  getent ahostsv4 "$1" | awk '{print $1}' | sort -u | tr '\n' ' '
}

check_http() {
  local url="$1" code
  code="$(curl -sS -o /dev/null -w '%{http_code}' -m 15 "$url" 2>/dev/null)"
  if [ "$code" != "200" ]; then
    # ลองซ้ำอีกครั้งกันสะดุดชั่วคราว
    sleep 10
    code="$(curl -sS -o /dev/null -w '%{http_code}' -m 15 "$url" 2>/dev/null)"
  fi
  if [ "$code" != "200" ]; then
    echo "HTTP $url → ${code:-timeout}"
    return 1
  fi
  return 0
}

problems=""
for u in $HTTP_URLS; do
  if msg="$(check_http "$u")"; then continue; fi
  problems="${problems}- ${msg}${NL}"
  # เว็บ Hostinger ล่ม → เช็ค origin เพื่อบอกว่าเป็นที่ CDN หรือที่ Hosting
  for h in $HOSTINGER_HOSTS; do
    case "$u" in
      *"$h"*)
        if origin_ok "$h"; then
          problems="${problems}  origin ($ORIGIN_IP) ปกติ → ปัญหาที่ CDN/DNS (DNS ตอบ: $(dns_ips "$h"))${NL}"
          problems="${problems}  แก้ชั่วคราว: hPanel → Websites → $h → Performance → CDN → ปิด${NL}"
        else
          problems="${problems}  origin ($ORIGIN_IP) ก็ไม่ตอบ → ปัญหาที่ Hosting ของ Hostinger${NL}"
        fi
        ;;
    esac
  done
done

now="$(TZ=Asia/Bangkok date '+%Y-%m-%d %H:%M')"

if [ "${1:-}" = "--test" ]; then
  if [ -z "$problems" ]; then result="ทุกอย่างปกติ ✅"; else result="พบปัญหา:${NL}${problems}"; fi
  echo "$result"
  send_line "🧪 [loanEasy monitor] ทดสอบแจ้งเตือน ($now)${NL}${result}"
  exit 0
fi

mkdir -p "$STATE_DIR"
state_file="$STATE_DIR/state"
prev="$(cat "$state_file" 2>/dev/null || echo OK)"

if [ -n "$problems" ]; then
  echo "DOWN" > "$state_file"
  echo "$now DOWN${NL}${problems}"
  if [ "$prev" != "DOWN" ]; then
    send_line "🔴 [loanEasy] ระบบมีปัญหา ($now)${NL}${problems}"
  fi
else
  echo "OK" > "$state_file"
  if [ "$prev" = "DOWN" ]; then
    send_line "🟢 [loanEasy] ระบบกลับมาปกติแล้ว ($now)"
  fi
fi
