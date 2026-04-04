# On-Prem vPro: RAS MPS Hostname Empty — Recovery Steps

## Problem
After uninstall + reinstall of edge node agents, `rpc amtinfo` shows:
```
RAS MPS Hostname        :
RAS Remote Status       : not connected
```

## Root Causes
1. **LMS masked/removed** — uninstall script stopped/masked `lms.service` or removed the `lms` package
2. **AppArmor blocks pm-agent** — profile missing `network unix stream` rule, so pm-agent can't run `systemctl` to fix LMS
3. **Zombie rpc holds MEI** — a stuck `rpc` process holds `/dev/mei0`, preventing LMS from connecting to HECI

## Recovery Steps

### Step 1: Kill any stuck rpc process
```bash
sudo fuser -v /dev/mei0
# If rpc is listed:
sudo kill -9 <rpc_pid>
```

### Step 2: Unmask and start LMS
```bash
sudo systemctl unmask lms.service
sudo systemctl enable lms.service
sudo systemctl daemon-reload
sudo systemctl start lms.service
```

### Step 3: Verify LMS is listening
```bash
ss -tlnp | grep 16992
```
Expected: `LISTEN 0 5 0.0.0.0:16992` — if not, check `journalctl -u lms.service` for HECI errors and repeat Step 1.

### Step 4: Fix AppArmor profile (one-time)
```bash
# Check if fix needed
grep "network unix stream" /etc/apparmor.d/opt.edge-node.bin.pm-agent || echo "NEEDS FIX"

# Add unix socket permissions
sudo sed -i '/network netlink raw,/a\
\n  # Allow unix sockets for systemctl D-Bus communication\n  network unix stream,\n  network unix dgram,' \
  /etc/apparmor.d/opt.edge-node.bin.pm-agent

sudo apparmor_parser -r /etc/apparmor.d/opt.edge-node.bin.pm-agent
```

### Step 5: Deactivate AMT
```bash
sudo /usr/bin/rpc deactivate -local
```

### Step 6: Restart dm-manager (on orchestrator)
```bash
kubectl -n orch-infra rollout restart deploy/dm-manager
kubectl -n orch-infra rollout status deploy/dm-manager --timeout=60s
```

### Step 7: Restart pm-agent
```bash
sudo systemctl restart platform-manageability-agent
```

### Step 8: Wait and verify (~2 minutes)
```bash
sudo /usr/bin/rpc amtinfo | grep -i "ras\|control"
```
Expected:
```
Control Mode            : activated in client control mode
RAS Network             : outside enterprise
RAS Remote Status       : connected
RAS Trigger             : periodic
RAS MPS Hostname        : mps.<domain>
```

## Prevention

Use the updated `uninstall_new.sh` which:
- Does NOT stop/mask/remove `lms.service` or the `lms` package
- Runs `ensure_lms_healthy()` at the end to unmask/enable/start LMS as a safety net
