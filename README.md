```markdown
# Ovi

A living infrastructure reconciliation engine. Your cloud is the state.

## What is Ovi?

Ovi watches your cloud infrastructure and keeps it aligned with simple config files. No state files, no drift, no complexity.

Think of it as a friendly guardian for your AWS/GCP resources - it notices when things change, asks before taking action, and helps keep your infrastructure clean.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         User Config                         │
│                      (infrastructure.yaml)                  │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      Ovi Reconciler                         │
│                                                              │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Watcher   │  │   Decider    │  │   Executor   │      │
│  │             │  │              │  │              │      │
│  │ Polls every │→ │  Compares    │→ │   Creates/   │      │
│  │ 30 seconds  │  │ desired vs   │  │   Updates/   │      │
│  │             │  │   actual     │  │   Notifies   │      │
│  └─────────────┘  └──────────────┘  └──────────────┘      │
│         ▲                                    │              │
│         │                                    ▼              │
│  ┌──────┴──────┐                    ┌──────────────┐      │
│  │   Registry  │                    │   Notifier   │      │
│  │             │                    │              │      │
│  │  Tracks IDs │                    │ Slack/Email  │      │
│  │  (not state)│                    │   Webhooks   │      │
│  └─────────────┘                    └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
        ┌──────────────────┴──────────────────┐
        │                                      │
        ▼                                      ▼
┌──────────────┐                      ┌──────────────┐
│              │                      │              │
│   AWS API    │                      │   GCP API    │
│              │                      │              │
│  The actual  │                      │  The actual  │
│    truth     │                      │    truth     │
│              │                      │              │
└──────────────┘                      └──────────────┘
```

## How it works

```yaml
# ovi.yaml
resources:
  servers:
    type: ec2
    count: 5
    region: us-east-1
```

```bash
# Run Ovi
ovi apply

# Ovi responds
Found 3 existing servers
Creating 2 more to reach 5
Done!
```

## Key Features

- **No state files** - Your cloud is the source of truth
- **Friendly notifications** - Asks before deleting anything
- **Living reconciliation** - Continuously ensures desired state
- **Simple config** - Just YAML, no programming needed
- **Pluggable providers** - AWS, GCP, and more

## Installation

```bash
# Download binary (coming soon)
curl -L https://github.com/falsesystems/ovi/releases/latest/download/ovi -o ovi
chmod +x ovi
sudo mv ovi /usr/local/bin/

# Or build from source
git clone https://github.com/falsesystems/ovi
cd ovi
go build -o ovi cmd/ovi/main.go
```

## Quick Start

```bash
# See what's in your cloud
ovi scan

# Apply a simple config
ovi apply -f infrastructure.yaml

# Run as guardian (watches continuously)
ovi guardian
```

## Example Config

```yaml
# infrastructure.yaml
version: v1
region: us-east-1

resources:
  web-servers:
    type: ec2
    count: 3
    size: t3.micro
    tags:
      team: platform
      env: production

  database:
    type: rds
    engine: postgres
    size: db.t3.small

rules:
  - protect-blessed: true
  - notify-on-orphans: true
  - grace-period: 5m
```

## Philosophy

- Infrastructure isn't code, it's data about what should exist
- Your cloud provider knows the truth - we just reconcile with it
- Be friendly, not aggressive - always ask before destroying
- Keep it simple - if it needs explanation, it's too complex

## Development

```bash
# Run tests
go test ./...

# Format code (required)
go fmt ./...

# Run linter (required)
golangci-lint run

# Build
go build ./...
```

## Project Status

Early development. Not yet ready for production use.

## Contributing

We welcome contributions! Please read [STANDARDS.md](STANDARDS.md) for development guidelines.

## License

MIT

---

Part of [False Systems](https://github.com/falsesystems) - Infrastructure tools that make sense.
```
