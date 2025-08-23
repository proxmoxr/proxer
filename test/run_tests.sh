#!/bin/bash
# Test runner script for Proxer

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Change to project root
cd "$(dirname "$0")/.."

print_status "Running Proxer test suite..."

# Check if Go is available
if ! command -v go &> /dev/null; then
    print_error "Go is not installed or not in PATH"
    exit 1
fi

# Create test directories if they don't exist
mkdir -p test/coverage
mkdir -p test/reports

# Run different test suites based on arguments
TEST_TYPE="${1:-all}"
VERBOSE="${2:-false}"

# Set Go test flags
GO_TEST_FLAGS="-race"
if [ "$VERBOSE" = "true" ] || [ "$VERBOSE" = "-v" ]; then
    GO_TEST_FLAGS="$GO_TEST_FLAGS -v"
fi

run_unit_tests() {
    print_status "Running unit tests..."
    
    # Test core models
    go test $GO_TEST_FLAGS ./internal/models || {
        print_error "Unit tests for models failed"
        return 1
    }
    
    # Test configuration loading
    go test $GO_TEST_FLAGS ./pkg/config || {
        print_error "Unit tests for config failed"
        return 1
    }
    
    # Test builder (if it has tests)
    if [ -f pkg/builder/builder_test.go ]; then
        go test $GO_TEST_FLAGS ./pkg/builder || {
            print_error "Unit tests for builder failed"
            return 1
        }
    fi
    
    print_success "Unit tests passed"
}

run_integration_tests() {
    print_status "Running integration tests..."
    
    # Test CLI commands
    go test $GO_TEST_FLAGS ./internal/cmd || {
        print_error "Integration tests for CLI commands failed"
        return 1
    }
    
    print_success "Integration tests passed"
}

run_e2e_tests() {
    print_status "Running end-to-end tests..."
    
    # Check if we have the e2e test directory
    if [ ! -d "test/e2e" ]; then
        print_warning "End-to-end tests directory not found, skipping"
        return 0
    fi
    
    # Run e2e tests
    go test $GO_TEST_FLAGS ./test/e2e || {
        print_error "End-to-end tests failed"
        return 1
    }
    
    print_success "End-to-end tests passed"
}

run_coverage_tests() {
    print_status "Running tests with coverage analysis..."
    
    # Run tests with coverage
    go test -race -coverprofile=test/coverage/coverage.txt -covermode=atomic ./... || {
        print_error "Coverage tests failed"
        return 1
    }
    
    # Generate coverage report
    go tool cover -html=test/coverage/coverage.txt -o test/coverage/coverage.html
    
    # Display coverage summary
    COVERAGE=$(go tool cover -func=test/coverage/coverage.txt | grep total | awk '{print $3}')
    print_success "Test coverage: $COVERAGE"
    
    # Check if coverage meets minimum threshold (80%)
    COVERAGE_NUM=$(echo $COVERAGE | sed 's/%//')
    if [ "${COVERAGE_NUM%.*}" -lt 80 ]; then
        print_warning "Test coverage is below 80% threshold"
    fi
}

run_benchmark_tests() {
    print_status "Running benchmark tests..."
    
    # Run benchmarks if any exist
    if go test -list . | grep -q "Benchmark"; then
        go test -bench=. -benchmem ./... > test/reports/benchmarks.txt || {
            print_warning "Some benchmarks failed"
        }
        print_success "Benchmarks completed (see test/reports/benchmarks.txt)"
    else
        print_warning "No benchmark tests found"
    fi
}

run_lint_checks() {
    print_status "Running lint checks..."
    
    # Check if golangci-lint is available
    if command -v golangci-lint &> /dev/null; then
        golangci-lint run --timeout=5m || {
            print_warning "Lint checks found issues"
        }
    else
        print_warning "golangci-lint not found, skipping lint checks"
        print_warning "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    fi
    
    # Run go fmt check
    if [ -n "$(gofmt -l .)" ]; then
        print_warning "Code formatting issues found. Run 'go fmt ./...' to fix"
        gofmt -l .
    else
        print_success "Code formatting is correct"
    fi
    
    # Run go vet
    go vet ./... || {
        print_error "go vet found issues"
        return 1
    }
    
    print_success "Static analysis passed"
}

run_security_tests() {
    print_status "Running security tests..."
    
    # Check if gosec is available
    if command -v gosec &> /dev/null; then
        gosec ./... || {
            print_warning "Security scan found issues"
        }
    else
        print_warning "gosec not found, skipping security scan"
        print_warning "Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"
    fi
    
    # Check if govulncheck is available
    if command -v govulncheck &> /dev/null; then
        govulncheck ./... || {
            print_warning "Vulnerability scan found issues"
        }
    else
        print_warning "govulncheck not found, skipping vulnerability scan"
        print_warning "Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"
    fi
}

# Main execution based on test type
case "$TEST_TYPE" in
    "unit")
        run_unit_tests
        ;;
    "integration")
        run_integration_tests
        ;;
    "e2e")
        run_e2e_tests
        ;;
    "coverage")
        run_coverage_tests
        ;;
    "bench"|"benchmark")
        run_benchmark_tests
        ;;
    "lint")
        run_lint_checks
        ;;
    "security")
        run_security_tests
        ;;
    "ci")
        # CI mode - run essential tests
        print_status "Running CI test suite..."
        run_unit_tests && \
        run_integration_tests && \
        run_e2e_tests && \
        run_lint_checks && \
        run_coverage_tests
        ;;
    "all")
        # Full test suite
        print_status "Running full test suite..."
        run_unit_tests && \
        run_integration_tests && \
        run_e2e_tests && \
        run_lint_checks && \
        run_coverage_tests && \
        run_benchmark_tests && \
        run_security_tests
        ;;
    *)
        echo "Usage: $0 [unit|integration|e2e|coverage|bench|lint|security|ci|all] [-v]"
        echo ""
        echo "Test types:"
        echo "  unit        - Run unit tests only"
        echo "  integration - Run integration tests only"
        echo "  e2e         - Run end-to-end tests only"
        echo "  coverage    - Run tests with coverage analysis"
        echo "  bench       - Run benchmark tests"
        echo "  lint        - Run lint and static analysis"
        echo "  security    - Run security scans"
        echo "  ci          - Run CI test suite (unit + integration + e2e + lint + coverage)"
        echo "  all         - Run all tests and checks (default)"
        echo ""
        echo "Options:"
        echo "  -v          - Verbose output"
        echo ""
        echo "Examples:"
        echo "  $0                    # Run all tests"
        echo "  $0 unit -v           # Run unit tests with verbose output"
        echo "  $0 ci                # Run CI test suite"
        echo "  $0 coverage          # Run tests with coverage analysis"
        exit 1
        ;;
esac

if [ $? -eq 0 ]; then
    print_success "All requested tests completed successfully!"
else
    print_error "Some tests failed!"
    exit 1
fi