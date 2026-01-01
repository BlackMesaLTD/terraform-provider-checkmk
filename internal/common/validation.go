package common

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
)

// AttributeValidator provides validation for host/folder attributes using generated types.
type AttributeValidator struct {
	providerData *ProviderData
}

// NewAttributeValidator creates a new validator for the given provider data.
func NewAttributeValidator(pd *ProviderData) *AttributeValidator {
	return &AttributeValidator{providerData: pd}
}

// ShouldValidate returns true if attribute validation should be performed.
// Returns false for hollow mode or when types are not available.
func (v *AttributeValidator) ShouldValidate() bool {
	if v.providerData == nil {
		return false
	}
	if v.providerData.TypeMode == TypeModeHollow {
		return false
	}
	return v.providerData.Types != nil
}

// ValidateHostAttributes validates host attributes against the generated types.
// Returns diagnostics with any validation errors.
func (v *AttributeValidator) ValidateHostAttributes(ctx context.Context, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	if attributes.IsNull() || attributes.IsUnknown() {
		return diags
	}

	// Get valid field names for this version
	validFields := v.providerData.Types.HostCreateAttributeFieldNames()
	if len(validFields) == 0 {
		// No field validation available, skip
		return diags
	}

	// Check each attribute key
	for key := range attributes.Elements() {
		if !contains(validFields, key) {
			// Check if it's a custom attribute (starts with tag_ or custom_)
			if isCustomAttribute(key) {
				// Custom attributes are allowed, API will validate
				continue
			}

			diags.AddAttributeError(
				attrPath.AtMapKey(key),
				"Invalid Host Attribute",
				fmt.Sprintf("Attribute %q is not valid for CheckMK version %s. "+
					"Valid attributes include: %s. "+
					"If this is a custom attribute, prefix it with 'tag_' or ensure it's defined in CheckMK.",
					key, v.providerData.Client.Version.String(), formatValidFields(validFields)),
			)
		}

		// Warn about read-only fields
		if v.providerData.Types.IsReadOnlyField("HostCreateAttribute", key) {
			diags.AddAttributeWarning(
				attrPath.AtMapKey(key),
				"Read-Only Attribute",
				fmt.Sprintf("Attribute %q is read-only and cannot be set. "+
					"It will be computed by CheckMK and any value provided will be ignored.",
					key),
			)
		}
	}

	// Validate tag_agent values if present
	diags.Append(v.validateTagAgent(ctx, attributes, attrPath)...)

	return diags
}

// ValidateFolderAttributes validates folder attributes against the generated types.
func (v *AttributeValidator) ValidateFolderAttributes(ctx context.Context, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	if attributes.IsNull() || attributes.IsUnknown() {
		return diags
	}

	validFields := v.providerData.Types.FolderCreateAttributeFieldNames()
	if len(validFields) == 0 {
		return diags
	}

	for key := range attributes.Elements() {
		if !contains(validFields, key) {
			if isCustomAttribute(key) {
				continue
			}

			diags.AddAttributeError(
				attrPath.AtMapKey(key),
				"Invalid Folder Attribute",
				fmt.Sprintf("Attribute %q is not valid for CheckMK version %s. "+
					"Valid attributes include: %s.",
					key, v.providerData.Client.Version.String(), formatValidFields(validFields)),
			)
		}

		// Warn about read-only fields
		if v.providerData.Types.IsReadOnlyField("FolderCreateAttribute", key) {
			diags.AddAttributeWarning(
				attrPath.AtMapKey(key),
				"Read-Only Attribute",
				fmt.Sprintf("Attribute %q is read-only and cannot be set. "+
					"It will be computed by CheckMK and any value provided will be ignored.",
					key),
			)
		}
	}

	// Validate tag_agent values if present
	diags.Append(v.validateTagAgent(ctx, attributes, attrPath)...)

	return diags
}

// ValidateRequiredFields validates that all required fields are present in the provided fields map.
// schemaName is the OpenAPI schema name (e.g., "CreateHost", "CreateFolder").
func (v *AttributeValidator) ValidateRequiredFields(schemaName string, providedFields map[string]interface{}, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	// Check each field that might be required
	// Since we don't have a list of required fields exposed directly, we check each provided field
	// and also need to check for missing required fields.
	// For now, we use the IsRequiredField method to check if any known required field is missing.

	// Common required fields for different schemas
	requiredFieldsToCheck := getRequiredFieldsForSchema(schemaName)

	for _, field := range requiredFieldsToCheck {
		if v.providerData.Types.IsRequiredField(schemaName, field) {
			if _, exists := providedFields[field]; !exists {
				diags.AddAttributeError(
					attrPath,
					"Missing Required Field",
					fmt.Sprintf("Field %q is required for %s in CheckMK version %s.",
						field, schemaName, v.providerData.Client.Version.String()),
				)
			}
		}
	}

	return diags
}

