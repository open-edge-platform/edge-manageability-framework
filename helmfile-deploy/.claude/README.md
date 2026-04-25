# Claude Code Configuration for Edge Manageability Framework

This directory contains the Claude Code configuration for the Edge Manageability Framework helmfile-deploy project.

## 📁 Directory Structure

```
.claude/
├── README.md                 # This file
├── settings.json            # Project-wide Claude settings
├── settings.local.json      # Local user overrides
└── skills/                  # 20 custom skills for EMF operations
    ├── README.md
    ├── cert-manager.md
    ├── ci-cd-pipeline.md
    ├── cost-analysis.md
    ├── dashboard.md
    ├── disaster-recovery.md
    ├── git-flow.md
    ├── health-check.md
    ├── helm-deploy.md
    ├── helmfile-sync.md
    ├── helm-rollback.md
    ├── istio-check.md
    ├── k8s-debug.md
    ├── monitoring-setup.md
    ├── network-policy.md
    ├── performance-check.md
    ├── pod-security.md
    ├── rbac-audit.md
    ├── secret-audit.md
    ├── storage-manage.md
    └── yaml-validate.md
```

## 📚 Documentation Files

### Root Level
- **CLAUDE.md** - Comprehensive project documentation for humans and AI
- **agent.md** - Specific guidelines for AI agents working on this codebase

### Referenced in settings.json
The `projectDocs` field automatically loads these documents into Claude's context:
- `../CLAUDE.md` - Main project overview
- `../agent.md` - Agent-specific guidelines
- `../pre-orch/README.md` - Pre-orchestrator documentation

## ⚙️ Settings Configuration

### settings.json
Project-wide settings including:
- **Description**: Project identifier
- **Project Docs**: Auto-loaded documentation
- **Permissions**: 
  - ✅ **Allowed**: Read operations, kubectl queries, git status, helmfile diff/list
  - ❌ **Denied**: Destructive operations (force push, hard reset, delete namespaces)
- **Environment**: KUBECONFIG path

### settings.local.json
User-specific overrides (gitignored, for local customization)

## 🛠️ Available Skills

Use any skill with `/` prefix in Claude Code:

### Deployment & Operations
- `/helm-deploy` - Deploy Helm charts with validation
- `/helm-rollback` - Safely rollback releases
- `/helmfile-sync` - Synchronize Helmfile deployments

### Debugging
- `/k8s-debug` - Debug pods and services
- `/health-check` - Cluster health check
- `/performance-check` - Performance analysis

### Security
- `/secret-audit` - Audit secrets management
- `/pod-security` - Pod security standards
- `/rbac-audit` - RBAC permissions audit
- `/network-policy` - Network policies

### Infrastructure
- `/istio-check` - Istio mesh validation
- `/cert-manager` - TLS certificate management
- `/storage-manage` - Storage and volumes
- `/monitoring-setup` - Monitoring configuration

### Validation
- `/yaml-validate` - YAML validation
- `/disaster-recovery` - DR procedures
- `/cost-analysis` - Cost optimization

### Development
- `/git-flow` - Git workflow management
- `/ci-cd-pipeline` - CI/CD optimization

### Visualization
- `/dashboard` - Status dashboard

## 🔒 Security Features

### Permitted Operations (Auto-approved)
- Reading YAML, Helm, and config files
- Viewing Kubernetes resources (get, describe, logs)
- Git status and history queries
- Helmfile diff and template operations
- File system navigation (ls, find, grep)

### Denied Operations (Require approval)
- Destructive git operations (force push, hard reset)
- Kubernetes deletions (namespace, resources)
- Helmfile destroy operations
- System file deletion (rm -rf)

## 🚀 Usage

### Starting a Session
1. Claude automatically loads `CLAUDE.md` and `agent.md` for context
2. Skills are available via `/skill-name` commands
3. Permissions are enforced based on settings.json

### Working with Helmfile
```bash
# These are auto-approved:
/helmfile-sync                    # Use the sync skill
helmfile -e onprem-eim diff       # Preview changes
helmfile -e onprem-eim list       # List releases

# These require approval:
helmfile -e onprem-eim sync       # Apply changes
helmfile -e onprem-eim destroy    # Delete releases
```

### Working with Kubernetes
```bash
# Auto-approved:
kubectl get pods -A
kubectl describe pod <name>
kubectl logs <pod>

# Require approval:
kubectl delete <resource>
kubectl apply -f <file>
```

## 📝 Customization

### Adding New Skills
1. Create `skills/your-skill.md` with frontmatter:
```markdown
---
name: your-skill
description: What it does
---

Instructions here...
```

2. Add to skills/README.md for documentation

### Modifying Permissions
Edit `.claude/settings.json`:
- Add patterns to `permissions.allow` for auto-approval
- Add patterns to `permissions.deny` to block operations

### Local Overrides
Use `.claude/settings.local.json` (gitignored) for:
- Personal environment variables
- Additional permissions
- Custom hooks

## 🔗 Integration Points

### With CLAUDE.md
- Provides architectural overview
- Explains directory structure
- Documents common patterns

### With agent.md
- Defines code modification rules
- Provides troubleshooting guides
- Lists security considerations

### With Skills
- Each skill is self-contained
- Skills reference project patterns
- Follow consistent format

## 🧪 Testing the Configuration

```bash
# Verify settings are loaded
cat .claude/settings.json

# Check skills are available
ls .claude/skills/

# Test read permissions
kubectl get nodes

# Verify git operations
git status
```

## 📊 Maintenance

### Regular Updates
- Review and update CLAUDE.md when architecture changes
- Add new skills as workflows emerge
- Adjust permissions based on common operations

### Version Control
- `.claude/settings.json` - Committed (shared config)
- `.claude/settings.local.json` - Gitignored (personal config)
- `.claude/skills/*.md` - Committed (shared skills)

## 🆘 Troubleshooting

### Skills Not Showing
- Check file naming: `skill-name.md` in `.claude/skills/`
- Verify frontmatter is present
- Restart Claude Code session

### Permission Denied
- Check settings.json `allow` patterns
- Add specific command to allow list
- Use settings.local.json for personal overrides

### Documentation Not Loading
- Verify `projectDocs` paths in settings.json
- Check files exist: `CLAUDE.md`, `agent.md`
- Use relative paths from `.claude/` directory

## 📖 References

- [Claude Code Documentation](https://docs.anthropic.com/claude/docs)
- [Helmfile Documentation](https://helmfile.readthedocs.io/)
- Project CLAUDE.md: `../CLAUDE.md`
- Project agent.md: `../agent.md`
