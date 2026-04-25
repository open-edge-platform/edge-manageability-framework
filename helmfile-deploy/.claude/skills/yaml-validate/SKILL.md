---
name: yaml-validate
description: Validate YAML files for syntax and Kubernetes schema compliance
---

You are validating YAML files. Follow these steps:

1. **Syntax validation**:
   - Check YAML syntax with `yamllint` or `yq`
   - Look for indentation issues
   - Check for duplicate keys
   - Validate quotes and escape characters

2. **Kubernetes schema validation**:
   - Use `kubectl apply --dry-run=client -f <file>`
   - Check API version compatibility
   - Verify required fields are present
   - Validate resource names (DNS-1123 compliance)

3. **Security checks**:
   - Look for hardcoded secrets or sensitive data
   - Check for privileged containers
   - Validate resource limits are set
   - Review security contexts

4. **Best practices**:
   - Ensure labels and annotations are meaningful
   - Check namespace is specified where needed
   - Validate selector matching
   - Review probe configurations

5. **Report findings**:
   - List all syntax errors
   - Highlight security concerns
   - Suggest improvements
   - Provide corrected YAML if needed
