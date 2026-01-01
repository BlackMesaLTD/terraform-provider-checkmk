terraform {
  required_providers {
    checkmk = {
      source = "blackmesaltd/checkmk"
    }
  }
}

# Configure the CheckMK Provider with advanced options
provider "checkmk" {
  url      = "http://localhost:5000/test"
  username = "automation"
  password = "your-secret-here"

  # Activation settings
  activate              = "auto" # or "manual" (default)
  force_foreign_changes = true   # Activate changes made by other users
  activation_wait_time  = 5      # Seconds to wait after activation

  # Concurrency control
  strict_resource_locking = false # Use If-Match: * (Python approach)

  # Type validation mode (default: "auto")
  # "auto"   - Use static types for known versions, fall back to hollow with warning
  # "static" - Use static types for known versions, fail with error for unknown
  # "hollow" - Skip type validation, rely on API
  type_mode = "auto"

  # HTTP client settings
  request_timeout = 60 # Timeout in seconds
  max_retries     = 3  # Retries on 429/5xx errors

  # TLS settings (only for testing with self-signed certs)
  # insecure_skip_verify = true  # WARNING: Insecure, do not use in production
}
