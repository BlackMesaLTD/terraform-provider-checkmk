// Package common provides shared utilities for the Terraform provider.
package common

import (
	"testing"
)

// =============================================================================
// Tests for SchemaRegistry and helper functions
// =============================================================================

func TestGetSchemaForResource_CoreResources(t *testing.T) {
	tests := []struct {
		resourceType string
		wantCreate   string
		wantUpdate   string
		wantResponse string
		wantAttr     string
	}{
		{
			resourceType: "checkmk_host",
			wantCreate:   "HostCreateAttribute",
			wantUpdate:   "HostUpdateAttribute",
			wantResponse: "HostConfig",
			wantAttr:     "HostCreateAttribute",
		},
		{
			resourceType: "checkmk_folder",
			wantCreate:   "FolderCreateAttribute",
			wantUpdate:   "FolderUpdateAttribute",
			wantResponse: "Folder",
			wantAttr:     "FolderCreateAttribute",
		},
		{
			resourceType: "checkmk_user",
			wantCreate:   "UserObject",
			wantUpdate:   "UserObject",
			wantResponse: "UserObject",
			wantAttr:     "",
		},
		{
			resourceType: "checkmk_contact_group",
			wantCreate:   "ContactGroup",
			wantUpdate:   "ContactGroup",
			wantResponse: "ContactGroup",
			wantAttr:     "",
		},
		{
			resourceType: "checkmk_password",
			wantCreate:   "PasswordObject",
			wantUpdate:   "PasswordObject",
			wantResponse: "PasswordObject",
			wantAttr:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			info, ok := GetSchemaForResource(tt.resourceType)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", tt.resourceType)
			}
			if info.CreateSchema != tt.wantCreate {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, tt.wantCreate)
			}
			if info.UpdateSchema != tt.wantUpdate {
				t.Errorf("UpdateSchema = %q, want %q", info.UpdateSchema, tt.wantUpdate)
			}
			if info.ResponseSchema != tt.wantResponse {
				t.Errorf("ResponseSchema = %q, want %q", info.ResponseSchema, tt.wantResponse)
			}
			if info.AttributeSchema != tt.wantAttr {
				t.Errorf("AttributeSchema = %q, want %q", info.AttributeSchema, tt.wantAttr)
			}
		})
	}
}

func TestGetSchemaForResource_GroupResources(t *testing.T) {
	tests := []struct {
		resourceType string
		wantCreate   string
	}{
		{"checkmk_host_group", "HostGroup"},
		{"checkmk_service_group", "ServiceGroup"},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			info, ok := GetSchemaForResource(tt.resourceType)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", tt.resourceType)
			}
			if info.CreateSchema != tt.wantCreate {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, tt.wantCreate)
			}
		})
	}
}

func TestGetSchemaForResource_ConfigResources(t *testing.T) {
	tests := []struct {
		resourceType string
		wantCreate   string
		wantUpdate   string
	}{
		{"checkmk_tag_group", "HostTag", "HostTag"},
		{"checkmk_aux_tag", "AuxTagAttrsCreate", "AuxTagAttrsUpdate"},
		{"checkmk_time_period", "TimePeriod", "TimePeriod"},
		{"checkmk_activation", "ActivateChanges", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			info, ok := GetSchemaForResource(tt.resourceType)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", tt.resourceType)
			}
			if info.CreateSchema != tt.wantCreate {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, tt.wantCreate)
			}
			if info.UpdateSchema != tt.wantUpdate {
				t.Errorf("UpdateSchema = %q, want %q", info.UpdateSchema, tt.wantUpdate)
			}
		})
	}
}

func TestGetSchemaForResource_RuleResources(t *testing.T) {
	tests := []struct {
		resourceType string
		wantCreate   string
	}{
		{"checkmk_rule", "RuleObject"},
		{"checkmk_notification_rule", "NotificationRuleConfig"},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			info, ok := GetSchemaForResource(tt.resourceType)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", tt.resourceType)
			}
			if info.CreateSchema != tt.wantCreate {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, tt.wantCreate)
			}
		})
	}
}

func TestGetSchemaForResource_LabelResources(t *testing.T) {
	tests := []struct {
		resourceType string
		wantCreate   string
	}{
		{"checkmk_host_labels", "HostUpdateAttribute"},
		{"checkmk_service_labels", "RuleObject"},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			info, ok := GetSchemaForResource(tt.resourceType)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", tt.resourceType)
			}
			if info.CreateSchema != tt.wantCreate {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, tt.wantCreate)
			}
		})
	}
}

func TestGetSchemaForResource_RuleWrappers(t *testing.T) {
	// All rule wrappers should use RuleObject schema
	ruleWrappers := []string{
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

	for _, wrapper := range ruleWrappers {
		t.Run(wrapper, func(t *testing.T) {
			info, ok := GetSchemaForResource(wrapper)
			if !ok {
				t.Fatalf("GetSchemaForResource(%q) returned false, expected true", wrapper)
			}
			if info.CreateSchema != "RuleObject" {
				t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, "RuleObject")
			}
			if info.UpdateSchema != "RuleObject" {
				t.Errorf("UpdateSchema = %q, want %q", info.UpdateSchema, "RuleObject")
			}
			if info.ResponseSchema != "RuleObject" {
				t.Errorf("ResponseSchema = %q, want %q", info.ResponseSchema, "RuleObject")
			}
		})
	}
}

