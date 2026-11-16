#!/usr/bin/env bash
# ==============================================================================
# Concurrency Stress Test: Demonstrating Race Condition Prevention
# Simulates N concurrent users attempting to buy the LAST available item (Stock=1).
# ==============================================================================

set -euo pipefail

BASE_URL="${API_URL:-http://localhost:8080/api/v1}"
CONCURRENCY=10
PRODUCT_ID=5 # "Limited Edition Cyberpunk Jacket" with Quantity = 1

echo "======================================================================"
echo "⚡ CONCURRENCY RACE CONDITION TEST"
echo "Target: Product ID ${PRODUCT_ID} (Initial Stock: 1)"
echo "Simulating ${CONCURRENCY} concurrent shoppers placing orders simultaneously"
echo "======================================================================"

# 1. Register a test user and obtain JWT token
EMAIL="buyer_$(date +%s)@test.com"
REGISTER_PAYLOAD=$(cat <<EOF
{
  "email": "${EMAIL}",
  "password": "password123",
  "first_name": "Concurrent",
  "last_name": "Tester"
}
EOF
)

echo "1. Creating test user: ${EMAIL}..."
AUTH_RESPONSE=$(curl -s -X POST "${BASE_URL}/auth/register" \
  -H "Content-Type: application/json" \
  -d "${REGISTER_PAYLOAD}")

TOKEN=$(echo "${AUTH_RESPONSE}" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "${TOKEN}" ]; then
  echo "❌ Error: Could not obtain JWT token. Is the server running at ${BASE_URL}?"
  exit 1
fi
echo "✓ Authenticated with JWT token."

# 2. Check initial inventory
echo "2. Initial Inventory Check for Product ${PRODUCT_ID}..."
curl -s -X GET "${BASE_URL}/products/${PRODUCT_ID}" | grep -o '"name":"[^"]*' || true

# 3. Fire concurrent checkout requests
echo "3. Firing ${CONCURRENCY} parallel checkout requests..."
ORDER_PAYLOAD=$(cat <<EOF
{
  "items": [{"product_id": ${PRODUCT_ID}, "quantity": 1}],
  "shipping_address": "742 Evergreen Terrace, Sector 7G"
}
EOF
)

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

for i in $(seq 1 "${CONCURRENCY}"); do
  (
    HTTP_CODE=$(curl -s -o "${TMP_DIR}/resp_${i}.json" -w "%{http_code}" -X POST "${BASE_URL}/orders" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${TOKEN}" \
      -d "${ORDER_PAYLOAD}")
    echo "${HTTP_CODE}" > "${TMP_DIR}/status_${i}.txt"
  ) &
done

# Wait for all background requests to complete
wait

echo "4. Results Analysis:"
SUCCESS_COUNT=0
CONFLICT_COUNT=0
OTHER_COUNT=0

for i in $(seq 1 "${CONCURRENCY}"); do
  STATUS=$(cat "${TMP_DIR}/status_${i}.txt")
  if [ "${STATUS}" -eq 201 ]; then
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    echo "  Req #${i}: [201 CREATED]  -> Successfully acquired lock and ordered item!"
  elif [ "${STATUS}" -eq 409 ]; then
    CONFLICT_COUNT=$((CONFLICT_COUNT + 1))
    echo "  Req #${i}: [409 CONFLICT] -> Blocked/Rejected (Stock exhausted, 0 oversell)"
  else
    OTHER_COUNT=$((OTHER_COUNT + 1))
    echo "  Req #${i}: [${STATUS}] -> Other response"
  fi
done

echo "======================================================================"
echo "📊 FINAL SUMMARY:"
echo "  Total Requests:    ${CONCURRENCY}"
echo "  Successful Orders: ${SUCCESS_COUNT} (Expected: exactly 1)"
echo "  Rejected (409):    ${CONFLICT_COUNT} (Expected: exactly $((CONCURRENCY - 1)))"
echo "  Other Errors:      ${OTHER_COUNT}"
echo "======================================================================"

if [ "${SUCCESS_COUNT}" -eq 1 ] && [ "${CONFLICT_COUNT}" -eq $((CONCURRENCY - 1)) ]; then
  echo "🎉 SUCCESS: Database consistency preserved! Zero overselling observed."
else
  echo "⚠️ Warning: Expected exactly 1 success and $((CONCURRENCY - 1)) conflicts."
fi
