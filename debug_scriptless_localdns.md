# Debugging Scriptless LocalDNS Failure

**VMSS Name:** `lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless`
**Resource Group:** `abe2e-westus2`
**Issue:** `systemctl restart localdns` times out after 30 seconds (exit 124)

---

## Quick Start: SSH into the Failed VM

### Step 1: Get VM Instance Details
```bash
# List VMSS instances
az vmss list-instances \
  --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 \
  --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 \
  -o table

# Get private IP
az vmss list-instance-connection-info \
  --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 \
  --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8
```

### Step 2: SSH via Bastion or Jump Box
```bash
# If using SSH key from scenario logs
SSH_KEY="e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless/sshkey"

# SSH into the VM (replace <PRIVATE_IP> with actual IP)
ssh -i "$SSH_KEY" azureuser@<PRIVATE_IP>
```

---

## Debugging Commands (Run on the VM)

### 1. Check Systemd Unit Status
```bash
# Check if localdns service exists
systemctl list-unit-files | grep localdns

# Check service status
systemctl status localdns --no-pager -l

# Show full unit file configuration
systemctl cat localdns

# Check if unit file exists on disk
ls -la /etc/systemd/system/localdns*
ls -la /usr/lib/systemd/system/localdns*
```

**Expected Output Analysis:**
- If `systemctl cat localdns` shows "No files found" → Unit was never created
- If it exists, check `Type=`, `ExecStart=`, `Restart=` directives
- Look for missing dependencies in `Requires=`, `After=`, `Before=`

### 2. Check if CoreDNS Process is Running
```bash
# Check for running coredns process
ps aux | grep coredns

# Check if localdns is listening on expected IPs
ss -tulpn | grep -E "169.254.10.10|169.254.10.11"

# Check localdns PID file
cat /run/localdns.pid 2>/dev/null || echo "PID file not found"

# Verify the process from PID file
if [ -f /run/localdns.pid ]; then
  pid=$(cat /run/localdns.pid)
  ps -p "$pid" -o pid,cmd
fi
```

**Expected Output:**
- Should see coredns process running
- Should see listeners on 169.254.10.10:53 and 169.254.10.11:53
- PID in /run/localdns.pid should match running process

### 3. Check Systemd Logs
```bash
# Full localdns service journal
journalctl -u localdns --no-pager -n 500

# aks-node-controller logs (this creates the systemd unit)
journalctl -u aks-node-controller --no-pager -n 200

# Check for localdns-related messages
journalctl --no-pager | grep -i localdns | tail -100

# Check for timeout errors
journalctl --no-pager | grep -i "timeout\|timed out" | tail -50
```

**Look for:**
- Unit creation logs from aks-node-controller
- Start/restart attempts and failures
- Timeout messages
- Dependency cycle warnings

### 4. Check Localdns Configuration Files
```bash
# Check if localdns corefile exists
ls -la /opt/azure/containers/localdns/
cat /opt/azure/containers/localdns/updated.localdns.corefile 2>/dev/null || echo "Corefile not found"

# Check if hosts plugin file exists
cat /etc/localdns/hosts 2>/dev/null || echo "Hosts file not found"

# Check localdns binary
ls -la /opt/azure/containers/localdns/binary/coredns
/opt/azure/containers/localdns/binary/coredns --version
```

### 5. Check Network Configuration
```bash
# Check dummy interface for localdns IPs
ip addr show localdns 2>/dev/null || echo "localdns interface not found"

# Check resolv.conf
cat /etc/resolv.conf

# Check if localdns IPs are configured
ip addr | grep -E "169.254.10.10|169.254.10.11"

# Check iptables rules for localdns
iptables -t raw -L -n -v | grep -A5 "localdns"
```

### 6. Manual Test: Start Localdns
```bash
# Try to manually start localdns service
sudo systemctl daemon-reload
sudo systemctl start localdns

# Check status immediately
systemctl status localdns --no-pager

# If start works, try restart
sudo systemctl restart localdns

# Monitor the restart with timeout
timeout 35 systemctl restart localdns
echo "Restart exit code: $?"
```

