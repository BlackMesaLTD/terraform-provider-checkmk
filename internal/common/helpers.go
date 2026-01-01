package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
)

// ConfigureResource extracts provider data and returns it, or adds an error to diagnostics.
// Returns nil if provider data is not available or has wrong type.
func ConfigureResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *ProviderData {
	if req.ProviderData == nil {
		return nil
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return nil
	}

	return providerData
}

// HandleNotFoundOnRead checks if an error is a 404 API error and removes the resource from state.
// Returns true if the resource was removed (caller should return early).
func HandleNotFoundOnRead(err error, resp *resource.ReadResponse) bool {
	if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
		resp.State.RemoveResource(context.Background())
		return true
	}
	return false
}

// HandleDriftWarning checks if an error is a drift error and adds a warning.
// Returns true if drift was detected (caller may continue to reconcile).
func HandleDriftWarning(err error, resp *resource.UpdateResponse, resourceName string) bool {
	if driftErr, ok := err.(*client.DriftError); ok {
		resp.Diagnostics.AddWarning(
			"Configuration Drift Detected",
			fmt.Sprintf("%s '%s' was modified outside of Terraform: %s. "+
				"Terraform will attempt to reconcile the configuration.", driftErr.Resource, resourceName, driftErr.Message),
		)
		return true
	}
	return false
}

// ActivateIfAuto activates changes if the activation mode is "auto".
func ActivateIfAuto(ctx context.Context, c *client.Client, cfg BaseResourceConfig, operation string) error {
	if cfg.Activate == "auto" {
		return c.ActivateChanges(ctx, nil, cfg.ForceForeignChanges, cfg.StrictResourceLocking, cfg.ActivationWaitTime)
	}
	return nil
}

// TrackAndActivate tracks a change for the given resource type and activates if mode is "auto".
// This should be called after a successful create, update, or delete operation.
// The resourceType is used for reporting (e.g., "folder", "host", "rule").
func TrackAndActivate(ctx context.Context, pd *ProviderData, cfg BaseResourceConfig, resourceType string) error {
	// Always track the change for batch activation reporting
	pd.TrackChange(resourceType)

	// Activate immediately if in auto mode
	if cfg.Activate == "auto" {
		return pd.Client.ActivateChanges(ctx, nil, cfg.ForceForeignChanges, cfg.StrictResourceLocking, cfg.ActivationWaitTime)
	}
	return nil
}

// AddActivationWarning adds a standardized activation warning to diagnostics
func AddActivationWarning(resp interface{}, resourceType, operation string, err error) {
	warning := fmt.Sprintf("%s was %s but activation failed: %s", resourceType, operation, err)

	switch r := resp.(type) {
	case *resource.CreateResponse:
		r.Diagnostics.AddWarning("Activation Warning", warning)
	case *resource.UpdateResponse:
		r.Diagnostics.AddWarning("Activation Warning", warning)
	case *resource.DeleteResponse:
		r.Diagnostics.AddWarning("Activation Warning", warning)
	}
}

// ETagFetcher is a function type for fetching ETags
type ETagFetcher func(ctx context.Context) (string, error)

// FetchETagIfStrict fetches the ETag for a resource if strict resource locking is enabled.
func FetchETagIfStrict(ctx context.Context, strictResourceLocking bool, fetcher ETagFetcher) (string, error) {
	if !strictResourceLocking {
		return "", nil
	}
	return fetcher(ctx)
}

// GetEffectiveStrictLocking returns the effective strict_resource_locking setting
func GetEffectiveStrictLocking(providerDefault bool, resourceOverride types.Bool) bool {
	if !resourceOverride.IsNull() {
		return resourceOverride.ValueBool()
	}
	return providerDefault
}

// GetEffectiveActivate returns the effective activate setting
func GetEffectiveActivate(providerDefault string, resourceOverride types.String) string {
	if !resourceOverride.IsNull() {
		return resourceOverride.ValueString()
	}
	return providerDefault
}

