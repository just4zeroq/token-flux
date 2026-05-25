#!/usr/bin/env bash
# smoke-llm.sh — end-to-end smoke test for the LLM settlement / gateway flow.
#
# Prerequisites (operator must do these before running):
#
#   1. The api server must be running locally (start it in another terminal):
#        cd server && go run .
#      Both :8080 (api) and :8081 (gateway) must be reachable.
#
#   2. An OpenAI-compatible upstream must be running at $MOCK_OPENAI_BASE
#      (defaults to http://localhost:11434/v1 — a local Ollama works).
#      The upstream must serve the model named by $MOCK_UPSTREAM_MODEL
#      (default "mock-chat"). If using Ollama, e.g.:
#        ollama pull llama3.2:1b
#        MOCK_UPSTREAM_MODEL=llama3.2:1b ./smoke-llm.sh
#
#   3. An admin user (role >= 100) must already exist. Because the seeded
#      `platform` user has a disabled password, the operator should:
#        a) Register an admin via the API:
#             curl -X POST http://localhost:8080/api/v1/auth/register \
#               -d '{"email":"admin@example.com","password":"admin123456"}'
#           Then read the verification code from server logs and:
#             curl -X POST http://localhost:8080/api/v1/auth/verify-email \
#               -d '{"email":"admin@example.com","code":"<6-digit>"}'
#        b) Promote them to admin in psql:
#             UPDATE users SET role = 100 WHERE email = 'admin@example.com';
#      Then pass ADMIN_EMAIL / ADMIN_PASSWORD env vars (defaults below).
#
# Env overrides:
#   API_BASE             default http://localhost:8080
#   GW_BASE              default http://localhost:8081
#   MOCK_OPENAI_BASE     default http://localhost:11434/v1
#   MOCK_UPSTREAM_KEY    default sk-mock
#   MOCK_UPSTREAM_MODEL  default mock-chat
#   ADMIN_EMAIL          default admin@example.com
#   ADMIN_PASSWORD       default admin123456
#
# IMPORTANT — manual DB step required:
#   The api offers no endpoint to mark model-keys or key-model bindings as
#   `active` directly (TriggerKeyModelTest only schedules a test; no background
#   worker promotes the binding). After step 5 the script will print the
#   exact SQL the operator must run to flip both rows to status='active'
#   before the gateway can route requests. It then pauses for ENTER.

set -euo pipefail

echo "[smoke-llm] starting..."

# ---------- config ----------
API_BASE="${API_BASE:-http://localhost:8080}"
GW_BASE="${GW_BASE:-http://localhost:8081}"
MOCK_OPENAI_BASE="${MOCK_OPENAI_BASE:-http://localhost:11434/v1}"
MOCK_UPSTREAM_KEY="${MOCK_UPSTREAM_KEY:-sk-mock}"
MOCK_UPSTREAM_MODEL="${MOCK_UPSTREAM_MODEL:-mock-chat}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin123456}"

command -v jq >/dev/null 2>&1 || { echo "[smoke-llm] FAIL: jq is required but not installed"; exit 1; }
command -v curl >/dev/null 2>&1 || { echo "[smoke-llm] FAIL: curl is required"; exit 1; }

# ---------- helpers ----------

# req METHOD URL [JSON_BODY] [BEARER_TOKEN_OR_APIKEY]
# Echoes the response body. Fails (exits) if HTTP status is not 2xx.
req() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local auth="${4:-}"

  local tmp_body
  tmp_body="$(mktemp)"
  local status
  local -a args=(-sS -o "$tmp_body" -w '%{http_code}' -X "$method" "$url")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$body")
  fi
  if [[ -n "$auth" ]]; then
    args+=(-H "Authorization: Bearer $auth")
  fi
  status="$(curl "${args[@]}")" || {
    echo "[smoke-llm] FAIL: curl error calling $method $url" >&2
    cat "$tmp_body" >&2 || true
    rm -f "$tmp_body"
    exit 1
  }
  if [[ "$status" =~ ^2[0-9][0-9]$ ]]; then
    cat "$tmp_body"
    rm -f "$tmp_body"
    return 0
  fi
  echo "[smoke-llm] FAIL: $method $url returned HTTP $status" >&2
  cat "$tmp_body" >&2 || true
  rm -f "$tmp_body"
  exit 1
}

