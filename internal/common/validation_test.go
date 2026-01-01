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
		name           string
		attributes     map[string]string
		expectErrors   int
		errorContains  string
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
