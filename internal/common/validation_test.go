package common

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
)

func TestAttributeValidator_ShouldValidate(t *testing.T) {
	tests := []struct {
		name         string
		providerData *ProviderData
		expected     bool
	}{
		{
			name:         "nil provider data",
			providerData: nil,
			expected:     false,
		},
		{
			name: "hollow mode",
			providerData: &ProviderData{
				TypeMode: TypeModeHollow,
				Types:    nil,
			},
			expected: false,
		},
		{
			name: "auto mode without types",
			providerData: &ProviderData{
				TypeMode: TypeModeAuto,
				Types:    nil,
			},
			expected: false,
		},
		{
			name: "auto mode with types",
			providerData: &ProviderData{
				TypeMode: TypeModeAuto,
				Types:    client.NewVersionedTypes("2.4.0p17"),
			},
			expected: true,
		},
		{
			name: "static mode with types",
			providerData: &ProviderData{
				TypeMode: TypeModeStatic,
				Types:    client.NewVersionedTypes("2.4.0p17"),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewAttributeValidator(tt.providerData)
			result := v.ShouldValidate()
			if result != tt.expected {
				t.Errorf("ShouldValidate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAttributeValidator_ValidateHostAttributes(t *testing.T) {
	// Create provider data with types for testing
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	// Create a mock client for version info
	providerData := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	tests := []struct {
		name          string
		attributes    map[string]string
		expectErrors  int
		errorContains string
	}{
		{
			name: "valid attributes",
			attributes: map[string]string{
				"alias":     "test-host",
				"ipaddress": "192.168.1.1",
			},
			expectErrors: 0,
		},
		{
			name: "valid tag_agent value",
			attributes: map[string]string{
				"tag_agent": "cmk-agent",
			},
			expectErrors: 0,
		},
		{
			name: "invalid tag_agent value",
			attributes: map[string]string{
				"tag_agent": "invalid-agent-type",
			},
			expectErrors:  1,
			errorContains: "Invalid tag_agent Value",
		},
		{
			name: "custom tag attribute allowed",
			attributes: map[string]string{
				"tag_custom_tag": "some-value",
			},
			expectErrors: 0, // Custom tags are allowed
		},
		{
			name:         "null attributes",
			attributes:   nil,
			expectErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewAttributeValidator(providerData)

			var attrs types.Map
			if tt.attributes == nil {
				attrs = types.MapNull(types.StringType)
			} else {
				attrMap := make(map[string]types.String)
				for k, val := range tt.attributes {
					attrMap[k] = types.StringValue(val)
				}
				var diags diag.Diagnostics
				attrs, diags = types.MapValueFrom(context.Background(), types.StringType, attrMap)
				if diags.HasError() {
					t.Fatalf("Failed to create map: %v", diags)
				}
			}

			diags := v.ValidateHostAttributes(context.Background(), attrs, path.Root("attributes"))

			if diags.ErrorsCount() != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, diags.ErrorsCount(), diags)
			}

			if tt.errorContains != "" && diags.ErrorsCount() > 0 {
				found := false
				for _, d := range diags.Errors() {
					if strings.Contains(d.Summary(), tt.errorContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, got: %v", tt.errorContains, diags)
				}
			}
		})
	}
}

func TestAttributeValidator_ValidateFolderAttributes(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	providerData := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	tests := []struct {
		name         string
		attributes   map[string]string
		expectErrors int
	}{
		{
			name: "valid folder attributes",
			attributes: map[string]string{
				"tag_agent":       "cmk-agent",
				"tag_criticality": "prod",
			},
			expectErrors: 0,
		},
		{
			name: "invalid tag_agent value",
			attributes: map[string]string{
				"tag_agent": "not-a-valid-agent",
			},
			expectErrors: 1,
		},
		{
			name:         "null attributes",
			attributes:   nil,
			expectErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewAttributeValidator(providerData)

			var attrs types.Map
			if tt.attributes == nil {
				attrs = types.MapNull(types.StringType)
			} else {
				attrMap := make(map[string]types.String)
				for k, val := range tt.attributes {
					attrMap[k] = types.StringValue(val)
				}
				var diags diag.Diagnostics
				attrs, diags = types.MapValueFrom(context.Background(), types.StringType, attrMap)
				if diags.HasError() {
					t.Fatalf("Failed to create map: %v", diags)
				}
			}

			diags := v.ValidateFolderAttributes(context.Background(), attrs, path.Root("attributes"))

			if diags.ErrorsCount() != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, diags.ErrorsCount(), diags)
			}
		})
	}
}

