# Custom Attributes in CheckMK

## Overview

CheckMK supports **custom host attributes** that extend the built-in attributes (`alias`, `ipaddress`, `site`, etc.). Custom attributes are site-specific and must be configured manually via the OMD CLI - they cannot be created or managed through the REST API.

## Important Limitations

⚠️ **Custom attributes can only be created via CLI access to the OMD site**

- The REST API allows **using** custom attributes but not **creating** them
- The OpenAPI specification will NOT list custom attributes (they're site-specific)
- The Terraform provider works with custom attributes automatically (any key-value pair)

## Common Use Cases for Custom Attributes

Custom attributes are typically used for:

- **Network Configuration**: `proxy_port`, `tcp_port`, `rtt_warn`, `rtt_crit`
- **Device Metadata**: `device_description`, `device_make`, `notes`
- **Integration Fields**: `snowgroup`, `snow_action`, `snowservice` (ServiceNow)
- **Circuit References**: `vf_circuit`, `centrica_circuit`
- **Location Data**: `site_address`
- **Alerting Configuration**: `twilio`, custom escalation fields

## How to Configure Custom Attributes

### 1. Access the OMD Site

```bash
# SSH to CheckMK server
ssh user@checkmk-server

# Switch to OMD site user
su - cmk  # or your site name
```

### 2. Create Custom Attributes File

Create or edit: `etc/check_mk/multisite.d/wato/custom_attrs.mk`

```python
# encoding: utf-8

wato_host_attrs = []
if type(wato_host_attrs) != list:
    wato_host_attrs = []

wato_host_attrs += [
    {
        'add_custom_macro': True,
        'help': 'Set the Proxy Port',
        'name': 'proxy_port',
        'show_in_table': True,
        'title': 'Proxy Port',
        'topic': 'address',
        'type': 'TextAscii'
    },
    {
        'add_custom_macro': True,
        'help': 'Device description for documentation',
        'name': 'device_description',
        'show_in_table': True,
        'title': 'Device Description',
        'topic': 'basic',
        'type': 'TextAscii'
    },
    {
        'add_custom_macro': True,
        'help': 'ServiceNow group for ticket assignment',
        'name': 'snowgroup',
        'show_in_table': False,
        'title': 'ServiceNow Group',
        'topic': 'alerting',
        'type': 'TextAscii'
    }
]
```

### 3. Restart CheckMK

```bash
omd restart
```

## Using Custom Attributes with Terraform

Once custom attributes are configured in CheckMK, the Terraform provider can use them immediately. 

### Smart Validation

The provider supports **smart validation** for custom attributes:
- ✅ You can use attributes **without any prefix** (e.g., `proxy_port = "8080"`).
- ✅ You can still use the **`tag_` prefix** to explicitly mark an attribute as custom and skip fuzzy matching.
- ❌ The provider will **catch typos** of built-in attributes. For example, if you type `ailas = "..."`, the provider will detect it's close to the built-in `alias` and show an error: *"Did you mean 'alias'?"*. This check uses a Levenshtein distance of 2.

```hcl
resource "checkmk_host" "network_device" {
  host_name = "router-01"
  folder    = "/network"

  attributes = {
    # Built-in attributes
    alias     = "Core Router 01"
    ipaddress = "10.0.1.1"

    # Custom attributes (accepted without prefix)
    proxy_port         = "8080"
    device_description = "Main office core router"
    device_make        = "Cisco"
    snowgroup          = "network-team"
  }
}
```

## Validation and Error Handling

### Provider Behavior

- ✅ The provider accepts any attribute name that doesn't closely match a built-in attribute.
- ✅ CheckMK API validates whether the attribute exists during `apply`.
- ❌ If you use a non-existent custom attribute that doesn't exist in CheckMK, the API will return an error during execution.

### Example Error

```
Error: Client Error
Unable to create host, got error: API error (400):
Unknown attribute: 'my_custom_field'
```

**Solution**: Ensure the custom attribute is configured in CheckMK via CLI first.

## Docker/Testing Setup

For testing environments with Docker containers, custom attributes can be bootstrapped during container startup:

### Bootstrap Script Example

```bash
#!/bin/bash
# docker-entrypoint.d/pre-start/custom_attrs.sh

echo "Copying custom attributes file"
volume_path='/wato'
app_path='/omd/sites/cmk/etc/check_mk/multisite.d/wato'

cp "${volume_path}/custom_attrs.mk" "${app_path}/custom_attrs.mk"
```

### Docker Compose Volume

```yaml
services:
  checkmk:
    image: checkmk/check-mk-enterprise:2.3.0p41
    volumes:
      - ./custom_attrs.mk:/wato/custom_attrs.mk:ro
      - ./custom_attrs.sh:/docker-entrypoint.d/pre-start/custom_attrs.sh:ro
```

## Best Practices

### 1. Document Your Custom Attributes

Maintain a separate document listing all custom attributes:

```markdown
# CheckMK Custom Attributes

| Name | Type | Purpose | Team |
|------|------|---------|------|
| proxy_port | TextAscii | HTTP proxy port | Network |
| snowgroup | TextAscii | ServiceNow assignment group | Operations |
| device_make | TextAscii | Hardware manufacturer | Asset Management |
```

### 2. Use Lifecycle Ignore for Dynamic Fields

Some custom attributes may be modified via UI ("click-ops"):

```hcl
resource "checkmk_host" "example" {
  host_name = "server"

  attributes = {
    alias              = "Production Server"
    device_description = "Web application server"
    notes              = "Initial notes"  # May be updated via UI
  }

  lifecycle {
    ignore_changes = [
      attributes["notes"],  # Allow operators to update notes via UI
    ]
  }
}
```

### 3. Validate Before Deployment

Test that custom attributes exist before deploying Terraform:

```bash
# Test with curl
curl -u "automation:password" \
  "http://checkmk-server/cmk/check_mk/api/1.0/domain-types/host_config/collections/all" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "host_name": "test-host",
    "folder": "/",
    "attributes": {
      "proxy_port": "8080"
    }
  }'

# If custom attribute doesn't exist, you'll get an error immediately
```

## Troubleshooting

### Attribute Not Found Error

**Problem**: `Unknown attribute: 'custom_field'`

**Solution**:
1. Verify the custom attribute is defined in `custom_attrs.mk`
2. Restart CheckMK: `omd restart`
3. Check the WATO configuration UI to see available attributes

### Attribute Not Appearing in UI

**Problem**: Custom attribute configured but not visible in WATO

**Solution**:
1. Check file ownership: `chown cmk:cmk etc/check_mk/multisite.d/wato/custom_attrs.mk`
2. Check file syntax (must be valid Python)
3. Check CheckMK logs: `var/log/web.log`

### Docker Container Not Loading Attributes

**Problem**: Custom attributes not available in test containers

**Solution**:
1. Verify volume mount is correct
2. Check bootstrap script execution: `docker logs <container>`
3. Verify file permissions in container
4. Ensure script runs before CheckMK starts (use `pre-start` hook)

## Summary

| Aspect | Details |
|--------|---------|
| **Creation** | CLI only - cannot create via API |
| **Usage** | Terraform provider works with any custom attributes |
| **Discovery** | Not listed in OpenAPI spec (site-specific) |
| **Validation** | CheckMK API validates at runtime |
| **Testing** | Must bootstrap into test containers |
| **Best Practice** | Document custom attributes separately from Terraform |

## References

- [CheckMK Custom Attributes Documentation](https://docs.checkmk.com/latest/en/wato_user.html#custom_attributes)
- [Terraform Lifecycle Meta-Arguments](https://www.terraform.io/language/meta-arguments/lifecycle)
- Example custom_attrs.mk: `/workspaces/networks-codespace/networks-checkmk/import/rest/check_mk/config/custom_attrs.py`
