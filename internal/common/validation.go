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

// ValidateAttributes validates attributes against the generated types for a given schema.
// schemaName is the OpenAPI schema name (e.g., "HostCreateAttribute", "FolderCreateAttribute").
func (v *AttributeValidator) ValidateAttributes(ctx context.Context, schemaName string, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	if attributes.IsNull() || attributes.IsUnknown() {
		return diags
	}

	// Get valid field names for this version and schema
	validFields := v.providerData.Types.GetSchemaFieldNames(schemaName)
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
				"Invalid Attribute",
				fmt.Sprintf("Attribute %q is not valid for %s in CheckMK version %s. "+
					"Valid attributes include: %s. "+
					"If this is a custom attribute, prefix it with 'tag_' or ensure it's defined in CheckMK.",
					key, schemaName, v.providerData.Client.Version.String(), formatValidFields(validFields)),
			)
		}

		// Warn about read-only fields
		if v.providerData.Types.IsReadOnlyField(schemaName, key) {
			diags.AddAttributeWarning(
				attrPath.AtMapKey(key),
				"Read-Only Attribute",
				fmt.Sprintf("Attribute %q is read-only and cannot be set. "+
					"It will be computed by CheckMK and any value provided will be ignored.",
					key),
			)
		}

		// Warn about deprecated fields
		if v.providerData.Types.IsDeprecatedField(schemaName, key) {
			diags.AddAttributeWarning(
				attrPath.AtMapKey(key),
				"Deprecated Attribute",
				fmt.Sprintf("Attribute %q is deprecated and may be removed in a future CheckMK version.",
					key),
			)
		}
	}

	return diags
}

// ValidateHostAttributes validates host attributes against the generated types.
// Returns diagnostics with any validation errors.
func (v *AttributeValidator) ValidateHostAttributes(ctx context.Context, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	diags := v.ValidateAttributes(ctx, "HostCreateAttribute", attributes, attrPath)

	// Validate tag_agent enum values if present
	diags.Append(v.validateEnumField(ctx, "HostCreateAttribute", "tag_agent", attributes, attrPath)...)

	return diags
}

// ValidateFolderAttributes validates folder attributes against the generated types.
func (v *AttributeValidator) ValidateFolderAttributes(ctx context.Context, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	diags := v.ValidateAttributes(ctx, "FolderCreateAttribute", attributes, attrPath)

	// Validate tag_agent enum values if present
	diags.Append(v.validateEnumField(ctx, "FolderCreateAttribute", "tag_agent", attributes, attrPath)...)

	return diags
}

// ValidateRequiredFields validates that all required fields are present in the provided fields map.
// schemaName is the OpenAPI schema name (e.g., "CreateHost", "CreateFolder").
func (v *AttributeValidator) ValidateRequiredFields(schemaName string, providedFields map[string]interface{}, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	// Get required fields for this schema
	requiredFields := v.providerData.Types.GetSchemaRequiredFieldNames(schemaName)

	for _, field := range requiredFields {
		if _, exists := providedFields[field]; !exists {
			diags.AddAttributeError(
				attrPath,
				"Missing Required Field",
				fmt.Sprintf("Field %q is required for %s in CheckMK version %s.",
					field, schemaName, v.providerData.Client.Version.String()),
			)
		}
	}

	return diags
}

