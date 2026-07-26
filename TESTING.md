# Testing OpenDeploy

This guide explains how to run tests, linters, and checks locally for both the backend (Go) and frontend (Vue) parts of OpenDeploy.

## Backend (Go)

### Running Tests
To run the full suite of Go tests with race detection and coverage generation:
```bash
make test
```
To run a shorter, faster subset of tests during development:
```bash
make test-short
```

### Coverage
To view the generated `coverage.out` in your browser:
```bash
make coverage
```

### Linting
We use `golangci-lint` to maintain code quality.
To run the linter:
```bash
make lint
```
*Note: You must have `golangci-lint` installed on your system. If you do not have it, install it using the [official instructions](https://golangci-lint.run/usage/install/).*

To automatically format the code and fix basic issues:
```bash
make fmt
make vet
```

## Frontend (Vue 3)

The frontend is located in the `web/` directory and uses Vite, Vitest, and ESLint.

### Setup
First, ensure you have installed the npm dependencies:
```bash
cd web
npm ci
```

### Running Tests
To run unit tests using Vitest:
```bash
npm run test
```

### Linting & Formatting
To run ESLint and automatically fix linting errors:
```bash
npm run lint
```
To format the code with Prettier:
```bash
npm run format
```

## CI/CD Pipeline
Every Pull Request and push to `main` undergoes a rigorous CI pipeline via GitHub Actions:
1. **Security**: Runs `govulncheck` and `npm audit` to catch vulnerable dependencies.
2. **Backend Linting**: Runs `golangci-lint` with strict rules.
3. **Frontend Linting**: Runs `eslint` and checks for formatting errors.
4. **Backend Tests**: Executes `go test` with race detection.
5. **Frontend Tests**: Executes `vitest`.

If any of these checks fail, the Pull Request cannot be merged.

### Debugging Failures
If a CI job fails, check the GitHub Actions tab. The failure logs are verbose and should pinpoint the exact line of code causing the issue. Local reproduction is always identical to the CI environment using the commands listed above.
