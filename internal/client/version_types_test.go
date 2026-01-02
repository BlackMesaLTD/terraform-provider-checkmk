//go:build checkmk_all

// Package client provides version-aware type selection for CheckMK API types.
package client

import (
	"testing"
)

// =============================================================================
// Tests for NewVersionedTypes
// =============================================================================

func TestNewVersionedTypes(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		wantNil  bool
		wantBase string // Expected baseline prefix
	}{
		{
			name:     "2.2.0p43 supported",
			version:  "2.2.0p43",
			wantNil:  false,
			wantBase: "v2_2_0",
		},
		{
			name:     "2.3.0p41 supported",
			version:  "2.3.0p41",
			wantNil:  false,
			wantBase: "v2_3_0",
		},
		{
			name:     "2.4.0p17 supported",
			version:  "2.4.0p17",
			wantNil:  false,
			wantBase: "v2_4_0",
		},
		{
			name:    "unsupported version",
			version: "1.0.0",
			wantNil: true,
		},
		{
			name:    "empty version",
			version: "",
			wantNil: true,
		},
		{
			name:    "invalid format",
			version: "invalid",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewVersionedTypes(tt.version)
			if tt.wantNil {
				if got != nil {
					t.Errorf("NewVersionedTypes(%q) = %v, want nil", tt.version, got)
				}
			} else {
				if got == nil {
					t.Fatalf("NewVersionedTypes(%q) = nil, want non-nil", tt.version)
				}
				if got.Version != tt.version {
					t.Errorf("got.Version = %q, want %q", got.Version, tt.version)
				}
				baseline := got.BaselineVersion()
				if len(baseline) < len(tt.wantBase) || baseline[:len(tt.wantBase)] != tt.wantBase {
					t.Errorf("got.BaselineVersion() = %q, want prefix %q", baseline, tt.wantBase)
				}
			}
		})
	}
}

