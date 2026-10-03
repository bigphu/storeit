#!/bin/sh
# Chạy Vite trong container dev. node_modules là volume riêng (bản cài trên Windows có
# binary native không chạy được trên Linux): cài lại khi package-lock.json đổi.
set -eu
cd /app
stamp=node_modules/.package-lock.sha1
want=$(sha1sum package-lock.json | cut -d' ' -f1)
if [ "$(cat "$stamp" 2>/dev/null)" != "$want" ]; then
  npm ci --no-audit --no-fund
  echo "$want" > "$stamp"
fi
exec npm run dev -- --host 0.0.0.0