# req_apikey METHOD URL [JSON_BODY] APIKEY  — uses X-API-Key style (sk-…)
req_apikey() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local apikey="$4"

  local tmp_body
  tmp_body="$(mktemp)"
  local status
  local -a args=(-sS -o "$tmp_body" -w '%{http_code}' -X "$method" "$url"
                 -H "Authorization: Bearer $apikey")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' --data "$body")
  fi
  status="$(curl "${args[@]}")" || {
    echo "[smoke-llm] FAIL: curl error calling $method $url" >&2
    cat "$tmp_body" >&2 || true
    rm -f "$tmp_body"
    exit 1
  }
  if [[ "$status" =~ ^2[0-9][0-9]$ ]]; then
    cat "$tmp_body"
    rm -f "$tmp_body"
    return 0
  fi
  echo "[smoke-llm] FAIL: $method $url returned HTTP $status" >&2
  cat "$tmp_body" >&2 || true
  rm -f "$tmp_body"
  exit 1
}

# ---------- step 1: server precheck ----------
echo "[smoke-llm] step 1: server precheck ($API_BASE/health, $GW_BASE/health)"
curl -sf "$API_BASE/health" >/dev/null || {
  echo "[smoke-llm] FAIL: api server at $API_BASE not reachable. Start it first: (cd server && go run .)"
  exit 1
}
curl -sf "$GW_BASE/health" >/dev/null || {
  echo "[smoke-llm] FAIL: gateway server at $GW_BASE not reachable. Start it first: (cd server && go run .)"
  exit 1
}

# ---------- step 2: register + verify + login a fresh consumer ----------
SUFFIX="$RANDOM$RANDOM"
CONSUMER_EMAIL="llmuser_smoke_${SUFFIX}@example.com"
CONSUMER_PASSWORD="smoke-password-123"
echo "[smoke-llm] step 2: register consumer $CONSUMER_EMAIL"

REG_BODY="$(jq -nc --arg e "$CONSUMER_EMAIL" --arg p "$CONSUMER_PASSWORD" '{email:$e,password:$p}')"
req POST "$API_BASE/api/v1/auth/register" "$REG_BODY" >/dev/null

# Read the most-recent register code for this email from goose-tracked email_verifications
# is not possible without DB access; the server logs it instead. We grep server logs from
# stdin? Not feasible from this script — instead require the operator to either:
#   - disable email verification, or
#   - use a pre-verified user (set status=1).
#
# To keep the smoke test self-contained, we directly call verify-email with a synthesised
# request that will only succeed when the operator either bypasses verification or pipes
# the code via env. We honour CONSUMER_VERIFY_CODE if set.
if [[ -n "${CONSUMER_VERIFY_CODE:-}" ]]; then
  echo "[smoke-llm]   using CONSUMER_VERIFY_CODE from env"
  VERIFY_BODY="$(jq -nc --arg e "$CONSUMER_EMAIL" --arg c "$CONSUMER_VERIFY_CODE" \
    '{email:$e,code:$c}')"
  req POST "$API_BASE/api/v1/auth/verify-email" "$VERIFY_BODY" >/dev/null
else
  cat >&2 <<EOF
[smoke-llm] NOTE: email verification step skipped. The consumer was registered but
  status=0 (pending verification). To complete login the operator must EITHER:
    a) read the 6-digit code from the api server logs ("[EMAIL] Verification code for $CONSUMER_EMAIL: XXXXXX"),
       set CONSUMER_VERIFY_CODE=XXXXXX, and re-run, OR
    b) manually activate the user in psql:
         UPDATE users SET status = 1 WHERE email = '$CONSUMER_EMAIL';
       then re-run the script with CONSUMER_EMAIL=$CONSUMER_EMAIL CONSUMER_PASSWORD=$CONSUMER_PASSWORD