### 7. Check aks-node-controller Configuration
```bash
# aks-node-controller config (scriptless uses this)
cat /var/lib/aks-node-controller/aks-node-config.json 2>/dev/null

# Check if localdns profile is enabled
cat /var/lib/aks-node-controller/aks-node-config.json | jq '.localDnsProfile' 2>/dev/null

# aks-node-controller status
systemctl status aks-node-controller --no-pager -l
```

---

## Debugging from Local Machine (Using Scenario Logs)

### Analyze Cluster Provision Log
```bash
cd e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless

# Look for localdns setup steps
grep -i "localdns" cluster-provision.log | head -50

# Check for timeout
grep -i "timeout\|timed out" cluster-provision.log

# Check CSE output
grep -i "localdns" cluster-provision-cse-output.log
```

### Analyze aks-node-controller Log
```bash
# Check aks-node-controller log for unit creation
cat aks-node-controller.log | grep -i "localdns\|systemd"

# Look for provision failure
grep -i "provision failed\|exitCode" aks-node-controller.log
```

### Check Serial Console Output
```bash
# Serial console may have systemd startup messages
grep -i "localdns\|coredns" serial-console-vm-0.log | tail -50
```

---

## Common Issues and Solutions

### Issue 1: Systemd Unit Not Created
**Symptom:** `systemctl cat localdns` returns "No files found"

**Root Cause:** aks-node-controller didn't create the systemd unit file

**Debug:**
```bash
# Check if aks-node-controller attempted to create it
journalctl -u aks-node-controller | grep -i "localdns\|systemd"

# Check aks-node-controller source code for systemd unit generation
```

**Files to Check in AgentBaker:**
- `aks-node-controller/` directory (systemd unit generation code)
- Search for: "localdns.service", "systemd", "WriteSystemdUnit"

### Issue 2: Systemd Unit Exists but Restart Hangs
**Symptom:** `systemctl restart localdns` times out after 30 seconds

**Possible Causes:**
- **Type=oneshot with RemainAfterExit=no** - Causes restart to hang
- **Missing dependencies** - Waiting for services that never start
- **ExecStart command hangs** - Command doesn't properly daemonize
- **Dependency cycle** - Circular dependency with other units

**Debug:**
```bash
# Check Type= directive
systemctl show localdns -p Type

# Check dependencies
systemctl list-dependencies localdns

# Check for dependency cycles
systemctl --no-pager list-dependencies --all localdns | grep -E "circle|cycle"

# Trace the restart operation
strace -f -e trace=execve systemctl restart localdns 2>&1 | tail -100
```

**Expected Configuration:**
```ini
[Unit]
Description=LocalDNS service
After=network-online.target

[Service]
Type=simple
ExecStart=/path/to/localdns/start.sh
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Issue 3: CoreDNS Running but Systemd Doesn't Know About It
**Symptom:** `ps aux | grep coredns` shows process, but `systemctl status localdns` shows inactive

**Root Cause:** CoreDNS started manually (not via systemd), then systemctl tries to start it again

**Debug:**
```bash
# Check who started coredns
ps -eo pid,ppid,cmd | grep coredns

# Check if PID file exists but doesn't match systemd expectations
cat /run/localdns.pid
systemctl show localdns -p MainPID
```

### Issue 4: Localdns.sh Script Issues
**Symptom:** ExecStart command fails or hangs

**Debug:**
```bash
# Find the ExecStart command
systemctl cat localdns | grep ExecStart

# Run it manually to see what happens
# (Copy the ExecStart line and run it)

