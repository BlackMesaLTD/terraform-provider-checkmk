// Package common provides shared utilities for the Terraform provider.
package common

// SchemaInfo maps a Terraform resource to its OpenAPI schemas.
// Each resource may use different schemas for create, update, and read operations.
type SchemaInfo struct {
	// CreateSchema is the OpenAPI schema used for create operations.
	CreateSchema string

	// UpdateSchema is the OpenAPI schema used for update operations.
	// May be empty if same as CreateSchema.
	UpdateSchema string

	// ResponseSchema is the OpenAPI schema used for API response parsing.
	ResponseSchema string

	// AttributeSchema is the OpenAPI schema for nested attribute validation.
	// Used by hosts/folders for their "attributes" map.
	AttributeSchema string
}

// SchemaRegistry maps Terraform resource type names to their OpenAPI schemas.
// This enables version-aware validation and documentation generation.
var SchemaRegistry = map[string]SchemaInfo{
	// Core resources
	"checkmk_host": {
		CreateSchema:    "HostCreateAttribute",
		UpdateSchema:    "HostUpdateAttribute",
		ResponseSchema:  "HostConfig",
		AttributeSchema: "HostCreateAttribute",
	},
	"checkmk_folder": {
		CreateSchema:    "FolderCreateAttribute",
		UpdateSchema:    "FolderUpdateAttribute",
		ResponseSchema:  "Folder",
		AttributeSchema: "FolderCreateAttribute",
	},

	// Users & contacts
	"checkmk_user": {
		CreateSchema:   "UserObject",
		UpdateSchema:   "UserObject",
		ResponseSchema: "UserObject",
	},
	"checkmk_contact_group": {
		CreateSchema:   "ContactGroup",
		UpdateSchema:   "ContactGroup",
		ResponseSchema: "ContactGroup",
	},
	"checkmk_password": {
		CreateSchema:   "PasswordObject",
		UpdateSchema:   "PasswordObject",
		ResponseSchema: "PasswordObject",
	},

	// Groups
	"checkmk_host_group": {
		CreateSchema:   "HostGroup",
		UpdateSchema:   "HostGroup",
		ResponseSchema: "HostGroup",
	},
	"checkmk_service_group": {
		CreateSchema:   "ServiceGroup",
		UpdateSchema:   "ServiceGroup",
		ResponseSchema: "ServiceGroup",
	},

	// Configuration
	"checkmk_tag_group": {
		CreateSchema:   "HostTag",
		UpdateSchema:   "HostTag",
		ResponseSchema: "HostTag",
	},
	"checkmk_aux_tag": {
		CreateSchema:   "AuxTagAttrsCreate",
		UpdateSchema:   "AuxTagAttrsUpdate",
		ResponseSchema: "AuxTagAttrsResponse",
	},
	"checkmk_time_period": {
		CreateSchema:   "TimePeriod",
		UpdateSchema:   "TimePeriod",
		ResponseSchema: "TimePeriodAttrsResponse",
	},
	"checkmk_activation": {
		CreateSchema:   "ActivateChanges",
		ResponseSchema: "ActivationRunResponse",
	},

	// Rules
	"checkmk_rule": {
		CreateSchema:   "RuleObject",
		UpdateSchema:   "RuleObject",
		ResponseSchema: "RuleObject",
	},
	"checkmk_notification_rule": {
		CreateSchema:   "NotificationRuleConfig",
		UpdateSchema:   "NotificationRuleConfig",
		ResponseSchema: "NotificationRuleConfig",
	},

	// Labels (work with host/service attributes)
	"checkmk_host_labels": {
		CreateSchema:   "HostUpdateAttribute",
		UpdateSchema:   "HostUpdateAttribute",
		ResponseSchema: "HostConfig",
	},
	"checkmk_service_labels": {
		CreateSchema:   "RuleObject",
		UpdateSchema:   "RuleObject",
		ResponseSchema: "RuleObject",
	},
}

// GetSchemaForResource returns the schema info for a Terraform resource type.
// Returns the info and true if found, or empty info and false if not found.
func GetSchemaForResource(resourceType string) (SchemaInfo, bool) {
	info, ok := SchemaRegistry[resourceType]
	return info, ok
}

// GetCreateSchema returns the create schema for a resource type.
// Returns empty string if not found.
func GetCreateSchema(resourceType string) string {
	if info, ok := SchemaRegistry[resourceType]; ok {
		return info.CreateSchema
	}
	return ""
}

// GetUpdateSchema returns the update schema for a resource type.
// Falls back to CreateSchema if UpdateSchema is empty.
// Returns empty string if not found.
func GetUpdateSchema(resourceType string) string {
	if info, ok := SchemaRegistry[resourceType]; ok {
		if info.UpdateSchema != "" {
			return info.UpdateSchema
		}
		return info.CreateSchema
	}
	return ""
}

// GetResponseSchema returns the response schema for a resource type.
// Returns empty string if not found.
func GetResponseSchema(resourceType string) string {
	if info, ok := SchemaRegistry[resourceType]; ok {
		return info.ResponseSchema
	}
	return ""
}

// GetAttributeSchema returns the attribute schema for a resource type.
// This is used for validating nested "attributes" maps (e.g., host/folder attributes).
// Returns empty string if not applicable.
func GetAttributeSchema(resourceType string) string {
	if info, ok := SchemaRegistry[resourceType]; ok {
		return info.AttributeSchema
	}
	return ""
}

// RegisterRuleResource adds a rule wrapper resource to the registry.
// Rule wrappers all use the same RuleObject schema.
func RegisterRuleResource(resourceType string) {
	SchemaRegistry[resourceType] = SchemaInfo{
		CreateSchema:   "RuleObject",
		UpdateSchema:   "RuleObject",
		ResponseSchema: "RuleObject",
	}
}

// init registers all the rule wrapper resources.
// These are generated resources that wrap specific ruleset types.
func init() {
	// Host rule wrappers
	hostRuleResources := []string{
		"checkmk_host_check_interval",
		"checkmk_host_check_period",
		"checkmk_host_max_check_attempts",
		"checkmk_host_notification_options",
		"checkmk_host_notification_period",
		"checkmk_host_custom_notifications",
		"checkmk_host_icon_image",
		"checkmk_host_parents",
		"checkmk_host_check_command",
		"checkmk_only_hosts",
		"checkmk_host_tags",
	}

	// Service rule wrappers
	serviceRuleResources := []string{
		"checkmk_service_check_interval",
		"checkmk_service_check_period",
		"checkmk_service_max_check_attempts",
		"checkmk_service_notification_options",
		"checkmk_service_notification_period",
		"checkmk_service_custom_notifications",
		"checkmk_custom_service_attributes",
		"checkmk_service_icon_image",
		"checkmk_service_groups",
		"checkmk_active_checks_http",
		"checkmk_active_checks_ping",
		"checkmk_active_checks_ssh",
		"checkmk_agent_config_only_from",
		"checkmk_ignored_services",
		"checkmk_ignored_checks",
		"checkmk_clustered_services",
		"checkmk_extra_host_conf_notification_interval",
		"checkmk_extra_service_conf_notification_interval",
	}

	// Register all rule wrappers
	for _, r := range hostRuleResources {
		RegisterRuleResource(r)
	}
	for _, r := range serviceRuleResources {
		RegisterRuleResource(r)
	}
}
