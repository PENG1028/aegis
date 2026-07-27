# Aegis Simple Example

This directory contains an example workflow for Aegis v0.1.

## Prerequisites

- Go 1.22 or later
- Caddy (optional, for local testing)

## Quick Start

```bash
# Build Aegis
cd ../..
go build -o aegis ./cmd/aegis/

# Initialize
./aegis init

# Create a project
./aegis project create demo --description "Demo project"

# Add a backend service
./aegis service add demo-web \
  --project demo \
  --env prod \
  --upstream http://127.0.0.1:3001 \
  --health http://127.0.0.1:3001/health

# Add a route
./aegis route add demo.localhost --service demo-web

# Dry-run to see generated config
./aegis apply --dry-run

# Apply the configuration
./aegis apply

# Check health
./aegis health --all

# View operation logs
./aegis logs
```

## Testing Maintenance Mode

```bash
# Enable maintenance
./aegis maintenance on demo.localhost --message "Down for maintenance"
./aegis apply --dry-run

# Disable maintenance
./aegis maintenance off demo.localhost
./aegis apply
```

## Testing Route Switch

```bash
# Create a new version of the service
./aegis service add demo-web-v2 \
  --project demo \
  --env preview \
  --upstream http://127.0.0.1:3002

# Switch the route
./aegis route switch demo.localhost --service demo-web-v2
./aegis apply --dry-run
```