# Check localdns.sh script
cat /opt/azure/containers/localdns.sh
bash -x /opt/azure/containers/localdns.sh 2>&1 | tail -100
```

---

## Code Locations to Investigate

### In AgentBaker Repository

1. **Scriptless Systemd Unit Generation:**
```bash
# Search for where localdns systemd unit is created in scriptless mode
cd /home/sakwa/go/src/go.goms.io/AgentBaker
find . -name "*.go" -type f -exec grep -l "localdns.*systemd\|systemd.*localdns" {} \;
find aks-node-controller -name "*.go" -type f -exec grep -l "localdns" {} \;
```

2. **Localdns Service Script:**
```bash
# Check the localdns.sh script
cat parts/linux/cloud-init/artifacts/localdns.sh

# Look for systemd interaction
grep -n "systemctl\|systemd" parts/linux/cloud-init/artifacts/localdns.sh
```

3. **Scriptless Provisioning Logic:**
```bash
# Find scriptless provisioning code
find aks-node-controller -name "*.go" | xargs grep -l "LocalDNS\|localdns"
```

---

## Expected vs Actual Behavior

### Expected Behavior (Scriptless Mode)
1. aks-node-controller reads AKSNodeConfig with `EnableLocalDNS: true`
2. Creates systemd unit file for localdns service
3. Starts localdns service via systemd
4. Service runs CoreDNS with proper configuration
5. `systemctl restart localdns` should complete within seconds

### Actual Behavior
1. ✅ CoreDNS starts successfully (PID 23289)
2. ✅ Network configuration completes
3. ✅ Listening on 169.254.10.10:53 and 169.254.10.11:53
4. ❌ `systemctl restart localdns` times out after 30 seconds
5. ❌ Provisioning fails with exit 124

**Gap:** Something between steps 4 and 5 is broken

---

## Next Steps

### 1. SSH Investigation (Priority 1)
SSH into the VM and run all the debugging commands above to understand:
- Does the systemd unit exist?
- What's the unit configuration?
- What happens when you manually restart?
- Are there dependency issues?

### 2. Compare with Working Test (Priority 2)
Compare the scriptless test with the working standard test:
```bash
# Check if systemd unit differs
ssh standard-test-vm "systemctl cat localdns" > /tmp/standard-unit.txt
ssh scriptless-test-vm "systemctl cat localdns" > /tmp/scriptless-unit.txt
diff -u /tmp/standard-unit.txt /tmp/scriptless-unit.txt
```

### 3. Code Review (Priority 3)
Review how aks-node-controller creates the systemd unit:
- Is it using Type=oneshot? (should be Type=simple or Type=forking)
- Are dependencies correct?
- Is the ExecStart command correct?

---

## Quick Diagnostic Script

Run this on the VM for a quick overview:

```bash
#!/bin/bash
echo "=== LocalDNS Scriptless Diagnostic ==="
echo ""
echo "1. Systemd Unit Status:"
systemctl status localdns --no-pager || echo "Service not found"
echo ""
echo "2. Unit File:"
systemctl cat localdns 2>/dev/null || echo "Unit file not found"
echo ""
echo "3. CoreDNS Process:"
ps aux | grep -E "coredns|localdns" | grep -v grep
echo ""
echo "4. Network Listeners:"
ss -tulpn | grep -E "169.254.10.10|169.254.10.11"
echo ""
echo "5. PID File:"
cat /run/localdns.pid 2>/dev/null || echo "PID file not found"
echo ""
echo "6. Recent Logs:"
journalctl -u localdns --no-pager -n 20 2>/dev/null || echo "No journal entries"
echo ""
echo "7. aks-node-controller status:"
systemctl status aks-node-controller --no-pager | head -20
echo ""
echo "8. Localdns Interface:"
ip addr show localdns 2>/dev/null || echo "Interface not found"

---

## KEY FINDING FROM LOG ANALYSIS

### The Problem

The log shows:
```
Mar 06 03:16:02 localdns.sh: Starting localdns: systemd-cat --identifier=localdns-coredns ...
Mar 06 03:16:02 localdns-coredns: CoreDNS-1.13.2
Mar 06 03:16:03 localdns.sh: Localdns PID is 23289.
Mar 06 03:16:03 localdns.sh: Waiting for localdns to start and be able to serve traffic.
+ sleep 5
+ for i in $(seq 1 $retries)
+ timeout 30 systemctl daemon-reload
+ timeout 30 systemctl restart localdns
[TIMEOUT - never completes]
```

