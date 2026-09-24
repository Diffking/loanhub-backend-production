#!/usr/bin/env bash
# ============================================================
# ฟังก์ชันส่งข้อความ LINE ใช้ร่วมกันระหว่างสคริปต์ ops
#   source /var/www/loaneasy/scripts/lib/line-notify.sh
#   line_notify "ข้อความ"
#
# อ่าน LINE_CHANNEL_ACCESS_TOKEN และ ALERT_LINE_TO จาก $ENV_FILE
# (ค่าเริ่มต้น /var/www/loaneasy/.env)
# ============================================================

ENV_FILE="${ENV_FILE:-/var/www/loaneasy/.env}"

# อ่านค่าเฉพาะ key ที่ต้องใช้ (ไม่ source .env ทั้งไฟล์)
line_env_get() {
  grep -E "^$1=" "$ENV_FILE" 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '\r' | sed -e 's/^["'\'']//' -e 's/["'\'']$//'
}

line_notify() {
  local text="$1" token to json
  token="$(line_env_get LINE_CHANNEL_ACCESS_TOKEN)"
  to="$(line_env_get ALERT_LINE_TO)"
  if [ -z "$token" ] || [ -z "$to" ]; then
    echo "WARN: LINE_CHANNEL_ACCESS_TOKEN / ALERT_LINE_TO not set in $ENV_FILE — skip LINE" >&2
    return 1
  fi

  # escape สำหรับ JSON: \ " และขึ้นบรรทัดใหม่
  local bs='\' q='"' cr=$'\r' nl='
'
  text="${text//"$bs"/"$bs$bs"}"
  text="${text//"$q"/"$bs$q"}"
  text="${text//"$cr"/}"
  text="${text//"$nl"/"${bs}n"}"

  local one
  for one in $(echo "$to" | tr ',' ' '); do
    json="{\"to\":\"$one\",\"messages\":[{\"type\":\"text\",\"text\":\"$text\"}]}"
    curl -sS -m 15 -o /dev/null -w "LINE push -> %{http_code}\n" \
      -X POST https://api.line.me/v2/bot/message/push \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $token" \
      -d "$json"
  done
}
