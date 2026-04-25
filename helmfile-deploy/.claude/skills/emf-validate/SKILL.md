---
name: emf-validate
description: Validate helmfile and YAML configuration files
---

# EMF Validate

Validate helmfile and YAML configuration files.

## What this does
- Checks YAML syntax across helmfile configurations
- Validates helmfile template rendering
- Verifies environment configurations
- Checks for common configuration errors

## Usage
Use this skill when you want to:
- Validate changes before committing
- Check for YAML syntax errors
- Ensure helmfile templates render correctly
- Verify environment file consistency

---

Validate EMF configuration by:
1. Run yamllint on key files:
   - `post-orch/helmfile.yaml.gotmpl`
   - `post-orch/environments/*.yaml.gotmpl`
   - `post-orch/values/*.yaml`
2. Test helmfile template rendering with `helmfile -e onprem-eim template`
3. Check for required environment variables in post-orch.env
4. Report any syntax errors or warnings
5. Suggest fixes for any issues found