func TestGetSchemaForResource_NotFound(t *testing.T) {
	info, ok := GetSchemaForResource("nonexistent_resource")
	if ok {
		t.Error("GetSchemaForResource() returned true for nonexistent resource")
	}
	if info.CreateSchema != "" {
		t.Errorf("CreateSchema = %q, want empty string", info.CreateSchema)
	}
}

// =============================================================================
// Tests for individual helper functions
// =============================================================================

func TestGetCreateSchema(t *testing.T) {
	tests := []struct {
		resourceType string
		want         string
	}{
		{"checkmk_host", "HostCreateAttribute"},
		{"checkmk_folder", "FolderCreateAttribute"},
		{"checkmk_rule", "RuleObject"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			got := GetCreateSchema(tt.resourceType)
			if got != tt.want {
				t.Errorf("GetCreateSchema(%q) = %q, want %q", tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestGetUpdateSchema(t *testing.T) {
	tests := []struct {
		resourceType string
		want         string
	}{
		{"checkmk_host", "HostUpdateAttribute"},
		{"checkmk_folder", "FolderUpdateAttribute"},
		{"checkmk_rule", "RuleObject"},            // Same as create
		{"checkmk_aux_tag", "AuxTagAttrsUpdate"},  // Different from create
		{"checkmk_activation", "ActivateChanges"}, // Falls back to create (update is empty)
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			got := GetUpdateSchema(tt.resourceType)
			if got != tt.want {
				t.Errorf("GetUpdateSchema(%q) = %q, want %q", tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestGetResponseSchema(t *testing.T) {
	tests := []struct {
		resourceType string
		want         string
	}{
		{"checkmk_host", "HostConfig"},
		{"checkmk_folder", "Folder"},
		{"checkmk_aux_tag", "AuxTagAttrsResponse"},
		{"checkmk_time_period", "TimePeriodAttrsResponse"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			got := GetResponseSchema(tt.resourceType)
			if got != tt.want {
				t.Errorf("GetResponseSchema(%q) = %q, want %q", tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestGetAttributeSchema(t *testing.T) {
	tests := []struct {
		resourceType string
		want         string
	}{
		{"checkmk_host", "HostCreateAttribute"},
		{"checkmk_folder", "FolderCreateAttribute"},
		{"checkmk_user", ""}, // No attribute schema
		{"checkmk_rule", ""}, // No attribute schema
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			got := GetAttributeSchema(tt.resourceType)
			if got != tt.want {
				t.Errorf("GetAttributeSchema(%q) = %q, want %q", tt.resourceType, got, tt.want)
			}
		})
	}
}

func TestRegisterRuleResource(t *testing.T) {
	// Register a new rule resource
	RegisterRuleResource("checkmk_test_rule_wrapper")

	info, ok := GetSchemaForResource("checkmk_test_rule_wrapper")
	if !ok {
		t.Fatal("RegisterRuleResource() did not register the resource")
	}

	if info.CreateSchema != "RuleObject" {
		t.Errorf("CreateSchema = %q, want %q", info.CreateSchema, "RuleObject")
	}
	if info.UpdateSchema != "RuleObject" {
		t.Errorf("UpdateSchema = %q, want %q", info.UpdateSchema, "RuleObject")
	}
	if info.ResponseSchema != "RuleObject" {
		t.Errorf("ResponseSchema = %q, want %q", info.ResponseSchema, "RuleObject")
	}

	// Clean up
	delete(SchemaRegistry, "checkmk_test_rule_wrapper")
}

// =============================================================================
// Test registry coverage - ensure all expected resources are registered
// =============================================================================

func TestSchemaRegistry_ExpectedResources(t *testing.T) {
	// Core resources that should always be registered
	expectedResources := []string{
		"checkmk_host",
		"checkmk_folder",
		"checkmk_user",
		"checkmk_contact_group",
		"checkmk_password",
		"checkmk_host_group",
		"checkmk_service_group",
		"checkmk_tag_group",
		"checkmk_aux_tag",
		"checkmk_time_period",
		"checkmk_activation",
		"checkmk_rule",
		"checkmk_notification_rule",
		"checkmk_host_labels",
		"checkmk_service_labels",
	}

	for _, resource := range expectedResources {
		t.Run(resource, func(t *testing.T) {
			_, ok := GetSchemaForResource(resource)
			if !ok {
				t.Errorf("Expected resource %q not found in SchemaRegistry", resource)
			}
		})
	}
}

func TestSchemaRegistry_AllEntriesHaveCreateSchema(t *testing.T) {
	for resourceType, info := range SchemaRegistry {
		t.Run(resourceType, func(t *testing.T) {
			if info.CreateSchema == "" {
				t.Errorf("Resource %q has empty CreateSchema", resourceType)
			}
		})
	}
}

func TestSchemaRegistry_AllEntriesHaveResponseSchema(t *testing.T) {
	for resourceType, info := range SchemaRegistry {
		t.Run(resourceType, func(t *testing.T) {
			if info.ResponseSchema == "" {
				t.Errorf("Resource %q has empty ResponseSchema", resourceType)
			}
		})
	}
}

func TestSchemaRegistry_CountResources(t *testing.T) {
	// Count total registered resources
	count := len(SchemaRegistry)

	// We expect at least 40 resources (core + rule wrappers)
	minExpected := 40
	if count < minExpected {
		t.Errorf("SchemaRegistry has %d resources, expected at least %d", count, minExpected)
	}

	t.Logf("Total registered resources: %d", count)
}
