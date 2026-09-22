#!/usr/bin/env bash
# ============================================================
# loanEasy uptime + CDN watchdog — แจ้งเตือนทาง LINE
#
# ตรวจทุกครั้งที่รัน (cron ทุก 5 นาที):
#   1. admin/user.loanspsc.com ต้อง resolve ไป origin (ไม่ผ่าน Hostinger CDN)
#      — เคยล่มเพราะ CDN ถูกเปิดแล้ว CDN ขัดข้อง (2026-09-22)
#   2. admin / user / api ต้องตอบ HTTP 200
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

DNS_HOSTS="admin.loanspsc.com user.loanspsc.com"
HTTP_URLS="https://admin.loanspsc.com/ https://user.loanspsc.com/ https://api.loanspsc.com/health"

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
  local bs='\' q='"' nl=$'\n' cr=$'\r'
  text="${text//"$bs"/"$bs$bs"}"
  text="${text//"$q"/"$bs$q"}"
  text="${text//"$cr"/}"
  text="${text//"$nl"/"${bs}n"}"
  for to in $(echo "$ALERT_TO" | tr ',' ' '); do
    json="{\"to\":\"$to\",\"messages\":[{\"type\":\"text\",\"text\":\"$text\"}]}"
    curl -sS -m 15 -o /dev/null -w "LINE push -> %{http_code}\n" \
      -X POST https://api.line.me/v2/bot/message/push \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "$json"
  done
}

check_dns() {
  local host="$1" ips
  ips="$(getent ahostsv4 "$host" | awk '{print $1}' | sort -u | tr '\n' ' ')"
  if [ -z "$ips" ]; then
    echo "DNS $host: resolve ไม่ได้"
    return 1
  fi
  if ! echo " $ips" | grep -q " $ORIGIN_IP "; then
    echo "DNS $host → $ips(ไม่ใช่ origin $ORIGIN_IP — CDN อาจถูกเปิด)"
    return 1
  fi
  return 0
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
for h in $DNS_HOSTS; do
  if msg="$(check_dns "$h")"; then :; else problems="${problems}- ${msg}"$'\n'; fi
done
for u in $HTTP_URLS; do
  if msg="$(check_http "$u")"; then :; else problems="${problems}- ${msg}"$'\n'; fi
done

now="$(TZ=Asia/Bangkok date '+%Y-%m-%d %H:%M')"

if [ "${1:-}" = "--test" ]; then
  if [ -z "$problems" ]; then result="ทุกอย่างปกติ ✅"; else result="พบปัญหา:"$'\n'"$problems"; fi
  echo "$result"
  send_line "🧪 [loanEasy monitor] ทดสอบแจ้งเตือน ($now)"$'\n'"$result"
  exit 0
fi

mkdir -p "$STATE_DIR"
state_file="$STATE_DIR/state"
prev="$(cat "$state_file" 2>/dev/null || echo OK)"

if [ -n "$problems" ]; then
  echo "DOWN" > "$state_file"
  echo "$now DOWN"$'\n'"$problems"
  if [ "$prev" != "DOWN" ]; then
    send_line "🔴 [loanEasy] ระบบมีปัญหา ($now)"$'\n'"$problems"$'\n'"ถ้าเป็น DNS/CDN: hPanel → Websites → Performance → CDN → ปิด"
  fi
else
  echo "OK" > "$state_file"
  if [ "$prev" = "DOWN" ]; then
    send_line "🟢 [loanEasy] ระบบกลับมาปกติแล้ว ($now)"
  fi
fi