EOF
  read -r -p "[smoke-llm] press ENTER after activating the user to continue, or Ctrl-C to abort: " _
fi

echo "[smoke-llm] step 2: login consumer"
LOGIN_BODY="$(jq -nc --arg e "$CONSUMER_EMAIL" --arg p "$CONSUMER_PASSWORD" '{email:$e,password:$p}')"
LOGIN_RES="$(req POST "$API_BASE/api/v1/auth/login" "$LOGIN_BODY")"
CONSUMER_JWT="$(jq -r '.token // empty' <<<"$LOGIN_RES")"
CONSUMER_USER_ID="$(jq -r '.user.id // empty' <<<"$LOGIN_RES")"
if [[ -z "$CONSUMER_JWT" || -z "$CONSUMER_USER_ID" ]]; then
  echo "[smoke-llm] FAIL: consumer login did not return .token / .user.id"
  echo "$LOGIN_RES"
  exit 1
fi
echo "[smoke-llm]   consumer user_id=$CONSUMER_USER_ID"

# ---------- step 2b: admin login ----------
echo "[smoke-llm] step 2b: login admin $ADMIN_EMAIL"
ADMIN_LOGIN_BODY="$(jq -nc --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" '{email:$e,password:$p}')"
ADMIN_LOGIN_RES="$(req POST "$API_BASE/api/v1/auth/login" "$ADMIN_LOGIN_BODY")" || {
  echo "[smoke-llm] FAIL: admin login failed. Register an admin user first and promote to role=100,"
  echo "  or set ADMIN_EMAIL / ADMIN_PASSWORD env vars."
  exit 1
}
ADMIN_JWT="$(jq -r '.token // empty' <<<"$ADMIN_LOGIN_RES")"
ADMIN_ROLE="$(jq -r '.user.role // empty' <<<"$ADMIN_LOGIN_RES")"
if [[ -z "$ADMIN_JWT" ]]; then
  echo "[smoke-llm] FAIL: admin login returned no token: $ADMIN_LOGIN_RES"
  exit 1
fi
if [[ "$ADMIN_ROLE" != "100" ]]; then
  echo "[smoke-llm] FAIL: admin user role is '$ADMIN_ROLE', need >=100. Run:"
  echo "  UPDATE users SET role = 100 WHERE email = '$ADMIN_EMAIL';"
  exit 1
fi

# ---------- step 3: recharge consumer ----------
echo "[smoke-llm] step 3: recharge consumer (1,000,000 credits)"
REF_ID="$RANDOM$RANDOM"
RECHARGE_BODY="$(jq -nc --argjson amt 1000000 --arg rt "manual_recharge" --argjson rid "$REF_ID" \
  '{amount_credits:$amt, ref_type:$rt, ref_id:$rid}')"
req POST "$API_BASE/api/v1/billing/recharge" "$RECHARGE_BODY" "$CONSUMER_JWT" >/dev/null

# ---------- step 4: create virtual key ----------
echo "[smoke-llm] step 4: create virtual key"
KEY_RES="$(req POST "$API_BASE/api/v1/users/me/keys" \
  '{"name":"llm smoke","unlimited_quota":true}' "$CONSUMER_JWT")"
VIRTUAL_KEY="$(jq -r '.key // empty' <<<"$KEY_RES")"
if [[ -z "$VIRTUAL_KEY" || "${VIRTUAL_KEY:0:3}" != "sk-" ]]; then
  echo "[smoke-llm] FAIL: virtual key creation returned no sk- key: $KEY_RES"
  exit 1
fi
echo "[smoke-llm]   virtual key: ${VIRTUAL_KEY:0:10}…"

# ---------- step 5a: admin creates channel ----------
echo "[smoke-llm] step 5a: admin creates channel mock-openai"
CHANNEL_CODE="mock-openai-${SUFFIX}"
PROTOCOLS_JSON="$(jq -nc --arg url "$MOCK_OPENAI_BASE" \
  '{"openai-compatible": {base_url: $url}}' | jq -c .)"
