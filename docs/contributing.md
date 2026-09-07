# Contributing to JoinQuest

Thank you for your interest in contributing to JoinQuest!

We are building infrastructure so indie web developers can **ship multiplayer games** without getting stuck on lobby, auth, and commerce first. If that mission resonates, you are in the right place. Background: **[Product vision](vision.md)**.

**AI agents / maintainers:** start with **[AGENTS.md](../AGENTS.md)** and **[lobby-maintenance.md](lobby-maintenance.md)** before exploring the codebase.

> **Naming:** product = **JoinQuest** (`joinquest.cc`). GitHub repo = [`scruffyprodigy/joinquest`](https://github.com/scruffyprodigy/joinquest). Local folder is often `lobby`. The legacy `playhub` name is fully retired.

## Getting Started

1. **Fork the repository** on GitHub (or work on a branch if you have write access)
2. **Clone** locally
3. **Set up** with `./scripts/setup.sh`
4. **Create a feature branch** for your changes
5. **Make your changes** and test them
6. **Submit a pull request**

## Development Setup

### Quick Setup
```bash
git clone https://github.com/scruffyprodigy/joinquest.git
cd joinquest   # folder may be named lobby locally
./scripts/setup.sh
```

### Manual Setup
See [Development Guide](development.md) for detailed setup instructions.

## Development Workflow

### 1. Create a Feature Branch
```bash
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes
- Write clean, readable code
- Follow existing code style in the surrounding files
- Add tests for new functionality
- Update documentation as needed
- After editing `docs/developer-agent-playbook.md` or `docs/developer-integration-guide.md`, run `./scripts/sync-developer-docs.sh`
- After GraphQL schema changes, run `cd backend && make generate`

### 3. Test Your Changes
```bash
# Pre-ship checks (recommended)
./scripts/ship-joinquest.sh --check

# Or run individually
./scripts/test.sh
./scripts/test-backend.sh
./scripts/test-frontend.sh --e2e
```

### 4. Commit Your Changes
```bash
git add .
git commit -m "Add short description of why"
```

Use clear commit messages focused on the *why* (see recent history on `main`).

### 5. Push and Create Pull Request
```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Code Style

### Backend (Go)
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Run `go vet` for static analysis
- Write tests for all public functions

### Frontend (JavaScript/React)
- Use ESLint configuration provided
- Follow React best practices
- Write tests for components (Vitest)
- Frontend uses JSX (not TypeScript)

### Developer dashboard / MCP changes
When changing developer-facing GraphQL operations, update backend, frontend, MCP, and docs together — see the checklist in [AGENTS.md](../AGENTS.md).

### General
- Use meaningful variable and function names
- Write clear comments for complex logic
- Keep functions small and focused
- Follow the existing project structure

## Testing

### Backend Testing
- Write unit tests for all resolvers
- Test error conditions
- Include benchmark tests for performance-critical code
- Ensure drift detection tests pass

### Frontend Testing
- Write unit tests for components
- Test user interactions
- Include integration tests for user journeys
- Run E2E tests for critical paths

### Test Coverage
- Focus on critical business logic and integration paths
- Don't test implementation details

## Pull Request Guidelines

### Before Submitting
- [ ] `./scripts/ship-joinquest.sh --check` passes (or equivalent test runs)
- [ ] Code follows style guidelines
- [ ] Documentation is updated (including `./scripts/sync-developer-docs.sh` if playbook/guide changed)
- [ ] No sensitive data is committed
- [ ] Commit messages are clear

### Pull Request Template
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] E2E tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] No sensitive data committed
```

## Issue Guidelines

### Bug Reports
When reporting bugs, please include:
- Clear description of the issue
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, browser, etc.)
- Screenshots if applicable

### Feature Requests
When requesting features, please include:
- Clear description of the feature
- Use case and motivation
- Proposed implementation (if you have ideas)
- Any relevant examples

## Code Review Process

### For Contributors
- Address all review comments
- Keep PRs focused and small
- Respond to feedback promptly
- Be open to suggestions

### For Reviewers
- Be constructive and helpful
- Focus on code quality and correctness
- Test the changes locally when possible
- Approve when ready

## Release Process

### Versioning
We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Notes
Include clear release notes for each version:
- New features
- Bug fixes
- Breaking changes
- Migration instructions (if needed)

## Community Guidelines

### Be Respectful
- Use welcoming and inclusive language
- Be respectful of differing viewpoints
- Accept constructive criticism gracefully
- Focus on what's best for the community

### Be Collaborative
- Help others when you can
- Share knowledge and experience
- Be patient with newcomers
- Work together toward common goals

## Getting Help

### Documentation
- Check existing documentation first
- Look for similar issues or PRs
- Read the codebase for examples

### Communication
- Use GitHub issues for bug reports and feature requests
- Use GitHub discussions for questions and ideas
- Be specific and provide context

### Mentorship
- New contributors are welcome
- Ask questions if you're unsure
- We're here to help you succeed

## Recognition

Contributors will be recognized in:
- CONTRIBUTORS.md file
- Release notes
- Project documentation

Thank you for contributing to JoinQuest! 🚀
