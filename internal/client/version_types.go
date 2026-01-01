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