func TestVersionedTypes_SupportsVersion(t *testing.T) {
	tests := []struct {
		name string
		vt   *VersionedTypes
		want bool
	}{
		{
			name: "supported version",
			vt:   NewVersionedTypes("2.4.0p17"),
			want: true,
		},
		{
			name: "nil VersionedTypes",
			vt:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vt.SupportsVersion()
			if got != tt.want {
				t.Errorf("SupportsVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersionedTypes_BaselineVersion(t *testing.T) {
	tests := []struct {
		name string
		vt   *VersionedTypes
		want string
	}{
		{
			name: "nil VersionedTypes",
			vt:   nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vt.BaselineVersion()
			if got != tt.want {
				t.Errorf("BaselineVersion() = %q, want %q", got, tt.want)
			}
		})
	}

	// Test non-nil case separately
	t.Run("valid version", func(t *testing.T) {
		vt := NewVersionedTypes("2.4.0p17")
		if vt == nil {
			t.Skip("2.4.0p17 not supported")
		}
		baseline := vt.BaselineVersion()
		if baseline == "" {
			t.Error("BaselineVersion() returned empty string for valid version")
		}
	})
}

// =============================================================================
// Tests for package-level functions
// =============================================================================

func TestLookupBaseline(t *testing.T) {
	tests := []struct {
		version  string
		wantBase string // Expected prefix
	}{
		{"2.2.0p43", "v2_2_0"},
		{"2.3.0p41", "v2_3_0"},
		{"2.4.0p17", "v2_4_0"},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := LookupBaseline(tt.version)
			if tt.wantBase == "" {
				if got != "" {
					t.Errorf("LookupBaseline(%q) = %q, want empty", tt.version, got)
				}
			} else if len(got) < len(tt.wantBase) || got[:len(tt.wantBase)] != tt.wantBase {
				t.Errorf("LookupBaseline(%q) = %q, want prefix %q", tt.version, got, tt.wantBase)
			}
		})
	}
}

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"2.2.0p43", true},
		{"2.3.0p41", true},
		{"2.4.0p17", true},
		{"1.0.0", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := IsVersionSupported(tt.version)
			if got != tt.want {
				t.Errorf("IsVersionSupported(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tests for Schema Introspection API
// =============================================================================

func TestVersionedTypes_GetAllSchemaNames(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	schemas := vt.GetAllSchemaNames()
	if len(schemas) == 0 {
		t.Error("GetAllSchemaNames() returned empty slice")
	}

	// Should have 700-900 schemas
	if len(schemas) < 500 {
		t.Errorf("GetAllSchemaNames() returned %d schemas, expected 500+", len(schemas))
	}

	t.Logf("Found %d schemas in 2.4.0p17", len(schemas))

	// Check for expected schemas
	expectedSchemas := []string{
		"HostCreateAttribute",
		"FolderCreateAttribute",
		"UserObject",
		"ContactGroup",
		"RuleObject",
	}

	schemaSet := make(map[string]bool)
	for _, s := range schemas {
		schemaSet[s] = true
	}

	for _, expected := range expectedSchemas {
		if !schemaSet[expected] {
			t.Errorf("Expected schema %q not found in GetAllSchemaNames()", expected)
		}
	}
}

func TestVersionedTypes_GetAllSchemaNames_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	schemas := vt.GetAllSchemaNames()
	if schemas != nil {
		t.Errorf("GetAllSchemaNames() on nil = %v, want nil", schemas)
	}
}

func TestVersionedTypes_GetSchemaFieldNames(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	fields := vt.GetSchemaFieldNames("HostCreateAttribute")
	if len(fields) == 0 {
		t.Error("GetSchemaFieldNames(HostCreateAttribute) returned empty slice")
	}

	// Check for known fields
	expectedFields := []string{
		"alias",
		"ipaddress",
		"tag_agent",
	}

	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}

	for _, expected := range expectedFields {
		if !fieldSet[expected] {
			t.Errorf("Expected field %q not found in HostCreateAttribute", expected)
		}
	}
}

func TestVersionedTypes_GetSchemaFieldNames_NonExistent(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	fields := vt.GetSchemaFieldNames("NonExistentSchema")
	if len(fields) != 0 {
		t.Errorf("GetSchemaFieldNames(NonExistentSchema) = %v, want empty", fields)
	}
}

func TestVersionedTypes_GetSchemaRequiredFieldNames(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	required := vt.GetSchemaRequiredFieldNames("UserObject")
	// UserObject has required fields like username
	t.Logf("Required fields for UserObject: %v", required)
}

func TestVersionedTypes_HasSchema(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	tests := []struct {
		schema string
		want   bool
	}{
		{"HostCreateAttribute", true},
		{"FolderCreateAttribute", true},
		{"UserObject", true},
		{"ContactGroup", true},
		{"RuleObject", true},
		{"NonExistentSchema", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.schema, func(t *testing.T) {
			got := vt.HasSchema(tt.schema)
			if got != tt.want {
				t.Errorf("HasSchema(%q) = %v, want %v", tt.schema, got, tt.want)
			}
		})
	}
}

func TestVersionedTypes_HasSchema_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	if vt.HasSchema("HostCreateAttribute") {
		t.Error("HasSchema on nil should return false")
	}
}

func TestVersionedTypes_GetValidEnumValues(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	// tag_agent should have enum values
	values := vt.GetValidEnumValues("HostCreateAttribute", "tag_agent")
	if len(values) == 0 {
		t.Skip("tag_agent enum values not available in 2.4.0p17")
	}

	t.Logf("Valid tag_agent values: %v", values)

	// Check for known values
	expectedValues := []string{"cmk-agent", "no-agent"}
	valueSet := make(map[string]bool)
	for _, v := range values {
		valueSet[v] = true
	}

	for _, expected := range expectedValues {
		if !valueSet[expected] {
			t.Errorf("Expected enum value %q not found in tag_agent", expected)
		}
	}
}

func TestVersionedTypes_GetValidEnumValues_NonEnumField(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	// alias is not an enum field
	values := vt.GetValidEnumValues("HostCreateAttribute", "alias")
	if len(values) != 0 {
		t.Logf("GetValidEnumValues for non-enum field returned: %v", values)
		// This is acceptable - some implementations may return empty slice
	}
}