### Root Cause Analysis

**CoreDNS is started MANUALLY** (not via systemd):
- Started with: `systemd-cat --identifier=localdns-coredns -- /opt/azure/containers/localdns/binary/coredns ...`
- This runs coredns directly and pipes output to journald
- **Systemd doesn't know about this process**

Then the script tries to:
1. `systemctl daemon-reload` - looks for systemd unit files
2. `systemctl restart localdns` - tries to restart a service that systemd doesn't manage

**The issue:** In scriptless mode, there's likely **NO localdns.service systemd unit file**, or it exists but doesn't properly track the manually-started coredns process.

### Why It Times Out

`systemctl restart localdns` hangs for 30 seconds because:
1. **Option A:** Unit file doesn't exist → systemd waits/searches/fails slowly
2. **Option B:** Unit file exists but is misconfigured:
   - `Type=oneshot` without proper configuration → restart hangs
   - ExecStart doesn't match the already-running process
   - Systemd tries to start a second instance that conflicts with PID 23289

### The Fix

**Immediate workaround:**  
Don't call `systemctl restart localdns` if localdns was started manually. Instead, just verify the process is running and healthy.

**Proper fix for scriptless mode:**
1. Create a proper systemd unit file: `/etc/systemd/system/localdns.service`
2. Use `Type=simple` or `Type=exec`
3. Let systemd manage the process from the start (don't use systemd-cat directly)
4. OR: Skip systemctl restart entirely if process is already running successfully

### Files to Check

1. **Does systemd unit exist in scriptless mode?**
   ```bash
   ls -la /etc/systemd/system/localdns.service
   systemctl cat localdns
   ```

2. **Check how localdns is started in scriptless vs standard:**
   - Standard CSE: Uses systemd unit properly
   - Scriptless: Starts coredns manually then tries to use systemctl (mismatch!)

3. **Code locations:**
   - `parts/linux/cloud-init/artifacts/cse_helpers.sh:466-485` - systemctl_restart function
   - `parts/linux/cloud-init/artifacts/cse_config.sh` - localdns setup logic
   - `aks-node-controller/` - scriptless systemd unit generation

---

## Quick SSH Diagnostic Commands

```bash
# Get VMSS instance IP
az vmss list-instance-connection-info \
  --name lmah-2026-03-06-ubuntu2204localdnshostspluginscriptless \
  --resource-group abe2e-westus2 \
  --output table

# SSH with key from logs
ssh -i e2e/scenario-logs/Test_Ubuntu2204_LocalDNSHostsPlugin_Scriptless/sshkey \
  azureuser@<IP>

# Once on VM, run:
systemctl cat localdns                    # Does unit exist?
systemctl show localdns -p Type           # What Type= is it?
ps aux | grep coredns                     # Is process running?
systemctl status localdns                 # What does systemd think?
journalctl -u localdns -n 100             # Recent logs
```

---

## Recommended Fix

### Option 1: Skip systemctl restart if process is healthy (Quick Fix)

In scriptless mode, after starting coredns manually:
```bash
# Check if coredns is running and healthy
if ps aux | grep -v grep | grep "coredns.*localdns" && \
   dig @169.254.10.10 health-check.localdns.local +short; then
    echo "LocalDNS is running and healthy, skipping systemctl restart"
    exit 0
fi
```

### Option 2: Proper systemd integration (Correct Fix)

1. Generate systemd unit file in scriptless mode:
```ini
[Unit]
Description=LocalDNS CoreDNS Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/opt/azure/containers/localdns/binary/coredns \
  -conf /opt/azure/containers/localdns/updated.localdns.corefile \
  -pidfile /run/localdns.pid
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=localdns-coredns

[Install]
WantedBy=multi-user.target
```

2. Start via systemd from the beginning:
```bash
systemctl daemon-reload
systemctl enable --now localdns
```

---
