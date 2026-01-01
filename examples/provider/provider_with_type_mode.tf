terraform {
  required_providers {
    checkmk = {
      source = "blackmesaltd/checkmk"
    }
  }
}

# Configure the CheckMK Provider with type validation mode
#
# Type validation modes control how the provider validates resource attributes
# against the CheckMK API schema before sending requests.
#
# Available modes:
#   "auto"   - (Default) Use static types for known CheckMK versions (2.2, 2.3, 2.4).
#              Falls back to hollow mode with a warning for unknown versions.
#   "static" - Use static types for known versions. Fails with an error if the
#              CheckMK version is not recognized. Use this when you want to ensure
#              configurations are validated against known schemas.
#   "hollow" - Skip type validation entirely and rely on the CheckMK API for
#              validation. Useful for experimental features or untested versions.

provider "checkmk" {
  url      = "http://localhost:5000/test"
  username = "automation"
  password = "your-secret-here"

  # Use "auto" for most production deployments
  type_mode = "auto"
}

# Example: Using hollow mode for an experimental CheckMK version
#
# provider "checkmk" {
#   url       = "http://localhost:5000/test"
#   username  = "automation"
#   password  = "your-secret-here"
#   type_mode = "hollow"
# }

# Example: Using static mode to enforce version compatibility
#
# provider "checkmk" {
#   url       = "http://localhost:5000/test"
#   username  = "automation"
#   password  = "your-secret-here"
#   type_mode = "static"
# }
