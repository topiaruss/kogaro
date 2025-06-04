# Contributing to Kogaro

Thank you for your interest in contributing to Kogaro! This document provides guidelines for contributing to the project.

## Development Setup

### Prerequisites

- Go 1.21 or later
- Docker (for containerized builds)
- kubectl and access to a Kubernetes cluster
- make

### Setting up the Development Environment

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/yourusername/kogaro.git
   cd kogaro
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build the project:
   ```bash
   make build
   ```

## Making Changes

### Code Style

- Follow standard Go conventions and formatting
- Use `gofmt` to format your code
- Run `go vet` to check for common mistakes
- Follow the existing code structure and patterns

### Testing

- Write tests for new functionality
- Ensure all tests pass: `make test`
- Test your changes against a real Kubernetes cluster when possible

### Commit Guidelines

- Use clear, descriptive commit messages
- Reference issue numbers when applicable
- Keep commits focused on a single change

## Submitting Changes

1. Create a feature branch: `git checkout -b feature/your-feature-name`
2. Make your changes and commit them
3. Push to your fork: `git push origin feature/your-feature-name`
4. Create a Pull Request on GitHub

### Pull Request Requirements

- Include a clear description of the changes
- Reference any related issues
- Ensure all tests pass
- Update documentation if necessary
- Follow the existing code style

## Adding New Validators

To add a new validation type:

1. Add the validation logic to `internal/validators/reference_validator.go`
2. Update the `ValidateCluster()` method to include your new validator
3. Add appropriate error types and metrics
4. Include tests for your validation logic
5. Update the README with examples of issues your validator catches

## Reporting Issues

When reporting issues, please include:

- Kubernetes version
- Kogaro version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs or error messages

## Questions?

Feel free to open an issue for questions about contributing or join the discussions in the GitHub repository.