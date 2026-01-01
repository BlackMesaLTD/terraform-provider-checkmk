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

// BaselineVersion returns the baseline version string (e.g., "v2_4_0p17").
func (v *VersionedTypes) BaselineVersion() string {
	if v == nil {
		return ""
	}
	return string(v.Baseline)
}

// Host Attribute Validators

// ValidHostTagAgentValues returns valid tag_agent values for the connected CheckMK version.
// Returns nil if the version is not recognized.
func (v *VersionedTypes) ValidHostTagAgentValues() []string {
	if v == nil {
		return nil
	}
	return types.ValidHostTagAgentValues(v.Baseline)
}

// Field Name Lists

// HostCreateAttributeFieldNames returns valid host attribute field names for the version.
func (v *VersionedTypes) HostCreateAttributeFieldNames() []string {
	if v == nil {
		return nil
	}
	return types.HostCreateAttributeFieldNames(v.Baseline)
}

// FolderCreateAttributeFieldNames returns valid folder attribute field names.
func (v *VersionedTypes) FolderCreateAttributeFieldNames() []string {
	if v == nil {
		return nil
	}
	return types.FolderCreateAttributeFieldNames(v.Baseline)
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

// Metadata Accessors

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

// Compare Key Fields

// HostCreateAttributeCompareKeyFields returns fields used for comparison/hashing.
func (v *VersionedTypes) HostCreateAttributeCompareKeyFields() []string {
	if v == nil {
		return nil
	}
	return types.HostCreateAttributeCompareKeyFields(v.Baseline)
}

// FolderCreateAttributeCompareKeyFields returns fields used for comparison/hashing.
func (v *VersionedTypes) FolderCreateAttributeCompareKeyFields() []string {
	if v == nil {
		return nil
	}
	return types.FolderCreateAttributeCompareKeyFields(v.Baseline)
}

// Request Builders

// BuildCreateHostFromMap creates a typed CreateHost request from a map.
func (v *VersionedTypes) BuildCreateHostFromMap(data map[string]interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	return types.BuildCreateHostFromMap(v.Baseline, data)
}

// BuildCreateFolderFromMap creates a typed CreateFolder request from a map.
func (v *VersionedTypes) BuildCreateFolderFromMap(data map[string]interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	return types.BuildCreateFolderFromMap(v.Baseline, data)
}

// Response Parsers

// ParseHostConfigFromMap parses a map into a typed HostConfig response.
func (v *VersionedTypes) ParseHostConfigFromMap(data map[string]interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	return types.ParseHostConfigFromMap(v.Baseline, data)
}

// ParseFolderFromMap parses a map into a typed Folder response.
func (v *VersionedTypes) ParseFolderFromMap(data map[string]interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	return types.ParseFolderFromMap(v.Baseline, data)
}

// Import State Mapping

// ExtractHostConfigField extracts a Terraform field value from a HostConfig API response.
func (v *VersionedTypes) ExtractHostConfigField(response map[string]interface{}, tfField string) interface{} {
	if v == nil {
		return nil
	}
	return types.ExtractHostConfigField(v.Baseline, response, tfField)
}

// ExtractFolderField extracts a Terraform field value from a Folder API response.
func (v *VersionedTypes) ExtractFolderField(response map[string]interface{}, tfField string) interface{} {
	if v == nil {
		return nil
	}
	return types.ExtractFolderField(v.Baseline, response, tfField)
}

// HostConfigFieldMappings returns the field mappings for HostConfig.
func (v *VersionedTypes) HostConfigFieldMappings() map[string][]string {
	if v == nil {
		return nil
	}
	return types.HostConfigFieldMappings(v.Baseline)
}

// FolderFieldMappings returns the field mappings for Folder.
func (v *VersionedTypes) FolderFieldMappings() map[string][]string {
	if v == nil {
		return nil
	}
	return types.FolderFieldMappings(v.Baseline)
}

// Deprecation Warnings

// IsDeprecatedField checks if a field is deprecated for a given schema.
func (v *VersionedTypes) IsDeprecatedField(schemaName, fieldName string) bool {
	if v == nil {
		return false
	}
	return types.IsDeprecatedField(v.Baseline, schemaName, fieldName)
}