CH_BODY="$(jq -nc --arg code "$CHANNEL_CODE" --arg name "mock-openai-$SUFFIX" \
  --arg pj "$PROTOCOLS_JSON" \
  '{code:$code, name:$name, description:"smoke test channel", protocols_json:$pj}')"
CH_RES="$(req POST "$API_BASE/api/v1/admin/llm/channels" "$CH_BODY" "$ADMIN_JWT")"
CHANNEL_ID="$(jq -r '.id // empty' <<<"$CH_RES")"
if [[ -z "$CHANNEL_ID" ]]; then
  echo "[smoke-llm] FAIL: channel creation returned no id: $CH_RES"
  exit 1
fi
echo "[smoke-llm]   channel id=$CHANNEL_ID"

echo "[smoke-llm] step 5a: approve channel (status=active)"
req PUT "$API_BASE/api/v1/admin/llm/channels/$CHANNEL_ID/review" \
  '{"status":"active","review_note":"smoke approved"}' "$ADMIN_JWT" >/dev/null

# ---------- step 5b: admin creates model spec ----------
echo "[smoke-llm] step 5b: admin creates model spec test/mock-chat"
SPEC_BODY="$(jq -nc \
  --arg dev "test" \
  --arg mn "mock-chat-$SUFFIX" \
  --arg mc "test/mock-chat-$SUFFIX" \
  --arg caps '["chat"]' \
  '{developer_name:$dev, model_name:$mn, model_code:$mc,
    display_name:"Mock Chat (smoke)", capabilities_json:$caps,
    supports_stream:true}')"
SPEC_RES="$(req POST "$API_BASE/api/v1/admin/llm/models" "$SPEC_BODY" "$ADMIN_JWT")"
SPEC_ID="$(jq -r '.id // empty' <<<"$SPEC_RES")"
MODEL_CODE="$(jq -r '.model_code // empty' <<<"$SPEC_RES")"
if [[ -z "$SPEC_ID" ]]; then
  echo "[smoke-llm] FAIL: model spec creation returned no id: $SPEC_RES"
  exit 1
fi
echo "[smoke-llm]   model spec id=$SPEC_ID code=$MODEL_CODE"

echo "[smoke-llm] step 5b: approve model spec (status=active)"
req PUT "$API_BASE/api/v1/admin/llm/models/$SPEC_ID/review" \
  '{"status":"active","review_note":"smoke approved"}' "$ADMIN_JWT" >/dev/null

# ---------- step 5c: admin sets price ----------
echo "[smoke-llm] step 5c: admin upserts price for chat capability"
PRICE_BODY='{"capability":"chat","cache_hit_price_per_1k":100,"cache_miss_price_per_1k":100,"output_price_per_1k":100,"status":"active"}'
req POST "$API_BASE/api/v1/admin/llm/models/$SPEC_ID/prices" "$PRICE_BODY" "$ADMIN_JWT" >/dev/null

# ---------- step 5d: provider (admin, role 100>=10) creates model key ----------
echo "[smoke-llm] step 5d: provider creates model key on channel $CHANNEL_ID"
MK_BODY="$(jq -nc --argjson cid "$CHANNEL_ID" --arg key "$MOCK_UPSTREAM_KEY" \
  '{channel_id:$cid, name:"smoke upstream key", key:$key, quota_limit_credits:0}')"
MK_RES="$(req POST "$API_BASE/api/v1/provider/llm/model-keys" "$MK_BODY" "$ADMIN_JWT")"
MODEL_KEY_ID="$(jq -r '.id // empty' <<<"$MK_RES")"
if [[ -z "$MODEL_KEY_ID" ]]; then
  echo "[smoke-llm] FAIL: model key creation returned no id: $MK_RES"
  exit 1
fi
echo "[smoke-llm]   model key id=$MODEL_KEY_ID"

# ---------- step 5e: bind key to spec ----------
echo "[smoke-llm] step 5e: provider binds model key to spec"
BIND_BODY="$(jq -nc --argjson sid "$SPEC_ID" --arg um "$MOCK_UPSTREAM_MODEL" \
  '{model_spec_id:$sid, upstream_model_name:$um, quota_limit_credits:0, provider_share_bps:5000}')"