// GetEffectiveForceForeignChanges returns the effective force_foreign_changes setting
func GetEffectiveForceForeignChanges(providerDefault bool, resourceOverride types.Bool) bool {
	if !resourceOverride.IsNull() {
		return resourceOverride.ValueBool()
	}
	return providerDefault
}

// BuildBaseConfig creates a BaseResourceConfig from provider data and resource-level overrides
func BuildBaseConfig(pd *ProviderData, activateOverride types.String, forceForeignOverride types.Bool, strictLockingOverride types.Bool) BaseResourceConfig {
	return BaseResourceConfig{
		Client:                pd.Client,
		Activate:              GetEffectiveActivate(pd.Activate, activateOverride),
		ForceForeignChanges:   GetEffectiveForceForeignChanges(pd.ForceForeignChanges, forceForeignOverride),
		ActivationWaitTime:    pd.ActivationWaitTime,
		StrictResourceLocking: GetEffectiveStrictLocking(pd.StrictResourceLocking, strictLockingOverride),
	}
}

// BuildSimpleBaseConfig creates a BaseResourceConfig for resources that only support strict_resource_locking override
func BuildSimpleBaseConfig(pd *ProviderData, strictLockingOverride types.Bool) BaseResourceConfig {
	return BaseResourceConfig{
		Client:                pd.Client,
		Activate:              pd.Activate,
		ForceForeignChanges:   pd.ForceForeignChanges,
		ActivationWaitTime:    pd.ActivationWaitTime,
		StrictResourceLocking: GetEffectiveStrictLocking(pd.StrictResourceLocking, strictLockingOverride),
	}
}

// =============================================================================
// Type Conversion Helpers
// =============================================================================
// These helpers reduce boilerplate for extracting values from Terraform types
// with proper null/unknown handling.

// GetStringValue returns the string value or empty string if null/unknown.
func GetStringValue(val types.String) string {
	if val.IsNull() || val.IsUnknown() {
		return ""
	}
	return val.ValueString()
}

// GetStringValueOr returns the string value or the default if null/unknown.
func GetStringValueOr(val types.String, defaultVal string) string {
	if val.IsNull() || val.IsUnknown() {
		return defaultVal
	}
	return val.ValueString()
}

// GetOptionalString returns the optional value if set, otherwise the required value.
// Useful for fields like Title that fall back to Name if not specified.
func GetOptionalString(required, optional types.String) string {
	if !optional.IsNull() && !optional.IsUnknown() {
		return optional.ValueString()
	}
	return required.ValueString()
}

// GetBoolValue returns the bool value or false if null/unknown.
func GetBoolValue(val types.Bool) bool {
	if val.IsNull() || val.IsUnknown() {
		return false
	}
	return val.ValueBool()
}

// GetBoolValueOr returns the bool value or the default if null/unknown.
func GetBoolValueOr(val types.Bool, defaultVal bool) bool {
	if val.IsNull() || val.IsUnknown() {
		return defaultVal
	}
	return val.ValueBool()
}

// GetInt64Value returns the int64 value or 0 if null/unknown.
func GetInt64Value(val types.Int64) int64 {
	if val.IsNull() || val.IsUnknown() {
		return 0
	}
	return val.ValueInt64()
}

// GetInt64ValueOr returns the int64 value or the default if null/unknown.
func GetInt64ValueOr(val types.Int64, defaultVal int64) int64 {
	if val.IsNull() || val.IsUnknown() {
		return defaultVal
	}
	return val.ValueInt64()
}

// =============================================================================
// Type Coercion Helpers (API → Terraform)
// =============================================================================
// These helpers convert API response values to Terraform types using the
// generated field type metadata for proper type handling.

// TypeCoercer provides type coercion using version-specific field metadata.
type TypeCoercer struct {
	types *client.VersionedTypes
}

// NewTypeCoercer creates a new TypeCoercer with the given versioned types.
// Returns nil if types is nil.
func NewTypeCoercer(vt *client.VersionedTypes) *TypeCoercer {
	if vt == nil {
		return nil
	}
	return &TypeCoercer{types: vt}
}

