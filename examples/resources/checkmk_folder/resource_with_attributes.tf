# Folder with attributes inherited by its hosts.
#
# Built-in tag groups may be written with or without the `tag_` prefix; the
# provider promotes the unprefixed form to the API automatically. Both styles
# are shown below and are equivalent.
resource "checkmk_folder" "databases" {
  name  = "databases"
  path  = "/"
  title = "Database Servers"

  attributes = {
    # Unprefixed built-in tag groups (promoted to tag_criticality / tag_networking).
    criticality = "prod"
    networking  = "lan"
  }
}

# The explicit `tag_` form remains valid and is required for custom tag groups.
resource "checkmk_folder" "web" {
  name  = "web"
  path  = "/"
  title = "Web Servers"

  attributes = {
    tag_criticality = "prod"
    tag_networking  = "dmz"
  }
}
