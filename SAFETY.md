# Elava Safety Principles

## 🔒 Core Safety Rules

### 1. **NEVER DELETE WITHOUT CONSENT**
Elava will **NEVER** automatically delete resources. Period.
- We only **detect** and **report** untracked resources
- We only **recommend** cleanup actions
- Actual deletion must be done by humans through AWS Console or IaC tools

### 2. **READ-ONLY BY DEFAULT**
Elava operates in read-only mode for discovery:
- ✅ List resources
- ✅ Read tags
- ✅ Analyze patterns
- ❌ Delete resources
- ❌ Modify infrastructure
- ❌ Stop services

### 3. **EXPLICIT ACTIONS ONLY**
Any write operations require explicit user action:
- Tagging requires `--confirm` flag
- Cleanup scripts are generated but not executed
- All actions are logged and auditable

### 4. **SAFE RECOMMENDATIONS**
When suggesting cleanup, we:
- Show exactly what would be affected
- Provide rollback information
- Never touch "blessed" or critical resources
- Respect "do-not-delete" tags

## Example Safe Workflow

```bash
# 1. Scan and detect (READ-ONLY)
ovi scan --region us-east-1

# 2. Generate cleanup script (NOT EXECUTED)
ovi cleanup --dry-run > cleanup_script.sh

# 3. Human reviews the script
cat cleanup_script.sh

# 4. Human manually executes IF they agree
# bash cleanup_script.sh  # <-- Human decision required
```

## Protected Resources

These resources are ALWAYS protected:
- Resources tagged with `elava:blessed=true`
- Resources tagged with `do-not-delete=true`
- Resources tagged with `production=true`
- NAT Gateways (expensive to recreate)
- RDS instances with backups disabled
- Resources modified in last 24 hours

## Audit Trail

All Elava operations are logged:
- WHO ran the scan
- WHEN it was run
- WHAT was detected
- WHAT was recommended
- NO automatic actions taken

## Philosophy

> "Elava is your helpful assistant that points out problems.
> It never touches your infrastructure without permission.
> Think of it as a friendly auditor, not an enforcer."

---

**Remember**: Elava helps you FIND waste. YOU decide what to do about it.