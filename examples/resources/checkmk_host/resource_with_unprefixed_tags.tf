# Example: Built-in tag groups without the `tag_` prefix
#
# Built-in host tag groups are exposed by the CheckMK API with a `tag_` prefix
# (tag_agent, tag_criticality, ...). The provider lets you write them without
# the prefix for convenience: it promotes them to the API form automatically
# and keeps your configured key in state, so there is no perpetual diff.
#
# Custom attributes (proxy_port below) are passed straight through to the API
# unchanged - they are never rewritten.

resource "checkmk_host" "edge_router" {
  host_name = "edge-router-01"
  folder    = "/network/routers"

  attributes = {
    alias     = "Edge Router 01"
    ipaddress = "10.0.1.1"

    # Built-in tag groups, written without the `tag_` prefix.
    agent       = "cmk-agent"
    criticality = "prod"

    # A custom attribute (must be pre-configured in CheckMK), no prefix needed.
    proxy_port = "8080"
  }
}

# The explicit `tag_` form is still fully supported and is required for custom
# tag groups (tag groups you defined yourself), which are not part of the
# generated schema.
resource "checkmk_host" "core_switch" {
  host_name = "core-switch-01"
  folder    = "/network/switches"

  attributes = {
    alias = "Core Switch 01"

    tag_agent      = "snmp-v2"
    tag_my_company = "platform-team" # custom tag group: explicit prefix required
  }
}
