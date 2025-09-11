# Contributing to LLM Proxy

Thank you for your interest in contributing to the LLM Proxy project! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone. Please be kind and considerate in all interactions.

## Getting Started

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR-USERNAME/llm-proxy.git
   cd llm-proxy
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/sofatutor/llm-proxy.git
   ```
4. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Environment

1. **Requirements**:
   - Go 1.23 or higher
   - Docker and Docker Compose (for containerized development)
   - Git

2. **Setup**:
   ```bash
   # Install dependencies
   go mod download
   
   # Copy environment configuration and update values as needed
   cp .env.example .env
   
   # Run development server
   make run
   ```

## Development Workflow

### Test-Driven Development

This project **strictly follows** test-driven development (TDD) principles:

1. **Write failing tests first** - Before implementing any feature or fix
2. **Implement minimal code** to make the tests pass
3. **Refactor while maintaining passing tests**

**Coverage Requirements:**
- **Minimum**: 90% code coverage for all packages under `internal/`
- **Enforcement**: CI fails if coverage drops below minimum
- **New Code**: Must maintain or improve overall coverage percentage

**No exceptions**: All code changes **must** be covered by tests.

### Coding Standards

- Follow Go best practices and the [Effective Go](https://go.dev/doc/effective_go) guide
- Use `gofmt` and `golangci-lint` before committing
- Add comprehensive comments for exported functions, types, and constants
- Each package should have a package-level comment explaining its purpose
- Follow the [Code Organization Guide](docs/code-organization.md) for architecture

**Pre-commit Checks:**
```bash
# Format code
make fmt

# Run linter (required - must pass)
make lint

# Run all tests with race detection
make test

