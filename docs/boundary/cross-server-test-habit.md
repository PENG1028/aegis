# Cross-Server Test Habit

## What went wrong

During v1.7AC-2 two-node acceptance, repeatedly tested against ports that weren't open (3000, 3100, 80 without sudo) before succeeding with 80 + sudo. Each time started a new test without checking whether the target port was actually reachable from the source server.

## Why

Focus was on testing the Aegis feature (Gateway Link, trace, etc.) while assuming the port was open because the service was running locally on the target. Cross-server connectivity requires BOTH:
- Service listening on the port (local on target)
- Port allowed through cloud security group (between source and target)

Only the first condition was checked.

## Fix

**Before any cross-server test**, run:
```bash
ssh <source> "curl --connect-timeout 3 http://<target>:<port>/"
```

If this doesn't return 2xx/4xx, investigate why before proceeding further. Don't skip this.
