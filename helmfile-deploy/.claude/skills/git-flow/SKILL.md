---
name: git-flow
description: Manage git workflow with best practices for this project
---

You are managing git workflow. Follow these steps:

1. **Branch strategy**:
   - Create feature branches from main
   - Use naming: `EMF_<FEATURE>_<YYYY>_<MM>`
   - Keep branches focused and short-lived
   - Sync with main regularly

2. **Commit practices**:
   - Write clear, descriptive commit messages
   - Use conventional commits format
   - Make atomic commits (one logical change per commit)
   - Sign commits if required
   - Run linters before committing

3. **Pull request workflow**:
   - Create PR with descriptive title and body
   - Reference related issues
   - Add test plan section
   - Request appropriate reviewers
   - Address review comments promptly

4. **Code review checklist**:
   - Verify tests pass
   - Check for security issues
   - Review YAML syntax and validation
   - Ensure documentation is updated
   - Validate Helm charts and templates

5. **Merge and cleanup**:
   - Squash commits if needed
   - Update CHANGELOG if applicable
   - Delete branch after merge
   - Tag releases appropriately
   - Notify relevant teams