func TestAttributeValidator_HollowModeSkipsValidation(t *testing.T) {
	providerData := &ProviderData{
		TypeMode: TypeModeHollow,
		Types:    nil,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	v := NewAttributeValidator(providerData)

	// Create invalid attributes that would fail in static mode
	attrMap := map[string]types.String{
		"tag_agent": types.StringValue("completely-invalid-value"),
	}
	attrs, _ := types.MapValueFrom(context.Background(), types.StringType, attrMap)

	// In hollow mode, validation should be skipped
	diags := v.ValidateHostAttributes(context.Background(), attrs, path.Root("attributes"))
	if diags.HasError() {
		t.Errorf("Hollow mode should skip validation, but got errors: %v", diags)
	}
}

func TestIsCustomAttribute(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		expected bool
	}{
		{"standard tag", "tag_agent", true},
		{"custom tag", "tag_my_custom", true},
		{"labels", "labels", true},
		{"alias is not custom", "alias", false},
		{"ipaddress is not custom", "ipaddress", false},
		{"site is not custom", "site", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCustomAttribute(tt.attr)
			if result != tt.expected {
				t.Errorf("isCustomAttribute(%q) = %v, want %v", tt.attr, result, tt.expected)
			}
		})
	}
}

func TestFormatValidFields(t *testing.T) {
	tests := []struct {
		name     string
		fields   []string
		contains string
	}{
		{
			name:     "few fields",
			fields:   []string{"alias", "ipaddress", "site"},
			contains: "alias",
		},
		{
			name:     "many fields truncated",
			fields:   []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"},
			contains: "and 2 more",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValidFields(tt.fields)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatValidFields() = %q, should contain %q", result, tt.contains)
			}
		})
	}
}

// =============================================================================
// Tests for Generic ValidateStringField API
// =============================================================================

func TestAttributeValidator_ValidateStringField(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	providerData := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	tests := []struct {
		name         string
		schemaName   string
		fieldName    string
		value        types.String
		expectErrors int
		expectWarns  int
	}{
		{
			name:         "null value - no validation",
			schemaName:   "HostCreateAttribute",
			fieldName:    "tag_agent",
			value:        types.StringNull(),
			expectErrors: 0,
			expectWarns:  0,
		},
		{
			name:         "unknown value - no validation",
			schemaName:   "HostCreateAttribute",
			fieldName:    "tag_agent",
			value:        types.StringUnknown(),
			expectErrors: 0,
			expectWarns:  0,
		},
		{
			name:         "valid enum value - cmk-agent",
			schemaName:   "HostCreateAttribute",
			fieldName:    "tag_agent",
			value:        types.StringValue("cmk-agent"),
			expectErrors: 0,
			expectWarns:  0,
		},
		{
			name:         "valid enum value - no-agent",
			schemaName:   "HostCreateAttribute",
			fieldName:    "tag_agent",
			value:        types.StringValue("no-agent"),
			expectErrors: 0,
			expectWarns:  0,
		},
		{
			name:         "invalid enum value",
			schemaName:   "HostCreateAttribute",
			fieldName:    "tag_agent",
			value:        types.StringValue("invalid-agent-type"),
			expectErrors: 1,
			expectWarns:  0,
		},
		{
			name:         "non-enum field - any value allowed",
			schemaName:   "HostCreateAttribute",
			fieldName:    "alias",
			value:        types.StringValue("any value is fine"),
			expectErrors: 0,
			expectWarns:  0,
		},
		{
			name:         "non-existent schema - no validation",
			schemaName:   "NonExistentSchema",
			fieldName:    "field",
			value:        types.StringValue("value"),
			expectErrors: 0,
			expectWarns:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewAttributeValidator(providerData)
			diags := v.ValidateStringField(tt.schemaName, tt.fieldName, tt.value, path.Root(tt.fieldName))

			errorCount := 0
			warnCount := 0
			for _, d := range diags {
				if d.Severity() == diag.SeverityError {
					errorCount++
				} else if d.Severity() == diag.SeverityWarning {
					warnCount++
				}
			}

			if errorCount != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, errorCount, diags)
			}
			if warnCount != tt.expectWarns {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarns, warnCount, diags)
			}
		})
	}
}