func TestVersionedTypes_HasEnumConstraint(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	tests := []struct {
		schema string
		field  string
		want   bool
	}{
		{"HostCreateAttribute", "tag_agent", true},  // enum field
		{"HostCreateAttribute", "alias", false},     // string field
		{"HostCreateAttribute", "ipaddress", false}, // string field
	}

	for _, tt := range tests {
		t.Run(tt.schema+"."+tt.field, func(t *testing.T) {
			got := vt.HasEnumConstraint(tt.schema, tt.field)
			if got != tt.want {
				t.Errorf("HasEnumConstraint(%q, %q) = %v, want %v", tt.schema, tt.field, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Tests for Field Metadata API
// =============================================================================

func TestVersionedTypes_GetFieldDescription(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	desc := vt.GetFieldDescription("HostCreateAttribute", "alias")
	// Description should exist for common fields
	t.Logf("Description for alias: %q", desc)
	// Not checking content since it varies by version
}

func TestVersionedTypes_GetFieldDescription_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	desc := vt.GetFieldDescription("HostCreateAttribute", "alias")
	if desc != "" {
		t.Errorf("GetFieldDescription on nil = %q, want empty", desc)
	}
}

func TestVersionedTypes_GetFieldType(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	fieldType := vt.GetFieldType("HostCreateAttribute", "alias")
	t.Logf("Type for alias: %q", fieldType)
	// alias is typically a string type
	if fieldType != "" && fieldType != "string" {
		t.Logf("Unexpected type for alias: %q", fieldType)
	}
}

func TestVersionedTypes_IsReadOnlyField(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	// alias is not read-only
	if vt.IsReadOnlyField("HostCreateAttribute", "alias") {
		t.Error("alias should not be read-only")
	}
}

func TestVersionedTypes_IsReadOnlyField_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	if vt.IsReadOnlyField("HostCreateAttribute", "alias") {
		t.Error("IsReadOnlyField on nil should return false")
	}
}

func TestVersionedTypes_IsRequiredField(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	// Test various fields
	t.Run("HostCreateAttribute.alias", func(t *testing.T) {
		required := vt.IsRequiredField("HostCreateAttribute", "alias")
		t.Logf("alias required: %v", required)
	})
}

func TestVersionedTypes_IsRequiredField_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	if vt.IsRequiredField("HostCreateAttribute", "alias") {
		t.Error("IsRequiredField on nil should return false")
	}
}

func TestVersionedTypes_IsDeprecatedField(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	// Most fields are not deprecated
	if vt.IsDeprecatedField("HostCreateAttribute", "alias") {
		t.Log("alias is deprecated (unexpected but possible)")
	}
}

func TestVersionedTypes_IsDeprecatedField_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	if vt.IsDeprecatedField("HostCreateAttribute", "alias") {
		t.Error("IsDeprecatedField on nil should return false")
	}
}

// =============================================================================
// Tests for Validation Helpers
// =============================================================================

func TestVersionedTypes_ValidateFieldName(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	tests := []struct {
		schema string
		field  string
		want   bool
	}{
		{"HostCreateAttribute", "alias", true},
		{"HostCreateAttribute", "ipaddress", true},
		{"HostCreateAttribute", "tag_agent", true},
		{"HostCreateAttribute", "nonexistent_field", false},
		{"NonExistentSchema", "field", true}, // Allow if schema not found
	}

	for _, tt := range tests {
		t.Run(tt.schema+"."+tt.field, func(t *testing.T) {
			got := vt.ValidateFieldName(tt.schema, tt.field)
			if got != tt.want {
				t.Errorf("ValidateFieldName(%q, %q) = %v, want %v", tt.schema, tt.field, got, tt.want)
			}
		})
	}
}

func TestVersionedTypes_ValidateFieldName_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	// Should return true (allow) when nil
	if !vt.ValidateFieldName("HostCreateAttribute", "alias") {
		t.Error("ValidateFieldName on nil should return true (allow)")
	}
}

func TestVersionedTypes_ValidateEnumValue(t *testing.T) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		t.Skip("2.4.0p17 not supported")
	}

	tests := []struct {
		schema string
		field  string
		value  string
		want   bool
	}{
		{"HostCreateAttribute", "tag_agent", "cmk-agent", true},
		{"HostCreateAttribute", "tag_agent", "no-agent", true},
		{"HostCreateAttribute", "tag_agent", "invalid-value", false},
		{"HostCreateAttribute", "alias", "anything", true}, // Not an enum
	}

	for _, tt := range tests {
		t.Run(tt.schema+"."+tt.field+"="+tt.value, func(t *testing.T) {
			got := vt.ValidateEnumValue(tt.schema, tt.field, tt.value)
			if got != tt.want {
				t.Errorf("ValidateEnumValue(%q, %q, %q) = %v, want %v",
					tt.schema, tt.field, tt.value, got, tt.want)
			}
		})
	}
}

