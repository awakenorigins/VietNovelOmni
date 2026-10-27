#!/usr/bin/env bash
# verify-endpoints.sh — audit toàn bộ endpoint của omni-dashboard.
# Usage: ./verify-endpoints.sh [base_url]  (default http://127.0.0.1:8770)
set -u
BASE="${1:-http://127.0.0.1:8770}"
AUTH_JSON="/root/.openclaw/workspace/novels/.omni-app/auth.json"
USER=$(python3 -c "import json;print(json.load(open('$AUTH_JSON'))['user'])")
PASS=$(python3 -c "import json;print(json.load(open('$AUTH_JSON'))['password'])")

PASS_N=0; FAIL_N=0
check() { # name method url expected [curl extra args...]
  local name="$1" method="$2" url="$3" want="$4"; shift 4
  local got
  got=$(curl -s -o /dev/null -w '%{http_code}' -X "$method" "$@" "$BASE$url")
  if [ "$got" = "$want" ]; then
    printf 'PASS  %-42s %s %s → %s\n' "$name" "$method" "$url" "$got"; PASS_N=$((PASS_N+1))
  else
    printf 'FAIL  %-42s %s %s → %s (muốn %s)\n' "$name" "$method" "$url" "$got" "$want"; FAIL_N=$((FAIL_N+1))
  fi
}

SLUG=$(curl -s "$BASE/api/novels" | python3 -c "import json,sys;d=json.load(sys.stdin);print(d[0]['slug'] if d else 'trong-sinh')")

echo "== GET public =="
check "index"                GET "/" 200
check "novel page"           GET "/novel/$SLUG" 200
check "novel 404"            GET "/novel/khong-ton-tai-xyz" 404
check "novel slug xấu"       GET "/novel/..%2Fetc" 400
check "api novels"           GET "/api/novels" 200
check "api novel"            GET "/api/novels/$SLUG" 200
check "api jobs"             GET "/api/jobs" 200
check "api auto"             GET "/api/auto" 200
check "api library"          GET "/api/library" 200
check "jobs page"            GET "/jobs" 200
check "library page"         GET "/library" 200
check "download traversal"   GET "/download/$SLUG/..%2F..%2Fetc%2Fpasswd" 400
check "library traversal"    GET "/library/tien-hiep/..%2Fx.epub" 400

echo "== GET cần auth =="
check "create noauth"        GET "/create" 401
check "settings noauth"      GET "/settings" 401
check "auto noauth"          GET "/auto" 401
check "create auth"          GET "/create" 200 -u "$USER:$PASS"
check "settings auth"        GET "/settings" 200 -u "$USER:$PASS"
check "auto auth"            GET "/auto" 200 -u "$USER:$PASS"

echo "== POST không auth → 401 =="
check "POST create noauth"   POST "/create" 401
check "POST settings noauth" POST "/settings" 401
check "POST settings/test"   POST "/settings/test" 401
check "POST auto/start"      POST "/auto/start" 401
check "POST auto/stop"       POST "/auto/stop" 401
check "POST export noauth"   POST "/novel/$SLUG/export" 401
check "POST cancel noauth"   POST "/jobs/xxxx/cancel" 401

echo "== POST auth (no-op an toàn) =="
# settings/test với base_url không tồn tại → 200 JSON {ok:false}
check "settings/test auth"   POST "/settings/test" 200 -u "$USER:$PASS" --data "kind=custom&base_url=http://127.0.0.1:1/v1&api_key=x"
# auto/stop khi không chạy → redirect 303 (no-op an toàn)
check "auto/stop auth noop"  POST "/auto/stop" 303 -u "$USER:$PASS"
# settings action rác → redirect 303 kèm err (không ghi gì)
check "settings action xấu"  POST "/settings" 303 -u "$USER:$PASS" --data "action=khong-co"
# create thiếu title → redirect 303 err (không tạo gì)
check "create thiếu title"   POST "/create" 303 -u "$USER:$PASS" --data "seeding=x"

echo
echo "TỔNG: PASS=$PASS_N FAIL=$FAIL_N"
[ "$FAIL_N" -eq 0 ]