// CoerceToString converts an API value to a Terraform string value.
// Handles nil, string, int, float, and bool inputs.
func (c *TypeCoercer) CoerceToString(value interface{}) types.String {
	if value == nil {
		return types.StringNull()
	}
	switch v := value.(type) {
	case string:
		return types.StringValue(v)
	case int:
		return types.StringValue(fmt.Sprintf("%d", v))
	case int64:
		return types.StringValue(fmt.Sprintf("%d", v))
	case float64:
		// Check if it's actually an integer
		if v == float64(int64(v)) {
			return types.StringValue(fmt.Sprintf("%d", int64(v)))
		}
		return types.StringValue(fmt.Sprintf("%g", v))
	case bool:
		if v {
			return types.StringValue("true")
		}
		return types.StringValue("false")
	default:
		return types.StringValue(fmt.Sprintf("%v", value))
	}
}

// CoerceToBool converts an API value to a Terraform bool value.
func (c *TypeCoercer) CoerceToBool(value interface{}) types.Bool {
	if value == nil {
		return types.BoolNull()
	}
	switch v := value.(type) {
	case bool:
		return types.BoolValue(v)
	case string:
		return types.BoolValue(v == "true" || v == "1" || v == "yes")
	case int, int64:
		return types.BoolValue(v != 0)
	case float64:
		return types.BoolValue(v != 0)
	default:
		return types.BoolNull()
	}
}

// CoerceToInt64 converts an API value to a Terraform int64 value.
func (c *TypeCoercer) CoerceToInt64(value interface{}) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	switch v := value.(type) {
	case int:
		return types.Int64Value(int64(v))
	case int64:
		return types.Int64Value(v)
	case float64:
		return types.Int64Value(int64(v))
	case string:
		var i int64
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return types.Int64Value(i)
		}
		return types.Int64Null()
	default:
		return types.Int64Null()
	}
}

// CoerceValue converts an API value to the appropriate Terraform type based on
// the OpenAPI field type from the generated metadata.
func (c *TypeCoercer) CoerceValue(schemaName, fieldName string, value interface{}) interface{} {
	if c == nil || c.types == nil {
		// No type info available, return as-is
		return value
	}

	fieldType := c.types.GetFieldType(schemaName, fieldName)
	switch fieldType {
	case "string":
		return c.CoerceToString(value)
	case "boolean":
		return c.CoerceToBool(value)
	case "integer":
		return c.CoerceToInt64(value)
	default:
		// For arrays, objects, or unknown types, return as-is
		return value
	}
}

// =============================================================================
// Error Helpers
// =============================================================================
// Standardized error message formatting for consistency across resources.

// AddClientError adds a standardized client error message to diagnostics.
func AddClientError(resp interface{}, action, resourceType string, err error) {
	msg := fmt.Sprintf("Unable to %s %s: %s", action, resourceType, err)

	switch r := resp.(type) {
	case *resource.CreateResponse:
		r.Diagnostics.AddError("Client Error", msg)
	case *resource.ReadResponse:
		r.Diagnostics.AddError("Client Error", msg)
	case *resource.UpdateResponse:
		r.Diagnostics.AddError("Client Error", msg)
	case *resource.DeleteResponse:
		r.Diagnostics.AddError("Client Error", msg)
	}
}

// AddValidationError adds a standardized validation error to diagnostics.
func AddValidationError(resp interface{}, field, message string) {
	msg := fmt.Sprintf("Invalid value for '%s': %s", field, message)

	switch r := resp.(type) {
	case *resource.CreateResponse:
		r.Diagnostics.AddError("Validation Error", msg)
	case *resource.ReadResponse:
		r.Diagnostics.AddError("Validation Error", msg)
	case *resource.UpdateResponse:
		r.Diagnostics.AddError("Validation Error", msg)
	case *resource.DeleteResponse:
		r.Diagnostics.AddError("Validation Error", msg)
	}
}
