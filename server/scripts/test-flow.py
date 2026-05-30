#!/usr/bin/env python3
"""test-flow.py — end-to-end flow: user → API key → credits → LLM call.

Usage:
  python3 scripts/test-flow.py

Env overrides:
  API_BASE     http://localhost:18080
  GW_BASE      http://localhost:18081
  ADMIN_BASE   http://localhost:18082
  ADMIN_TOKEN  admin-dev-token
"""
import json, os, sys, time, urllib.request

API    = os.getenv("API_BASE",    "http://localhost:18080")
GW     = os.getenv("GW_BASE",     "http://localhost:18081")
ADMIN  = os.getenv("ADMIN_BASE",  "http://localhost:18082")
ATOKEN = os.getenv("ADMIN_TOKEN", "admin-dev-token")

email = f"test-{int(time.time())}@example.com"
pw    = "test123456"

def req(method, url, data=None, token=None):
    body = json.dumps(data).encode() if data else None
    hdrs = {"Content-Type": "application/json"}
    if token: hdrs["Authorization"] = f"Bearer {token}"
    r = urllib.request.Request(url, data=body, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(r) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as e:
        return {"_status": e.code, "_body": e.read().decode()}

print("=" * 50)
print(" ai-platform flow test")
print("=" * 50)
print(f"user: {email}\n")

# Step 1: Admin create user
print("--- Step 1: Create user via admin ---")
r = req("POST", f"{ADMIN}/api/admin/users",
        {"email": email, "password": pw, "role": 0}, ATOKEN)
uid = r.get("data", r).get("id", 0)
assert uid, f"create user failed: {r}"
print(f"  OKuser_id: {uid}")

# Step 2: Login
print("\n--- Step 2: Login ---")
r = req("POST", f"{API}/api/v1/auth/login",
        {"email": email, "password": pw})
jwt = r.get("token", "")
assert jwt, f"login failed: {r}"
print(f"  OKJWT: {jwt[:20]}...")

# Step 3: Create API key
print("\n--- Step 3: Create API key ---")
r = req("POST", f"{API}/api/v1/users/me/keys",
        {"name": "test-key", "unlimited_quota": True}, jwt)
ak = r.get("key", "")
assert ak, f"create key failed: {r}"
print(f"  OKAPI key: {ak}")

# Step 4: Add credits
print("\n--- Step 4: Add credits ---")
r = req("POST", f"{ADMIN}/api/admin/recharge?userId={uid}&amountCredits=1000000&note=test-topup",
        token=ATOKEN)
print(f"  OKrecharge: {json.dumps(r, ensure_ascii=False)[:80]}")

# Step 5: Check balance
print("\n--- Step 5: Check balance ---")
r = req("GET", f"{API}/api/v1/billing/balance", token=jwt)
print(f"  balance: {json.dumps(r, ensure_ascii=False)}")

print("\n" + "=" * 50)
print(" User ready!")
print("=" * 50)
print(f"email:    {email}")
print(f"api_key:  {ak}")
print(f"credits:  1,000,000")
print()
print("Test LLM call:")
print(f'  curl -H "Authorization: Bearer {ak}" \\')
print(f'    -d \'{{"model":"<model>","messages":[{{"role":"user","content":"hi"}}]}}\' \\')
print(f"    {GW}/v1/chat/completions")