BIND_RES="$(req POST "$API_BASE/api/v1/provider/llm/model-keys/$MODEL_KEY_ID/models" \
  "$BIND_BODY" "$ADMIN_JWT")"
KEY_MODEL_ID="$(jq -r '.id // empty' <<<"$BIND_RES")"
BIND_STATUS="$(jq -r '.status // empty' <<<"$BIND_RES")"
if [[ -z "$KEY_MODEL_ID" ]]; then
  echo "[smoke-llm] FAIL: key-model binding returned no id: $BIND_RES"
  exit 1
fi
echo "[smoke-llm]   key-model id=$KEY_MODEL_ID status=$BIND_STATUS"

# ---------- step 5f: MANUAL DB ACTIVATION ----------
cat <<EOF

[smoke-llm] ====== MANUAL STEP REQUIRED ======
The api currently has no endpoint to mark a model_key or a key_model binding
as 'active' — TriggerKeyModelTest only schedules a test and no background worker
exists yet. Open a psql session and run:

    UPDATE llm_model_keys SET status = 'active' WHERE id = $MODEL_KEY_ID;
    UPDATE llm_model_key_models SET status = 'active' WHERE id = $KEY_MODEL_ID;

After that, the gateway will route requests for model_code='$MODEL_CODE'
through this binding to $MOCK_OPENAI_BASE.
==================================================
EOF
read -r -p "[smoke-llm] press ENTER once the two UPDATEs are committed: " _

# ---------- step 5 verify: GET /v1/models lists the model ----------
echo "[smoke-llm] step 5 verify: GET $GW_BASE/v1/models"
MODELS_RES="$(req_apikey GET "$GW_BASE/v1/models" "" "$VIRTUAL_KEY")"
if ! jq -e --arg mc "$MODEL_CODE" '.data[]? | select(.id == $mc)' <<<"$MODELS_RES" >/dev/null; then
  echo "[smoke-llm] FAIL: $MODEL_CODE not present in /v1/models response:"
  echo "$MODELS_RES"
  exit 1
fi
echo "[smoke-llm]   $MODEL_CODE present in /v1/models"

# ---------- step 6: chat completion ----------
echo "[smoke-llm] step 6: POST $GW_BASE/v1/chat/completions"
CHAT_BODY="$(jq -nc --arg model "$MODEL_CODE" \
  '{model:$model, messages:[{role:"user", content:"Say hello in 5 words."}]}')"
CHAT_RES="$(req_apikey POST "$GW_BASE/v1/chat/completions" "$CHAT_BODY" "$VIRTUAL_KEY")"
echo "[smoke-llm]   chat response:"
echo "$CHAT_RES" | jq . 2>/dev/null || echo "$CHAT_RES"

if ! jq -e '.choices[0].message' <<<"$CHAT_RES" >/dev/null 2>&1; then
  echo "[smoke-llm] WARN: response did not contain .choices[0].message — check upstream compatibility"
fi

# ---------- step 6 verify (printout only) ----------
cat <<EOF

[smoke-llm] ====== POST-RUN DB VERIFICATION ======
Run these in psql to confirm settlement worked end-to-end:

    SELECT id, request_id, model_code, status, total_credits_charged
      FROM llm_usage_records
     WHERE user_id = $CONSUMER_USER_ID
     ORDER BY id DESC LIMIT 5;

    SELECT id, usage_record_id, status, total_credits, platform_credits, provider_credits
      FROM settlement_records
     WHERE user_id = $CONSUMER_USER_ID
     ORDER BY id DESC LIMIT 5;

    SELECT t.id, t.tx_type, t.ref_type, t.ref_id, t.created_at,
           e.account_id, e.delta_micro, e.balance_after_micro
      FROM transactions t
      JOIN transaction_entries e ON e.tx_id = t.id
     WHERE t.created_at > now() - interval '5 minutes'
     ORDER BY t.id DESC, e.id ASC
     LIMIT 30;

==================================================
EOF

echo "[smoke-llm] PASS"