# Check coverage (must be ≥ 90%)
make test-coverage
```

### Commit Guidelines

- Use clear, descriptive commit messages
- Reference issue numbers when applicable: `fix: resolve rate limiting issue (#123)`
- Follow a modified [Conventional Commits](https://www.conventionalcommits.org/) format:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation changes
  - `test:` for test additions or modifications
  - `refactor:` for code refactoring
  - `chore:` for maintenance tasks
  - `style:` for formatting, missing semicolons, etc.
  - `perf:` for performance improvements

### Pull Request Process

1. **Follow TDD Workflow**: Write failing tests first, then implement (see [Testing Guide](docs/testing-guide.md))
2. **Run Pre-Push Checks**:
   ```bash
   make fmt           # Format code
   make lint          # Run linter (must pass)
   make test          # Run tests with race detection
   make test-coverage # Verify ≥90% coverage
   ```
3. **Update Documentation**: 
   - Update relevant issue docs in `docs/issues/`
   - Update `PLAN.md` if architecture changes
   - Add/update package documentation for new components
4. **Submit PR** with:
   - Clear description of changes and motivation
   - Reference to related issues
   - Test coverage impact analysis
   - Documentation updates summary
5. **CI Validation**: Ensure all GitHub Actions pass
6. **Code Review**: Address all review feedback before merging

## Pull Request Strategy

### PR Guidelines

- **Test-Driven Development (TDD) Required**: Every PR must start with failing tests
- **Coverage Enforcement**: PRs must maintain or improve the 90%+ coverage target  
- **Focused Changes**: Each PR should address a single logical component or feature
- **Documentation First**: Update documentation before or with implementation
- **Review-Friendly Size**: Keep PRs small enough for effective code review

### Branch Naming

- `feature/description`: New features or enhancements
- `fix/description`: Bug fixes  
- `docs/description`: Documentation updates
- `refactor/description`: Code refactoring without behavior changes

### PR Description Template

```markdown
## Summary
Brief description of changes and motivation

## Changes
- List of specific changes made
- Reference to issue docs/checklist items  
- Related PLAN.md updates

## Testing
- New tests added (TDD requirement)
- Coverage impact: X% → Y%
- Performance implications

## Documentation  
- Updated issue doc: docs/issues/xxx.md
- Other documentation changes

Fixes #issue-number
```

## Documentation

Documentation is a critical part of the project. All changes should include appropriate documentation updates:

### Core Documentation

- **[Architecture Guide](docs/architecture.md)**: System design, async event system, and component interactions
- **[Code Organization](docs/code-organization.md)**: Package structure, layering, and dependency management  
- **[Testing Guide](docs/testing-guide.md)**: TDD workflow, coverage requirements, and testing practices
- **[API Configuration](docs/api-configuration.md)**: Advanced proxy configuration and API provider setup
- **[Security Guide](docs/security.md)**: Production security, secrets management, and best practices

### Development Resources

- **[CLI Reference](docs/cli-reference.md)**: Complete command-line interface documentation
- **[Go Packages](docs/go-packages.md)**: Using LLM Proxy packages in Go applications
- **[Instrumentation](docs/instrumentation.md)**: Event system and observability setup
- **[Coverage Reports](docs/coverage-reports.md)**: Test coverage reporting and CI integration

### Project Tracking

- **[PLAN.md](PLAN.md)**: Current project architecture and implementation roadmap
- **[Issues](docs/issues/)**: Active task tracking and design decisions
- **[Working Agreement](working-agreement.mdc)**: Core development workflow rules

### Documentation Guidelines

- **Update First**: Update documentation before or with code changes
- **Clear Examples**: Include code examples and usage patterns
- **Architecture Changes**: Update PLAN.md and architecture diagrams
- **API Changes**: Update OpenAPI specifications and CLI reference
- **Keep Current**: Remove outdated information and fix broken links

## Code Governance & Quality Assurance

The project maintains high code quality through systematic review processes:

### Codebase Review Process

- **[Full Inventory & Review](docs/tasks/prd-full-inventory-review.md)**: Comprehensive codebase review framework
- **[Review Templates](docs/reviews/)**: Standardized templates for systematic quality assessment
- **Review Scope**: Package-by-package analysis, documentation alignment, security audit
- **Quality Gates**: 90%+ coverage, clean lints, architectural compliance
- **Non-blocking**: Reviews don't halt development; used for continuous improvement

### Quality Standards

- **Architecture Alignment**: Regular verification against `docs/architecture.md`
- **Security Compliance**: Access control, secret scanning, dependency audits
- **Performance Monitoring**: Latency targets, memory efficiency, scalability assessment
- **Documentation Currency**: Alignment between code and documentation

## Testing

Comprehensive testing is mandatory. See the [Testing Guide](docs/testing-guide.md) for detailed information.

### Testing Requirements

- **TDD Mandatory**: Write failing tests before implementation
- **90% Coverage**: Minimum coverage requirement for all `internal/` packages
- **Race Detection**: All tests must pass with `-race` flag
- **Integration Tests**: Test package interactions and external integrations
- **Performance Tests**: Benchmark critical paths and memory usage

### Running Tests

```bash
# Quick test run (unit tests only)
make test

# Full test suite with coverage
make test-coverage

# Integration tests  
make integration-test

# View coverage report
make test-coverage-html
```

### Coverage Reports

Code coverage reports are automatically generated and made available in multiple ways:

**Local Development:**
```bash
# Generate and view HTML coverage report
make test-coverage-html
# This opens the report in your browser
```

**CI/CD Artifacts:**
- HTML coverage reports are uploaded as build artifacts for each PR and push to main
- Navigate to the Actions tab → Select a workflow run → Download the "coverage-report" artifact
- Extract and open `coverage.html` in your browser

**GitHub Pages (Live):**
- Coverage reports for the main branch are automatically deployed to GitHub Pages
- Access the live report at: `https://[your-org].github.io/llm-proxy/`
- Updated automatically on each push to main

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

## Questions?

If you have questions about contributing, please open an issue or reach out to the maintainers.