// Package common provides shared utilities for the CheckMK Terraform provider.
package common

import (
	"github.com/BlackMesaLTD/checkmk-api-spec/generated/go/union"
)

// Desc returns the union description for a schema field.
// This provides version-annotated descriptions for documentation.
// Returns empty string if not found.
func Desc(schemaName, fieldName string) string {
	return union.GetUnionDescription(schemaName, fieldName)
}

// DescMarkdown returns the formatted markdown description for a schema field.
// Includes version annotations like "Available in CheckMK 2.3+".
// Returns empty string if not found.
func DescMarkdown(schemaName, fieldName string) string {
	field := union.GetUnionField(schemaName, fieldName)
	if field == nil {
		return ""
	}
	return field.FormatMarkdown()
}

// FieldInfo returns the full union field metadata for a schema field.
// Returns nil if not found.
func FieldInfo(schemaName, fieldName string) *union.UnionField {
	return union.GetUnionField(schemaName, fieldName)
}