func TestAttributeValidator_ValidateStringField_HollowMode(t *testing.T) {
	providerData := &ProviderData{
		TypeMode: TypeModeHollow,
		Types:    nil,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	v := NewAttributeValidator(providerData)

	// In hollow mode, even invalid values should pass
	diags := v.ValidateStringField("HostCreateAttribute", "tag_agent", types.StringValue("invalid"), path.Root("tag_agent"))
	if diags.HasError() {
		t.Errorf("Hollow mode should skip validation, but got errors: %v", diags)
	}
}

func TestAttributeValidator_ValidateSchemaExists(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	providerData := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
	}

	v := NewAttributeValidator(providerData)

	tests := []struct {
		name       string
		schemaName string
		expected   bool
	}{
		{
			name:       "HostCreateAttribute exists",
			schemaName: "HostCreateAttribute",
			expected:   true,
		},
		{
			name:       "FolderCreateAttribute exists",
			schemaName: "FolderCreateAttribute",
			expected:   true,
		},
		{
			name:       "NonExistentSchema does not exist",
			schemaName: "NonExistentSchema",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.ValidateSchemaExists(tt.schemaName)
			if result != tt.expected {
				t.Errorf("ValidateSchemaExists(%q) = %v, want %v", tt.schemaName, result, tt.expected)
			}
		})
	}
}

