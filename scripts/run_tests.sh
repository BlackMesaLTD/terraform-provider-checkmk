#!/usr/bin/env bash
set -euo pipefail

# Run acceptance tests with credentials from container environment
#
# Usage:
#   ./scripts/run_tests.sh                    # Run against default version (2.4)
#   ./scripts/run_tests.sh --version 2.3      # Run against specific version
#   ./scripts/run_tests.sh --all              # Run against all versions
#   ./scripts/run_tests.sh --package ./internal/provider  # Specific package
#
# Containers are started automatically if not running.
# Credentials are read from container environment variables via docker inspect.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Generate docker-compose.yml if missing
if [[ ! -f docker-compose.yml ]]; then
    echo "Generating docker-compose.yml..."
    python3 scripts/generate-compose.py -o docker-compose.yml
fi

# Default configuration
DEFAULT_VERSION="2.4"
TEST_PACKAGE="./..."
VERBOSE=false
RUN_ALL=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version|-v)
            DEFAULT_VERSION="$2"
            shift 2
            ;;
        --all|-a)
            RUN_ALL=true
            shift
            ;;
        --package|-p)
            TEST_PACKAGE="$2"
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version, -v VERSION   Run tests against specific version (2.2, 2.3, 2.4)"
            echo "  --all, -a               Run tests against all versions"
            echo "  --package, -p PACKAGE   Run specific package tests (default: ./...)"
            echo "  --verbose               Show verbose output"
            echo "  --help, -h              Show this help message"
            echo ""
            echo "Containers are started automatically if not running."
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Function to get credentials from container environment via docker inspect
get_credentials() {
    local version=$1

    # Find container by checking running containers
    # Local mode: checkmk-2.2.0p43, checkmk-2.3.0p41, checkmk-2.4.0p17
    # CI mode:    checkmk-ci-2.2.0p43, checkmk-ci-2.3.0p41, checkmk-ci-2.4.0p17
    local container
    container=$(docker ps --filter "health=healthy" --format '{{.Names}}' | grep -E "checkmk(-ci)?-${version}\." | head -1)

    if [[ -z "$container" ]]; then
        return 1
    fi

    # Get credentials from container environment and labels via docker inspect
    local env_vars api_url password site_id
    env_vars=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$container")
    api_url=$(docker inspect --format '{{index .Config.Labels "checkmk.api_url"}}' "$container")

    password=$(echo "$env_vars" | grep '^CMK_AUTOMATION_SECRET=' | cut -d= -f2-)
    site_id=$(echo "$env_vars" | grep '^CMK_SITE_ID=' | cut -d= -f2-)

    if [[ -z "$password" || -z "$api_url" ]]; then
        return 1
    fi

    # Return JSON format
    echo "{\"url\":\"${api_url}\",\"username\":\"automation\",\"password\":\"${password}\",\"container\":\"${container}\",\"site_id\":\"${site_id}\"}"
}

# Function to wait for containers to be ready
wait_for_containers() {
    echo -n "Waiting for containers... "
    local max_wait=120
    local waited=0
    while [[ $waited -lt $max_wait ]]; do
        # Check if any checkmk container is healthy
        local healthy
        healthy=$(docker ps --filter "health=healthy" --format '{{.Names}}' | grep -c checkmk) || healthy=0
        if [[ $healthy -gt 0 ]]; then
            echo "ready ($healthy healthy)"
            return 0
        fi
        sleep 2
        waited=$((waited + 2))
    done
    echo "timeout!"
    return 1
}