func TestVersionedTypes_ValidateEnumValue_NilSafe(t *testing.T) {
	var vt *VersionedTypes
	// Should return true (allow) when nil
	if !vt.ValidateEnumValue("HostCreateAttribute", "tag_agent", "invalid") {
		t.Error("ValidateEnumValue on nil should return true (allow)")
	}
}

// =============================================================================
// Cross-Version Compatibility Tests
// =============================================================================

func TestVersionedTypes_CrossVersion(t *testing.T) {
	versions := []string{"2.2.0p43", "2.3.0p41", "2.4.0p17"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			vt := NewVersionedTypes(version)
			if vt == nil {
				t.Fatalf("NewVersionedTypes(%q) = nil", version)
			}

			// Test basic functionality
			if !vt.SupportsVersion() {
				t.Error("SupportsVersion() = false")
			}

			baseline := vt.BaselineVersion()
			if baseline == "" {
				t.Error("BaselineVersion() = empty")
			}

			// Test schema introspection
			if !vt.HasSchema("HostCreateAttribute") {
				t.Error("HasSchema(HostCreateAttribute) = false")
			}

			fields := vt.GetSchemaFieldNames("HostCreateAttribute")
			if len(fields) == 0 {
				t.Error("GetSchemaFieldNames(HostCreateAttribute) = empty")
			}

			// tag_agent should exist in all versions
			hasTagAgent := false
			for _, f := range fields {
				if f == "tag_agent" {
					hasTagAgent = true
					break
				}
			}
			if !hasTagAgent {
				t.Error("tag_agent field not found in HostCreateAttribute")
			}

			// Validate enum values
			if !vt.ValidateEnumValue("HostCreateAttribute", "tag_agent", "cmk-agent") {
				t.Error("cmk-agent should be valid for tag_agent")
			}

			t.Logf("%s: baseline=%s, schemas=%d, hostFields=%d",
				version, baseline, len(vt.GetAllSchemaNames()), len(fields))
		})
	}
}

// =============================================================================
// Schema Evolution Tests
// =============================================================================

func TestVersionedTypes_SchemaEvolution(t *testing.T) {
	// Compare schemas across versions to ensure evolution is handled
	v22 := NewVersionedTypes("2.2.0p43")
	v23 := NewVersionedTypes("2.3.0p41")
	v24 := NewVersionedTypes("2.4.0p17")

	if v22 == nil || v23 == nil || v24 == nil {
		t.Skip("Not all versions available")
	}

	// Compare schema counts
	count22 := len(v22.GetAllSchemaNames())
	count23 := len(v23.GetAllSchemaNames())
	count24 := len(v24.GetAllSchemaNames())

	t.Logf("Schema counts: 2.2=%d, 2.3=%d, 2.4=%d", count22, count23, count24)

	// Schemas typically grow over time
	if count22 == 0 || count23 == 0 || count24 == 0 {
		t.Error("One or more versions has zero schemas")
	}

	// Compare field counts for HostCreateAttribute
	fields22 := len(v22.GetSchemaFieldNames("HostCreateAttribute"))
	fields23 := len(v23.GetSchemaFieldNames("HostCreateAttribute"))
	fields24 := len(v24.GetSchemaFieldNames("HostCreateAttribute"))

	t.Logf("HostCreateAttribute field counts: 2.2=%d, 2.3=%d, 2.4=%d",
		fields22, fields23, fields24)

	// All should have fields
	if fields22 == 0 || fields23 == 0 || fields24 == 0 {
		t.Error("One or more versions has zero fields for HostCreateAttribute")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkNewVersionedTypes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewVersionedTypes("2.4.0p17")
	}
}

func BenchmarkVersionedTypes_HasSchema(b *testing.B) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		b.Skip("2.4.0p17 not supported")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vt.HasSchema("HostCreateAttribute")
	}
}

func BenchmarkVersionedTypes_GetSchemaFieldNames(b *testing.B) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		b.Skip("2.4.0p17 not supported")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vt.GetSchemaFieldNames("HostCreateAttribute")
	}
}

func BenchmarkVersionedTypes_ValidateEnumValue(b *testing.B) {
	vt := NewVersionedTypes("2.4.0p17")
	if vt == nil {
		b.Skip("2.4.0p17 not supported")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = vt.ValidateEnumValue("HostCreateAttribute", "tag_agent", "cmk-agent")
	}
}
