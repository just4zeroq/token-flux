#!/usr/bin/env bash
# test-flow.sh — end-to-end flow: user register → API key → credits → LLM call
#
# Usage: bash scripts/test-flow.sh
#
# Env overrides:
#   API_BASE     default http://localhost:18080
#   GW_BASE      default http://localhost:18081
#   ADMIN_BASE   default http://localhost:18082
#   ADMIN_TOKEN  default admin-dev-token

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:18080}"
GW_BASE="${GW_BASE:-http://localhost:18081}"
ADMIN_BASE="${ADMIN_BASE:-http://localhost:18082}"
ADMIN_TOKEN="${ADMIN_TOKEN:-admin-dev-token}"

# Find a working Python
PY=$(command -v python3 || command -v python || echo "/d/soft/install/miniconda3/python")
json_val() { echo "$1" | "$PY" -c "
import sys,json
try:
  d=json.load(sys.stdin)
  val = eval('d' + '$2'.replace('[','[\"').replace(']','\"]'))
  print(val if isinstance(val,str) else val)
except: print(0)
" 2>/dev/null || echo "0"; }

TEST_EMAIL="test-$(date +%s)@example.com"
TEST_PASS="test123456"

echo "=========================================="
echo " ai-platform flow test"
echo "=========================================="
echo "user:  $TEST_EMAIL"
echo ""

ok()   { echo "  ✅ $1"; }
die()  { echo "  ❌ $1"; exit 1; }

# ==== 1. Create user via admin (status=1, no email verify needed) ====
echo "--- Step 1: Create user via admin ---"
ADMIN_CREATE=$(curl -sS -X POST "$ADMIN_BASE/api/admin/users" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASS\",\"role\":0}")
echo "  response: $ADMIN_CREATE"
USER_ID=$(json_val "$ADMIN_CREATE" "['data']['id']")
if [ "$USER_ID" = "0" ]; then die "admin create user failed: $ADMIN_CREATE"; fi
ok "user_id: $USER_ID"

# ==== 2. Login → JWT ====
echo ""
echo "--- Step 2: Login ---"
LOGIN=$(curl -sS -X POST "$API_BASE/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASS\"}")
JWT=$(json_val "$LOGIN" "['token']")
if [ "$JWT" = "0" ] || [ -z "$JWT" ]; then die "login failed: $LOGIN"; fi
ok "JWT: ${JWT:0:20}..."

# ==== 3. Create API key ====
echo ""
echo "--- Step 3: Create API key ---"
KEY_OUT=$(curl -sS -X POST "$API_BASE/api/v1/users/me/keys" \
  -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $JWT" \
  -d '{"name":"test-key","unlimited_quota":true}')
API_KEY=$(json_val "$KEY_OUT" "['key']")
if [ "$API_KEY" = "0" ] || [ -z "$API_KEY" ]; then die "create key failed: $KEY_OUT"; fi
ok "API key: $API_KEY"

# ==== 4. Add credits (admin) ====
echo ""
echo "--- Step 4: Add credits ---"
RECHARGE=$(curl -sS -X POST "$ADMIN_BASE/api/admin/recharge?userId=$USER_ID&amountCredits=1000000&note=test-topup" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
ok "recharge: $(echo $RECHARGE | head -c 80)"

# ==== 5. Check balance ====
BAL=$(curl -sS -X GET "$API_BASE/api/v1/billing/balance" \
  -H "Authorization: Bearer $JWT")
echo "  balance: $BAL"

echo ""
echo "=========================================="
echo " User ready!"
echo "=========================================="
echo "email:    $TEST_EMAIL"
echo "api_key:  $API_KEY"
echo "credits:  1,000,000"
echo ""
echo "Test LLM call:"
echo "  curl -H 'Authorization: Bearer $API_KEY' \\"
echo "    -d '{\"model\":\"<model>\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}' \\"
echo "    $GW_BASE/v1/chat/completions"
echo ""
echo "Check balance:"
echo "  curl -H 'Authorization: Bearer $JWT' $API_BASE/api/v1/billing/balance"