// getRequiredFieldsForSchema returns a list of field names to check for required status.
// This is a hint list - actual required status is determined by the generated types.
func getRequiredFieldsForSchema(schemaName string) []string {
	switch schemaName {
	case "CreateHost":
		return []string{"host_name", "folder"}
	case "CreateClusterHost":
		return []string{"host_name", "folder", "nodes"}
	case "CreateFolder":
		return []string{"name", "parent", "title"}
	case "CreateUser":
		return []string{"username", "fullname"}
	default:
		return nil
	}
}

// validateTagAgent validates the tag_agent attribute value if present.
func (v *AttributeValidator) validateTagAgent(ctx context.Context, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	elements := attributes.Elements()
	tagAgentAttr, exists := elements["tag_agent"]
	if !exists {
		return diags
	}

	tagAgentStr, ok := tagAgentAttr.(types.String)
	if !ok || tagAgentStr.IsNull() || tagAgentStr.IsUnknown() {
		return diags
	}

	tagAgentValue := tagAgentStr.ValueString()
	validValues := v.providerData.Types.ValidHostTagAgentValues()
	if len(validValues) == 0 {
		return diags
	}

	if !contains(validValues, tagAgentValue) {
		diags.AddAttributeError(
			attrPath.AtMapKey("tag_agent"),
			"Invalid tag_agent Value",
			fmt.Sprintf("Value %q is not valid for tag_agent in CheckMK version %s. "+
				"Valid values are: %s.",
				tagAgentValue, v.providerData.Client.Version.String(), strings.Join(validValues, ", ")),
		)
	}

	return diags
}

// isCustomAttribute returns true if the attribute name indicates a custom attribute.
// Custom attributes include user-defined tags and custom host attributes.
func isCustomAttribute(name string) bool {
	// Standard tag prefixes
	if strings.HasPrefix(name, "tag_") {
		// Built-in tags are validated, custom tags start with tag_ but aren't in the schema
		return true
	}
	// Labels and custom attributes
	if strings.HasPrefix(name, "labels") {
		return true
	}
	return false
}

// formatValidFields formats a list of valid fields for display in error messages.
func formatValidFields(fields []string) string {
	// Sort and limit to first 10 for readability
	sorted := make([]string, len(fields))
	copy(sorted, fields)
	sort.Strings(sorted)

	if len(sorted) > 10 {
		return strings.Join(sorted[:10], ", ") + fmt.Sprintf(" (and %d more)", len(sorted)-10)
	}
	return strings.Join(sorted, ", ")
}

// contains checks if a string slice contains a value.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// VersionedTypesWrapper provides a nil-safe wrapper around client.VersionedTypes
// This is used when provider data might not be fully configured.
type VersionedTypesWrapper struct {
	types *client.VersionedTypes
}

// NewVersionedTypesWrapper creates a new wrapper that's safe to use even with nil types.
func NewVersionedTypesWrapper(types *client.VersionedTypes) *VersionedTypesWrapper {
	return &VersionedTypesWrapper{types: types}
}

// ValidHostTagAgentValues returns valid tag_agent values, or nil if not available.
func (w *VersionedTypesWrapper) ValidHostTagAgentValues() []string {
	if w.types == nil {
		return nil
	}
	return w.types.ValidHostTagAgentValues()
}

// HostCreateAttributeFieldNames returns valid host attribute field names, or nil if not available.
func (w *VersionedTypesWrapper) HostCreateAttributeFieldNames() []string {
	if w.types == nil {
		return nil
	}
	return w.types.HostCreateAttributeFieldNames()
}

// FolderCreateAttributeFieldNames returns valid folder attribute field names, or nil if not available.
func (w *VersionedTypesWrapper) FolderCreateAttributeFieldNames() []string {
	if w.types == nil {
		return nil
	}
	return w.types.FolderCreateAttributeFieldNames()
}

// GetFieldDescription returns the description for a field, or empty string if not available.
func (w *VersionedTypesWrapper) GetFieldDescription(schemaName, fieldName string) string {
	if w.types == nil {
		return ""
	}
	return w.types.GetFieldDescription(schemaName, fieldName)
}

// GetFieldType returns the OpenAPI type for a field, or empty string if not available.
func (w *VersionedTypesWrapper) GetFieldType(schemaName, fieldName string) string {
	if w.types == nil {
		return ""
	}
	return w.types.GetFieldType(schemaName, fieldName)
}

// IsReadOnlyField checks if a field is read-only, returns false if not available.
func (w *VersionedTypesWrapper) IsReadOnlyField(schemaName, fieldName string) bool {
	if w.types == nil {
		return false
	}
	return w.types.IsReadOnlyField(schemaName, fieldName)
}

// IsRequiredField checks if a field is required, returns false if not available.
func (w *VersionedTypesWrapper) IsRequiredField(schemaName, fieldName string) bool {
	if w.types == nil {
		return false
	}
	return w.types.IsRequiredField(schemaName, fieldName)
}
