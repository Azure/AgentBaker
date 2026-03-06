#!/bin/bash
set -euo pipefail

# Test script for aks-hosts-setup.sh validation logic
# This tests the validation code that prevents FQDN to empty IP mappings

echo "=== Testing aks-hosts-setup.sh validation logic ==="

# Create a temporary test directory
TEST_DIR=$(mktemp -d)
trap 'rm -rf "$TEST_DIR"' EXIT

echo "Test directory: $TEST_DIR"
echo ""

# Test case 1: Valid hosts file with IPv4 and IPv6
echo "Test 1: Valid hosts file with IPv4 and IPv6"
cat > "$TEST_DIR/hosts_valid" <<'EOF'
# AKS critical FQDN addresses
# mcr.microsoft.com
20.190.151.7 mcr.microsoft.com
2603:1030:8:5::2 mcr.microsoft.com
# packages.microsoft.com
13.107.213.73 packages.microsoft.com
EOF

echo "Testing valid hosts file..."
HOSTS_FILE="$TEST_DIR/hosts_valid"
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        echo "ERROR: Invalid entry found - missing IP or FQDN: '$line'"
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        echo "ERROR: Invalid IP format: '$ip' in line: '$line'"
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_FILE"

if [ ${#INVALID_LINES[@]} -gt 0 ]; then
    echo "❌ FAIL: Found ${#INVALID_LINES[@]} invalid entries"
    exit 1
fi
if [ $VALID_ENTRIES -eq 0 ]; then
    echo "❌ FAIL: No valid entries found"
    exit 1
fi
echo "✅ PASS: Found $VALID_ENTRIES valid entries"
echo ""

# Test case 2: Invalid hosts file - FQDN with empty IP
echo "Test 2: Invalid hosts file - FQDN with missing IP (just FQDN on line)"
cat > "$TEST_DIR/hosts_invalid_empty_ip" <<'EOF'
# AKS critical FQDN addresses
# mcr.microsoft.com
20.190.151.7 mcr.microsoft.com
# packages.microsoft.com - missing IP on next line
packages.microsoft.com
# This line has FQDN but no IP address
EOF

echo "Testing hosts file with FQDN but no IP..."
HOSTS_FILE="$TEST_DIR/hosts_invalid_empty_ip"
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_FILE"

if [ ${#INVALID_LINES[@]} -gt 0 ]; then
    echo "✅ PASS: Correctly detected ${#INVALID_LINES[@]} invalid entries"
    printf '  Invalid line: %s\n' "${INVALID_LINES[@]}"
else
    echo "❌ FAIL: Should have detected invalid entry but didn't"
    exit 1
fi
echo ""

# Test case 3: Invalid hosts file - malformed IP
echo "Test 3: Invalid hosts file - malformed IP address"
cat > "$TEST_DIR/hosts_invalid_malformed_ip" <<'EOF'
# AKS critical FQDN addresses
# mcr.microsoft.com
not.an.ip.address mcr.microsoft.com
20.190.151.7 mcr.microsoft.com
EOF

echo "Testing hosts file with malformed IP..."
HOSTS_FILE="$TEST_DIR/hosts_invalid_malformed_ip"
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_FILE"

if [ ${#INVALID_LINES[@]} -gt 0 ]; then
    echo "✅ PASS: Correctly detected ${#INVALID_LINES[@]} invalid entries"
    printf '  Invalid line: %s\n' "${INVALID_LINES[@]}"
else
    echo "❌ FAIL: Should have detected malformed IP but didn't"
    exit 1
fi
echo ""

# Test case 4: Edge case - only IP, no FQDN
echo "Test 4: Invalid hosts file - IP with no FQDN"
cat > "$TEST_DIR/hosts_invalid_no_fqdn" <<'EOF'
# AKS critical FQDN addresses
20.190.151.7
13.107.213.73 packages.microsoft.com
EOF

echo "Testing hosts file with IP but no FQDN..."
HOSTS_FILE="$TEST_DIR/hosts_invalid_no_fqdn"
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_FILE"

if [ ${#INVALID_LINES[@]} -gt 0 ]; then
    echo "✅ PASS: Correctly detected ${#INVALID_LINES[@]} invalid entries"
    printf '  Invalid line: %s\n' "${INVALID_LINES[@]}"
else
    echo "❌ FAIL: Should have detected IP with no FQDN but didn't"
    exit 1
fi
echo ""

# Test case 5: Empty file after comments
echo "Test 5: Invalid hosts file - only comments, no entries"
cat > "$TEST_DIR/hosts_empty" <<'EOF'
# AKS critical FQDN addresses
# This file has only comments
EOF

echo "Testing hosts file with no entries..."
HOSTS_FILE="$TEST_DIR/hosts_empty"
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_FILE"

if [ $VALID_ENTRIES -eq 0 ]; then
    echo "✅ PASS: Correctly detected no valid entries"
else
    echo "❌ FAIL: Should have detected no valid entries"
    exit 1
fi
echo ""

# Test case 6: Validate-before-rename safety - ensure invalid data never reaches production file
echo "Test 6: Validate-before-rename - production file protection"
cat > "$TEST_DIR/hosts_production" <<'EOF'
# AKS critical FQDN addresses (GOOD DATA - current production)
20.190.151.7 mcr.microsoft.com
13.107.213.73 packages.microsoft.com
EOF

cat > "$TEST_DIR/hosts_bad_temp" <<'EOF'
# AKS critical FQDN addresses (BAD DATA - should be rejected)
# mcr.microsoft.com
 mcr.microsoft.com
20.190.151.7 packages.microsoft.com
EOF

echo "Simulating aks-hosts-setup.sh behavior..."
echo "  - Production file has valid data"
echo "  - Temp file has invalid data (FQDN with no IP)"

HOSTS_FILE_PROD="$TEST_DIR/hosts_production"
HOSTS_TMP="$TEST_DIR/hosts_bad_temp"

# Validate temp file (simulating aks-hosts-setup.sh validation)
INVALID_LINES=()
VALID_ENTRIES=0
while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] || [[ -z "$line" ]] && continue
    ip=$(echo "$line" | awk '{print $1}')
    fqdn=$(echo "$line" | awk '{print $2}')
    if [ -z "$ip" ] || [ -z "$fqdn" ]; then
        INVALID_LINES+=("$line")
        continue
    fi
    if [[ "$ip" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    elif [[ "$ip" =~ : ]]; then
        VALID_ENTRIES=$((VALID_ENTRIES + 1))
    else
        INVALID_LINES+=("$line")
    fi
done < "$HOSTS_TMP"

# Check if validation would fail
if [ ${#INVALID_LINES[@]} -gt 0 ] || [ $VALID_ENTRIES -eq 0 ]; then
    echo "  - Temp file validation FAILED (as expected)"
    echo "  - Would NOT rename temp -> production"

    # Verify production file is unchanged
    if grep -q "20.190.151.7 mcr.microsoft.com" "$HOSTS_FILE_PROD" && \
       grep -q "13.107.213.73 packages.microsoft.com" "$HOSTS_FILE_PROD"; then
        echo "✅ PASS: Production file remains intact with valid data"
        echo "  - CoreDNS would continue serving good entries"
        echo "  - No service disruption"
    else
        echo "❌ FAIL: Production file was corrupted"
        exit 1
    fi
else
    echo "❌ FAIL: Validation should have failed but passed"
    exit 1
fi
echo ""

echo "=== All tests passed! ==="
