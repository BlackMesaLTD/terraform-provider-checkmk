// Package client provides version-aware type selection for CheckMK API types.
//
// This file wraps the generated types from the checkmk-api-spec companion
// repository, providing runtime selection based on the connected CheckMK version.
package client

import (
	types "github.com/BlackMesaLTD/checkmk-api-spec/generated/go"
)

// VersionedTypes provides version-specific type information.
// Used when type_mode = "auto" or "strict" for pre-validation against known CheckMK API schemas.
type VersionedTypes struct {
	Version  string
	Baseline types.BaselinePackage
}

// NewVersionedTypes creates a VersionedTypes instance for the given CheckMK version.
// Returns nil if the version cannot be mapped to a baseline.
func NewVersionedTypes(version string) *VersionedTypes {
	baseline := types.LookupBaseline(version)
	if baseline == "" {
		return nil
	}
	return &VersionedTypes{
		Version:  version,
		Baseline: baseline,
	}
}

// SupportsVersion returns true if generated types are available for this version.
func (v *VersionedTypes) SupportsVersion() bool {
	return v != nil && v.Baseline != ""
}

// BaselineVersion returns the baseline version string (e.g., "v2_4_0_p17").
func (v *VersionedTypes) BaselineVersion() string {
	if v == nil {
		return ""
	}
	return string(v.Baseline)
}

// LookupBaseline is a convenience wrapper around types.LookupBaseline.
// It returns the baseline package identifier for a given CheckMK version.
func LookupBaseline(version string) string {
	return string(types.LookupBaseline(version))
}

// IsVersionSupported returns true if the given version has a known baseline mapping.
func IsVersionSupported(version string) bool {
	return types.LookupBaseline(version) != ""
}

// =============================================================================
// Generic Schema Introspection API
// =============================================================================

// GetAllSchemaNames returns all schema names available in this baseline.
// Returns 700-900 schemas depending on version.
func (v *VersionedTypes) GetAllSchemaNames() []string {
	if v == nil {
		return nil
	}
	return types.GetAllSchemaNames(v.Baseline)
}

// GetSchemaFieldNames returns the field names for a given schema.
// Works with any of the 700-900 schemas in the OpenAPI spec.
func (v *VersionedTypes) GetSchemaFieldNames(schema string) []string {
	if v == nil {
		return nil
	}
	return types.GetSchemaFieldNames(v.Baseline, schema)
}

// GetSchemaRequiredFieldNames returns the required field names for a given schema.
func (v *VersionedTypes) GetSchemaRequiredFieldNames(schema string) []string {
	if v == nil {
		return nil
	}
	return types.GetSchemaRequiredFieldNames(v.Baseline, schema)
}

// HasSchema returns true if the schema exists in this baseline.
func (v *VersionedTypes) HasSchema(schema string) bool {
	if v == nil {
		return false
	}
	return types.HasSchema(v.Baseline, schema)
}

// GetValidEnumValues returns valid enum values for a field in a schema.
// Returns nil if the field doesn't have enum constraints.
func (v *VersionedTypes) GetValidEnumValues(schema, field string) []string {
	if v == nil {
		return nil
	}
	return types.GetValidEnumValues(v.Baseline, schema, field)
}

// HasEnumConstraint returns true if a field has enum constraints.
func (v *VersionedTypes) HasEnumConstraint(schema, field string) bool {
	if v == nil {
		return false
	}
	return types.HasEnumConstraint(v.Baseline, schema, field)
}

// =============================================================================
// Field Metadata API
// =============================================================================

// GetFieldDescription returns the description for a field in a schema.
// Returns empty string if not available.
func (v *VersionedTypes) GetFieldDescription(schemaName, fieldName string) string {
	if v == nil {
		return ""
	}
	return types.GetFieldDescription(v.Baseline, schemaName, fieldName)
}

// GetFieldType returns the OpenAPI type for a field in a schema.
// Returns empty string if not available.
func (v *VersionedTypes) GetFieldType(schemaName, fieldName string) string {
	if v == nil {
		return ""
	}
	return types.GetFieldType(v.Baseline, schemaName, fieldName)
}

// IsReadOnlyField checks if a field is read-only for a given schema.
func (v *VersionedTypes) IsReadOnlyField(schemaName, fieldName string) bool {
	if v == nil {
		return false
	}
	return types.IsReadOnlyField(v.Baseline, schemaName, fieldName)
}

// IsRequiredField checks if a field is required for a given schema.
func (v *VersionedTypes) IsRequiredField(schemaName, fieldName string) bool {
	if v == nil {
		return false
	}
	return types.IsRequiredField(v.Baseline, schemaName, fieldName)
}

// IsDeprecatedField checks if a field is deprecated for a given schema.
func (v *VersionedTypes) IsDeprecatedField(schemaName, fieldName string) bool {
	if v == nil {
		return false
	}
	return types.IsDeprecatedField(v.Baseline, schemaName, fieldName)
}

// =============================================================================
// Validation Helpers
// =============================================================================

// ValidateFieldName checks if a field name is valid for a given schema.
func (v *VersionedTypes) ValidateFieldName(schema, field string) bool {
	if v == nil {
		return true // Allow if no type info
	}
	fields := v.GetSchemaFieldNames(schema)
	if fields == nil {
		return true // Allow if schema not found
	}
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}

// ValidateEnumValue checks if a value is valid for an enum field.
func (v *VersionedTypes) ValidateEnumValue(schema, field, value string) bool {
	if v == nil {
		return true // Allow if no type info
	}
	if !v.HasEnumConstraint(schema, field) {
		return true // Allow if not an enum
	}
	validValues := v.GetValidEnumValues(schema, field)
	for _, valid := range validValues {
		if valid == value {
			return true
		}
	}
	return false
}
