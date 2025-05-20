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

This project strictly follows test-driven development (TDD) principles:

1. Write failing tests first
2. Implement code to make the tests pass
3. Refactor while maintaining passing tests

All code changes **must** be covered by tests, with a minimum of 90% code coverage.

### Coding Standards

- Follow Go best practices and the Go [coding style](https://go.dev/doc/effective_go)
- Run `gofmt` and `golangci-lint` before committing:
  ```bash
  make lint
  ```
- Add comprehensive comments, especially for exported functions, types, and constants
- Each package should have a package-level comment explaining its purpose

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

1. Update the WIP.markdown file to reflect your changes
2. Ensure all tests pass: `make test`
3. Verify code coverage meets the minimum 90% requirement: `make coverage`
4. Run the linter to check for code style issues: `make lint`
5. Update documentation as necessary
6. Submit your PR with a clear description of the changes and any related issues
7. Wait for the CI pipeline to complete and address any issues

## Pull Request Strategy

This project uses a structured PR strategy:

1. **Test-Driven Development (TDD) Required**: Every PR must begin with failing unit tests followed by implementation
2. **Coverage Enforcement**: PRs must maintain 90%+ code coverage
3. **Small, Focused PRs**: Each PR should address a specific logical component or feature
4. **Feature Branches**: Use feature branches named according to the phase and component (e.g., `feature/phase-1-directory-structure`)
5. **WIP Updates**: Each PR should update WIP.markdown to mark completed tasks
6. **Review Friendly**: Keep PRs small enough for effective code review
7. **Dependencies**: Consider task dependencies when planning PRs

## Documentation

- Update README.md with details of major changes
- Document all public APIs, types, and functions using godoc format
- Keep architecture diagrams and design docs in the `/docs` directory up to date
- Include examples where appropriate

## Testing

- Maintain comprehensive test coverage (>90%)
- Write both unit and integration tests
- Follow the example of existing tests when adding new ones
- Place unit tests next to the code being tested
- Use `/test` for integration/e2e tests and fixtures

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.

## Questions?

If you have questions about contributing, please open an issue or reach out to the maintainers.