# Function to run tests for a specific version
run_tests_for_version() {
    local version=$1

    echo ""
    echo "=========================================="
    echo "Testing against CheckMK ${version}"
    echo "=========================================="

    # Get credentials
    local creds
    creds=$(get_credentials "$version")

    if [[ -z "$creds" || "$creds" == "{}" ]]; then
        echo "ERROR: No credentials found for version ${version}"
        echo "Make sure container is running and healthy"
        return 1
    fi

    # Parse JSON credentials
    local url username password container
    url=$(echo "$creds" | jq -r '.url // empty')
    username=$(echo "$creds" | jq -r '.username // empty')
    password=$(echo "$creds" | jq -r '.password // empty')
    container=$(echo "$creds" | jq -r '.container // empty')

    if [[ -z "$url" || -z "$password" ]]; then
        echo "ERROR: Could not parse credentials"
        echo "Response: $creds"
        return 1
    fi

    # Extract base URL (remove /check_mk/api/1.0 suffix)
    local base_url="${url%/check_mk/api/1.0}"

    echo "Container: ${container}"
    echo "URL: ${base_url}"
    echo "Username: ${username}"
    echo ""

    # Export environment variables for this test run
    export CHECKMK_URL="$base_url"
    export CHECKMK_USERNAME="$username"
    export CHECKMK_PASSWORD="$password"
    export TF_ACC=1

    # Run tests
    # Note: -tags checkmk_all is required for checkmk-api-spec generated types
    local test_args="-tags checkmk_all -timeout 30m"
    if [[ "$VERBOSE" == "true" ]]; then
        test_args="$test_args -v"
    fi

    echo "Running: go test $test_args $TEST_PACKAGE"
    echo ""

    if go test $test_args "$TEST_PACKAGE"; then
        echo ""
        echo "Tests PASSED for CheckMK ${version}"
        return 0
    else
        echo ""
        echo "Tests FAILED for CheckMK ${version}"
        return 1
    fi
}

# Function to login to CheckMK registry using .env credentials
docker_registry_login() {
    local env_file="${PROJECT_ROOT}/.env"

    if [[ ! -f "$env_file" ]]; then
        echo "WARNING: .env file not found - registry login skipped"
        echo "Create .env with CHECKMK_DOCKER_USERNAME and CHECKMK_DOCKER_PASSWORD"
        return 1
    fi

    # Source credentials from .env
    local username password
    username=$(grep -E '^CHECKMK_DOCKER_USERNAME=' "$env_file" | cut -d= -f2-)
    password=$(grep -E '^CHECKMK_DOCKER_PASSWORD=' "$env_file" | cut -d= -f2-)

    if [[ -z "$username" || -z "$password" ]]; then
        echo "WARNING: Registry credentials not found in .env"
        return 1
    fi

    echo "Logging in to registry.checkmk.com..."
    echo "$password" | docker login registry.checkmk.com -u "$username" --password-stdin
}

# Main execution
echo "CheckMK Terraform Provider - Acceptance Tests"
echo "=============================================="

# Start containers if none running (only when docker-compose.yml exists)
if [[ -f docker-compose.yml ]] && ! docker ps --format '{{.Names}}' | grep -q checkmk; then
    # Login to registry before pulling images
    docker_registry_login || echo "Continuing without registry login..."

    echo "Starting containers..."
    docker compose up -d
fi

# Wait for containers to be ready
if ! wait_for_containers; then
    echo ""
    echo "ERROR: No healthy containers found"
    echo "Check container logs: docker compose logs"
    exit 1
fi

# Detect available versions from running containers
echo -n "Detecting available versions... "
AVAILABLE=""
for v in 2.2 2.3 2.4; do
    if get_credentials "$v" >/dev/null 2>&1; then
        AVAILABLE="$AVAILABLE $v"
    fi
done
AVAILABLE=$(echo "$AVAILABLE" | xargs)  # trim whitespace

if [[ -z "$AVAILABLE" ]]; then
    echo "none found"
    echo "ERROR: No healthy CheckMK containers found"
    echo "Check container logs: docker compose logs"
    exit 1
fi
echo "done"

echo "Available versions: $AVAILABLE"

# Track results
declare -A RESULTS

if [[ "$RUN_ALL" == "true" ]]; then
    # Run against all available versions
    for version in $AVAILABLE; do
        if run_tests_for_version "$version"; then
            RESULTS[$version]="PASS"
        else
            RESULTS[$version]="FAIL"
        fi
    done

    # Summary
    echo ""
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    FAILED=0
    for version in "${!RESULTS[@]}"; do
        echo "CheckMK ${version}: ${RESULTS[$version]}"
        [[ "${RESULTS[$version]}" == "FAIL" ]] && FAILED=$((FAILED + 1))
    done

    exit $FAILED
else
    # Run against single version
    if ! run_tests_for_version "$DEFAULT_VERSION"; then
        exit 1
    fi
fi