// validateEnumField validates an enum field value if present.
func (v *AttributeValidator) validateEnumField(ctx context.Context, schemaName, fieldName string, attributes types.Map, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	elements := attributes.Elements()
	fieldAttr, exists := elements[fieldName]
	if !exists {
		return diags
	}

	fieldStr, ok := fieldAttr.(types.String)
	if !ok || fieldStr.IsNull() || fieldStr.IsUnknown() {
		return diags
	}

	fieldValue := fieldStr.ValueString()

	// Check if this field has enum constraints
	if !v.providerData.Types.HasEnumConstraint(schemaName, fieldName) {
		return diags
	}

	validValues := v.providerData.Types.GetValidEnumValues(schemaName, fieldName)
	if len(validValues) == 0 {
		return diags
	}

	if !contains(validValues, fieldValue) {
		diags.AddAttributeError(
			attrPath.AtMapKey(fieldName),
			fmt.Sprintf("Invalid %s Value", fieldName),
			fmt.Sprintf("Value %q is not valid for %s in CheckMK version %s. "+
				"Valid values are: %s.",
				fieldValue, fieldName, v.providerData.Client.Version.String(), strings.Join(validValues, ", ")),
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

// GetSchemaFieldNames returns field names for a schema, or nil if not available.
func (w *VersionedTypesWrapper) GetSchemaFieldNames(schema string) []string {
	if w.types == nil {
		return nil
	}
	return w.types.GetSchemaFieldNames(schema)
}

// GetSchemaRequiredFieldNames returns required field names for a schema, or nil if not available.
func (w *VersionedTypesWrapper) GetSchemaRequiredFieldNames(schema string) []string {
	if w.types == nil {
		return nil
	}
	return w.types.GetSchemaRequiredFieldNames(schema)
}

// GetValidEnumValues returns valid enum values for a field, or nil if not available.
func (w *VersionedTypesWrapper) GetValidEnumValues(schema, field string) []string {
	if w.types == nil {
		return nil
	}
	return w.types.GetValidEnumValues(schema, field)
}

// HasEnumConstraint returns true if a field has enum constraints.
func (w *VersionedTypesWrapper) HasEnumConstraint(schema, field string) bool {
	if w.types == nil {
		return false
	}
	return w.types.HasEnumConstraint(schema, field)
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

// IsDeprecatedField checks if a field is deprecated, returns false if not available.
func (w *VersionedTypesWrapper) IsDeprecatedField(schemaName, fieldName string) bool {
	if w.types == nil {
		return false
	}
	return w.types.IsDeprecatedField(schemaName, fieldName)
}

// =============================================================================
// Generic Field Validators for Static Schema Resources
// =============================================================================

// ValidateStringField validates a string field against enum constraints if they exist.
// Returns diagnostics with validation errors or deprecation warnings.
func (v *AttributeValidator) ValidateStringField(schemaName, fieldName string, value types.String, attrPath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics

	if !v.ShouldValidate() {
		return diags
	}

	if value.IsNull() || value.IsUnknown() {
		return diags
	}

	fieldValue := value.ValueString()

	// Check for deprecation
	if v.providerData.Types.IsDeprecatedField(schemaName, fieldName) {
		diags.AddAttributeWarning(
			attrPath,
			"Deprecated Field",
			fmt.Sprintf("Field %q is deprecated and may be removed in a future CheckMK version.", fieldName),
		)
	}

	// Check enum constraints
	if v.providerData.Types.HasEnumConstraint(schemaName, fieldName) {
		validValues := v.providerData.Types.GetValidEnumValues(schemaName, fieldName)
		if len(validValues) > 0 && !contains(validValues, fieldValue) {
			diags.AddAttributeError(
				attrPath,
				fmt.Sprintf("Invalid %s Value", fieldName),
				fmt.Sprintf("Value %q is not valid for %s in CheckMK version %s. Valid values are: %s.",
					fieldValue, fieldName, v.providerData.Client.Version.String(), strings.Join(validValues, ", ")),
			)
		}
	}

	return diags
}

// ValidateSchemaExists checks if a schema exists for the current CheckMK version.
// Useful for warning about features not available in older versions.
func (v *AttributeValidator) ValidateSchemaExists(schemaName string) bool {
	if !v.ShouldValidate() {
		return true // Allow if no type info
	}
	return v.providerData.Types.HasSchema(schemaName)
}

// GetVersionString returns the CheckMK version string for error messages.
func (v *AttributeValidator) GetVersionString() string {
	if v.providerData == nil || v.providerData.Client == nil {
		return "unknown"
	}
	return v.providerData.Client.Version.String()
}
