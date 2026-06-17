# Lifecycle Testing Guide

This directory contains lifecycle tests for all Terraform resources. Lifecycle testing verifies the complete CRUD (Create, Read, Update, Delete) + Import workflow.

## Why Lifecycle Testing?

Acceptance tests (`_test.go` files) run the lifecycle automatically, but manual lifecycle testing:
- Helps debug issues interactively
- Verifies real-world usage patterns
- Tests import functionality thoroughly
- Confirms state management works correctly

## Directory Structure

```
testing/lifecycle/
├── README.md                 # This file
├── run_lifecycle_test.sh     # Generic lifecycle test runner
├── folder/
│   ├── main.tf              # Initial configuration
│   ├── main_updated.tf      # Updated configuration
│   └── run_test.sh          # Resource-specific test
├── host_group/
│   └── ...
└── <resource>/
    └── ...
```

## Running Lifecycle Tests

### Prerequisites

```bash
# Get credentials from container environment (example for 2.4)
# Container names: checkmk-2.2.0p47, checkmk-2.3.0p48, checkmk-2.4.0p32
CONTAINER="checkmk-2.4.0p32"

export CHECKMK_URL="http://localhost:5040/test"
export CHECKMK_USERNAME="automation"
export CHECKMK_PASSWORD=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$CONTAINER" | grep CMK_AUTOMATION_SECRET | cut -d= -f2-)
```

### Run a Single Resource Test

```bash
cd testing/lifecycle/<resource>
./run_test.sh
```

### Run All Lifecycle Tests

```bash
cd testing/lifecycle
./run_all_tests.sh
```

## Lifecycle Test Steps

Each test follows these steps:

### 1. Setup
```bash
terraform init
```

### 2. Create
```bash
terraform plan -out=create.tfplan
terraform apply create.tfplan
```
**Verify**: Resource exists via CheckMK API

### 3. Read (Refresh)
```bash
terraform refresh
terraform plan
```
**Verify**: Plan shows "No changes" - state matches remote

### 4. Update
```bash
# Replace main.tf with updated config
cp main_updated.tf main.tf
terraform plan -out=update.tfplan
terraform apply update.tfplan
```
**Verify**:
- Plan shows expected changes
- Resource updated in CheckMK API

### 5. Import
```bash
# Remove from state
terraform state rm <resource_address>

# Import existing resource
terraform import <resource_address> <resource_id>

# Verify import succeeded
terraform plan
```
**Verify**: Plan shows "No changes" after import

### 6. Destroy
```bash
terraform destroy -auto-approve
```
**Verify**: Resource returns 404 from API

## Writing New Lifecycle Tests

### 1. Create Directory
```bash
mkdir -p testing/lifecycle/<resource_name>
```

### 2. Create main.tf
```hcl
terraform {
  required_providers {
    checkmk = {
      source = "blackmesaltd/checkmk"
    }
  }
}

provider "checkmk" {}

resource "checkmk_<resource>" "test" {
  # Initial configuration
}
```

### 3. Create main_updated.tf
```hcl
# Same as main.tf but with updated values
resource "checkmk_<resource>" "test" {
  # Updated configuration
}
```

### 4. Create run_test.sh
```bash
#!/bin/bash
set -e

RESOURCE_TYPE="checkmk_<resource>"
RESOURCE_NAME="test"
RESOURCE_ID="<id_value>"

source ../run_lifecycle_test.sh
run_lifecycle_test "$RESOURCE_TYPE" "$RESOURCE_NAME" "$RESOURCE_ID"
```

## Troubleshooting

### "No changes" expected but plan shows changes
- Check if API returns fields differently than Terraform expects
- Look for ordering issues in lists/sets
- Verify computed fields are handled correctly

### Import fails
- Verify the import ID format matches what the resource expects
- Check ImportState implementation in the resource

### Destroy fails
- Check if resource has dependencies
- Verify delete endpoint returns correct status codes

## API Verification Commands

```bash
# Check if resource exists
curl -s -u "$CHECKMK_USERNAME:$CHECKMK_PASSWORD" \
  "$CHECKMK_URL/check_mk/api/1.0/objects/<domain>/<id>" | jq

# List all resources of type
curl -s -u "$CHECKMK_USERNAME:$CHECKMK_PASSWORD" \
  "$CHECKMK_URL/check_mk/api/1.0/domain-types/<domain>/collections/all" | jq
```
