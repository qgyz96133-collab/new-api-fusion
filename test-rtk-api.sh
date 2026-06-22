#!/bin/bash

# RTK Settings Test Script
# Tests authentication, GET/PUT/POST/RESET endpoints, and DB persistence

BASE_URL="http://localhost:3001"
CONTAINER="new-api-rtk-persist"

echo "=== RTK Settings Configuration Test ==="
echo ""

# Step 1: Login as admin user (get session cookie)
echo "Step 1: Logging in as admin user..."
COOKIE_FILE=$(mktemp)
curl -s -X POST "$BASE_URL/api/user/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}' \
  -c "$COOKIE_FILE" > /tmp/login_response.json

LOGIN_SUCCESS=$(python3 -c "import json; print(json.load(open('/tmp/login_response.json')).get('success', False))" 2>/dev/null)

if [ "$LOGIN_SUCCESS" != "True" ]; then
  echo "❌ Login failed!"
  cat /tmp/login_response.json | python3 -m json.tool 2>/dev/null
  rm -f "$COOKIE_FILE"
  exit 1
fi

echo "✅ Login successful"
echo ""

# Auth headers helper
AUTH_HEADERS="-H \"New-Api-User: 1\" -b \"$COOKIE_FILE\""

# Step 2: Get current RTK settings
echo "Step 2: Getting current RTK settings..."
SETTINGS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "$SETTINGS_RESPONSE" | python3 -m json.tool
echo ""

# Step 3: Update RTK settings
echo "Step 3: Updating RTK settings (level 3, caveman enabled)..."
UPDATE_RESPONSE=$(curl -s -X PUT "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "rtk_enabled": true,
    "rtk_compression_level": 3,
    "rtk_min_tokens": 150,
    "rtk_max_tokens": 8000,
    "caveman_enabled": true,
    "caveman_mode_level": 3,
    "caveman_min_tokens": 100,
    "enable_tool_call_validation": true,
    "enable_orphan_tool_fix": true,
    "enable_gemini_schema_cleaning": true,
    "enable_claude_normalization": true,
    "enable_remote_image_fetch": true
  }')

echo "$UPDATE_RESPONSE" | python3 -m json.tool
echo ""

# Step 4: Verify settings were saved
echo "Step 4: Verifying settings were saved..."
VERIFY_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "$VERIFY_RESPONSE" | python3 -m json.tool
echo ""

# Step 5: Check database
echo "Step 5: Checking database for RTK settings..."
docker exec "$CONTAINER" sqlite3 /data/one-api.db \
  "SELECT key, value FROM options WHERE key LIKE 'rtk_setting.%' ORDER BY key;"
echo ""

# Step 6: Test reset
echo "Step 6: Testing settings reset..."
RESET_RESPONSE=$(curl -s -X POST "$BASE_URL/api/user/rtk/settings/reset" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "$RESET_RESPONSE" | python3 -m json.tool
echo ""

# Step 7: Verify reset
echo "Step 7: Verifying reset (should be defaults)..."
AFTER_RESET=$(curl -s -X GET "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "$AFTER_RESET" | python3 -m json.tool
echo ""

# Step 8: Get compression stats
echo "Step 8: Getting compression stats..."
STATS_RESPONSE=$(curl -s -X GET "$BASE_URL/api/user/rtk/stats" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "$STATS_RESPONSE" | python3 -m json.tool
echo ""

# Step 9: Restart persistence test
echo "Step 9: Testing restart persistence..."
# Re-apply custom settings
curl -s -X PUT "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1" \
  -H "Content-Type: application/json" \
  -d '{"rtk_enabled":true,"rtk_compression_level":3,"rtk_min_tokens":150,"rtk_max_tokens":8000,"caveman_enabled":true,"caveman_mode_level":3,"caveman_min_tokens":100,"enable_tool_call_validation":true,"enable_orphan_tool_fix":true,"enable_gemini_schema_cleaning":true,"enable_claude_normalization":true,"enable_remote_image_fetch":true}' > /dev/null

echo "Restarting container..."
docker restart "$CONTAINER"
sleep 5

# Re-login after restart
curl -s -X POST "$BASE_URL/api/user/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456"}' \
  -c "$COOKIE_FILE" > /dev/null

AFTER_RESTART=$(curl -s -X GET "$BASE_URL/api/user/rtk/settings" \
  -b "$COOKIE_FILE" \
  -H "New-Api-User: 1")

echo "Settings after restart:"
echo "$AFTER_RESTART" | python3 -c "
import sys, json
data = json.load(sys.stdin)
if data.get('success'):
    d = data['data']
    print(f'  compression_level={d[\"rtk_compression_level\"]}, caveman_enabled={d[\"caveman_enabled\"]}, max_tokens={d[\"rtk_max_tokens\"]}')
    if d['rtk_compression_level'] == 3 and d['caveman_enabled'] == True and d['rtk_max_tokens'] == 8000:
        print('  ✅ Settings persisted correctly after restart!')
    else:
        print('  ❌ Settings did NOT persist correctly!')
else:
    print('  ❌ Failed to get settings after restart')
"

# Cleanup
rm -f "$COOKIE_FILE"

echo ""
echo "=== Test Complete ==="
