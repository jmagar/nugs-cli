---
name: release-notes
description: Generate release notes from git commits since last tag
user-invocable: true
disable-model-invocation: true
---

# Release Notes Generator

Generate structured release notes for the next version based on git commit history.

## Workflow

1. **Get commits since last tag**:
   ```bash
   git log $(git describe --tags --abbrev=0)..HEAD --pretty=format:"%h - %s%n%b%n---"
   ```

2. **Ask user for version number** (e.g., v1.2.0)

3. **Parse commits** and group by type:
   - `feat:` → Features
   - `fix:` → Bug Fixes
   - `docs:` → Documentation
   - `refactor:` → Refactoring
   - `test:` → Testing
   - `chore:` → Maintenance
   - `⚠️ BREAKING` or `BREAKING CHANGE:` → Breaking Changes

4. **Get contributors**:
   ```bash
   git shortlog $(git describe --tags --abbrev=0)..HEAD -sn
   ```

5. **Generate release notes** using this template:

```markdown
## [VERSION] - YYYY-MM-DD

### ⚠️ Breaking Changes
- **[short description]**: [detailed explanation]
  - Migration: [how to update]

### ✨ Features
- [description] ([commit hash])

### 🐛 Bug Fixes
- [description] ([commit hash])

### 📚 Documentation
- [description] ([commit hash])

### 🔧 Refactoring
- [description] ([commit hash])

### 🧪 Testing
- [description] ([commit hash])

### 🔨 Maintenance
- [description] ([commit hash])

### 👥 Contributors
- @username (X commits)
```

## Output Guidelines

- **Remove empty sections** (if no commits of that type)
- **Group similar commits** (e.g., multiple bug fixes for same feature)
- **Highlight user-facing changes** (skip internal refactoring unless significant)
- **Include migration guides** for breaking changes
- **Link to documentation** if applicable

## Example Output

```markdown
## [v1.3.0] - 2026-02-06

### ✨ Features
- Add catalog auto-refresh system with configurable schedule (0ffdf11)
- Implement gap detection for missing shows in collection (b44f85e)
- Add artist catalog shortcuts: `nugs <artist_id> full` (893e859)

### 🐛 Bug Fixes
- Harden gap detection and upload verification (0ffdf11)
- Fix cache corruption during concurrent catalog updates (b44f85e)

### 📚 Documentation
- Comprehensive catalog caching documentation in README.md
- Add CLAUDE.md development guide with architecture overview

### 👥 Contributors
- @jmagar (8 commits)
```

## Notes

- After generating, offer to create a GitHub release
- Offer to update CHANGELOG.md if it exists
- Suggest tagging the release: `git tag -a vX.Y.Z -m "Release vX.Y.Z"`