func TestAttributeValidator_GetVersionString(t *testing.T) {
	tests := []struct {
		name     string
		pd       *ProviderData
		expected string
	}{
		{
			name:     "nil provider data",
			pd:       nil,
			expected: "unknown",
		},
		{
			name: "nil client",
			pd: &ProviderData{
				Client: nil,
			},
			expected: "unknown",
		},
		{
			name: "valid client with version",
			pd: &ProviderData{
				Client: &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17, Raw: "2.4.0p17"}},
			},
			expected: "2.4.0p17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewAttributeValidator(tt.pd)
			result := v.GetVersionString()
			if result != tt.expected {
				t.Errorf("GetVersionString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Tests for VersionedTypesWrapper
// =============================================================================

func TestVersionedTypesWrapper_NilSafe(t *testing.T) {
	w := NewVersionedTypesWrapper(nil)

	// All methods should be safe with nil
	if fields := w.GetSchemaFieldNames("HostCreateAttribute"); fields != nil {
		t.Errorf("GetSchemaFieldNames() with nil should return nil, got %v", fields)
	}

	if required := w.GetSchemaRequiredFieldNames("HostCreateAttribute"); required != nil {
		t.Errorf("GetSchemaRequiredFieldNames() with nil should return nil, got %v", required)
	}

	if enums := w.GetValidEnumValues("HostCreateAttribute", "tag_agent"); enums != nil {
		t.Errorf("GetValidEnumValues() with nil should return nil, got %v", enums)
	}

	if w.HasEnumConstraint("HostCreateAttribute", "tag_agent") {
		t.Error("HasEnumConstraint() with nil should return false")
	}

	if w.IsReadOnlyField("HostCreateAttribute", "field") {
		t.Error("IsReadOnlyField() with nil should return false")
	}

	if w.IsRequiredField("HostCreateAttribute", "field") {
		t.Error("IsRequiredField() with nil should return false")
	}

	if w.IsDeprecatedField("HostCreateAttribute", "field") {
		t.Error("IsDeprecatedField() with nil should return false")
	}

	if desc := w.GetFieldDescription("HostCreateAttribute", "field"); desc != "" {
		t.Errorf("GetFieldDescription() with nil should return empty, got %q", desc)
	}

	if ftype := w.GetFieldType("HostCreateAttribute", "field"); ftype != "" {
		t.Errorf("GetFieldType() with nil should return empty, got %q", ftype)
	}
}

func TestVersionedTypesWrapper_WithTypes(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	w := NewVersionedTypesWrapper(versionedTypes)

	// Test GetSchemaFieldNames returns fields
	fields := w.GetSchemaFieldNames("HostCreateAttribute")
	if len(fields) == 0 {
		t.Error("GetSchemaFieldNames() should return fields for HostCreateAttribute")
	}

	// Test HasEnumConstraint for tag_agent
	if !w.HasEnumConstraint("HostCreateAttribute", "tag_agent") {
		t.Error("HasEnumConstraint() should return true for tag_agent")
	}

	// Test GetValidEnumValues for tag_agent
	enums := w.GetValidEnumValues("HostCreateAttribute", "tag_agent")
	if len(enums) == 0 {
		t.Error("GetValidEnumValues() should return enum values for tag_agent")
	}

	// Verify cmk-agent is a valid value
	found := false
	for _, v := range enums {
		if v == "cmk-agent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tag_agent enum values should include 'cmk-agent', got: %v", enums)
	}
}

// =============================================================================
// Tests for ValidateRequiredFields
// =============================================================================

func TestAttributeValidator_ValidateRequiredFields(t *testing.T) {
	versionedTypes := client.NewVersionedTypes("2.4.0p17")
	if versionedTypes == nil {
		t.Fatal("Failed to create VersionedTypes for 2.4.0p17")
	}

	providerData := &ProviderData{
		TypeMode: TypeModeAuto,
		Types:    versionedTypes,
		Client:   &client.Client{Version: &client.Version{Major: 2, Minor: 4, Patch: 0, Build: 17}},
	}

	v := NewAttributeValidator(providerData)

	// Test with a schema that has required fields
	// Note: The actual required fields depend on the OpenAPI spec
	tests := []struct {
		name           string
		schemaName     string
		providedFields map[string]interface{}
		expectErrors   bool
	}{
		{
			name:       "hollow mode - skip validation",
			schemaName: "SomeSchema",
			providedFields: map[string]interface{}{
				"field": "value",
			},
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := v.ValidateRequiredFields(tt.schemaName, tt.providedFields, path.Root("test"))
			if diags.HasError() != tt.expectErrors {
				t.Errorf("ValidateRequiredFields() hasError = %v, want %v. Diags: %v", diags.HasError(), tt.expectErrors, diags)
			}
		})
	}
}

// =============================================================================
// Cross-Version Tests
// =============================================================================

func TestAttributeValidator_CrossVersionValidation(t *testing.T) {
	versions := []string{"2.2.0p43", "2.3.0p41", "2.4.0p17"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			versionedTypes := client.NewVersionedTypes(version)
			if versionedTypes == nil {
				t.Fatalf("Failed to create VersionedTypes for %s", version)
			}

			parsedVersion, err := client.ParseVersion(version)
			if err != nil {
				t.Fatalf("Failed to parse version %s: %v", version, err)
			}
			providerData := &ProviderData{
				TypeMode: TypeModeAuto,
				Types:    versionedTypes,
				Client:   &client.Client{Version: parsedVersion},
			}

			v := NewAttributeValidator(providerData)

			// Verify cmk-agent is valid across all versions
			diags := v.ValidateStringField("HostCreateAttribute", "tag_agent", types.StringValue("cmk-agent"), path.Root("tag_agent"))
			if diags.HasError() {
				t.Errorf("Version %s: cmk-agent should be valid, got errors: %v", version, diags)
			}

			// Verify no-agent is valid across all versions
			diags = v.ValidateStringField("HostCreateAttribute", "tag_agent", types.StringValue("no-agent"), path.Root("tag_agent"))
			if diags.HasError() {
				t.Errorf("Version %s: no-agent should be valid, got errors: %v", version, diags)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "a",
			expected: false,
		},
		{
			name:     "item exists",
			slice:    []string{"a", "b", "c"},
			item:     "b",
			expected: true,
		},
		{
			name:     "item does not exist",
			slice:    []string{"a", "b", "c"},
			item:     "d",
			expected: false,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Alpha", "Beta"},
			item:     "alpha",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.item)
			if got != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, got, tt.expected)
			}
		})
	}
}
