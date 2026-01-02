// Package wrappers provides typed wrapper resources for common CheckMK rulesets.
// These are optional convenience resources that wrap the generic checkmk_rule resource
// with strongly-typed value attributes.
package wrappers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// ProviderData contains the provider configuration needed by resources.
// This mirrors the common.ProviderData struct fields plus a reference
// to the original for tracking changes.
type ProviderData struct {
	Client                *client.Client
	Activate              string
	ForceForeignChanges   bool
	ActivationWaitTime    int
	StrictResourceLocking bool
	// CommonProviderData holds reference to original provider data for tracking
	CommonProviderData *common.ProviderData
}

// RuleWrapperConfig defines what each typed wrapper must provide.
// This allows the base implementation to handle common CRUD operations
// while each wrapper only defines its specific schema and value conversion.
type RuleWrapperConfig struct {
	// Ruleset is the CheckMK ruleset name (e.g., "extra_host_conf:check_interval")
	Ruleset string

	// TypeName is the Terraform resource type suffix (e.g., "host_check_interval")
	TypeName string

	// Description is the schema description for the resource
	Description string

	// ValueAttrName is the name of the value attribute (default: "value")
	ValueAttrName string

	// ValueSchema is the schema for the typed value attribute
	ValueSchema schema.Attribute

	// ToValueRaw converts the typed value to API format (value_raw string)
	ToValueRaw func(ctx context.Context, value attr.Value) (string, diag.Diagnostics)

	// FromValueRaw converts API format (value_raw string) back to typed value
	FromValueRaw func(ctx context.Context, raw string) (attr.Value, diag.Diagnostics)

	// ValidateConditions performs ruleset-specific condition validation (optional)
	// Returns error strings if validation fails
	ValidateConditions func(conditions map[string]interface{}) []string
}

// BaseRuleWrapperModel is the common data model for all typed rule wrappers.
// Each wrapper can embed this and add its own typed value field.
type BaseRuleWrapperModel struct {
	ID         types.String `tfsdk:"id"`
	APIID      types.String `tfsdk:"api_id"`
	Folder     types.String `tfsdk:"folder"`
	Properties types.Object `tfsdk:"properties"`
	Conditions types.Object `tfsdk:"conditions"`
}

// RulePropertiesModel represents the properties nested object.
type RulePropertiesModel struct {
	Description types.String `tfsdk:"description"`
	Comment     types.String `tfsdk:"comment"`
	Disabled    types.Bool   `tfsdk:"disabled"`
}

// propertiesAttrTypes returns the attribute types for the properties object.
func propertiesAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"description": types.StringType,
		"comment":     types.StringType,
		"disabled":    types.BoolType,
	}
}

// BaseRuleWrapper provides the common implementation for typed rule wrappers.
type BaseRuleWrapper struct {
	Config       RuleWrapperConfig
	ProviderData *ProviderData
}

// Ensure BaseRuleWrapper implements the required interfaces.
var (
	_ resource.Resource                   = &BaseRuleWrapper{}
	_ resource.ResourceWithImportState    = &BaseRuleWrapper{}
	_ resource.ResourceWithValidateConfig = &BaseRuleWrapper{}
)

// Metadata returns the resource type name.
func (r *BaseRuleWrapper) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.Config.TypeName
}

// Schema returns the resource schema.
func (r *BaseRuleWrapper) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	valueAttrName := r.Config.ValueAttrName
	if valueAttrName == "" {
		valueAttrName = "value"
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: r.Config.Description,
		Attributes: map[string]schema.Attribute{
			// Note: We don't use UseStateForUnknown here because the ID can change
			// when description or conditions change. This will show "(known after apply)"
			// in plan output but correctly handles ID changes.
			"id": schema.StringAttribute{
				MarkdownDescription: "Computed hash based on description and conditions. Used for Terraform identity.",
				Computed:            true,
			},
			"api_id": schema.StringAttribute{
				MarkdownDescription: "CheckMK's internal UUID for the rule. Used for updates and deletes.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"folder": schema.StringAttribute{
				MarkdownDescription: "The folder path where the rule will be created. Default is '/' (root folder).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
			},
			valueAttrName: r.Config.ValueSchema,
			"properties": schema.SingleNestedAttribute{
				MarkdownDescription: "Rule properties including description, comment, and disabled status.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						MarkdownDescription: "Human-readable description of the rule. Required and part of rule identity.",
						Required:            true,
					},
					"comment": schema.StringAttribute{
						MarkdownDescription: "Optional comment for the rule.",
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(""),
					},
					"disabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the rule is disabled. Default is false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			"conditions": RuleConditionsSchema(),
		},
	}
}

// Configure sets up the resource with provider data.
func (r *BaseRuleWrapper) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	// Use reflection to extract ProviderData fields since we can't import
	// the provider package directly (would cause import cycle)
	providerData := extractProviderData(req.ProviderData)
	if providerData == nil {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Unable to extract provider data from type: %T", req.ProviderData),
		)
		return
	}

	r.ProviderData = providerData
}

// extractProviderData uses reflection to extract ProviderData fields from
// the provider package's ProviderData struct without importing it.
// It also stores a reference to the original common.ProviderData for change tracking.
func extractProviderData(data interface{}) *ProviderData {
	// Try to cast directly to common.ProviderData first
	commonPD, isCommonPD := data.(*common.ProviderData)

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Extract Client field
	clientField := v.FieldByName("Client")
	if !clientField.IsValid() || !clientField.CanInterface() {
		return nil
	}
	c, ok := clientField.Interface().(*client.Client)
	if !ok {
		return nil
	}

	// Extract other fields with defaults
	activate := "manual"
	if f := v.FieldByName("Activate"); f.IsValid() && f.Kind() == reflect.String {
		activate = f.String()
	}

	forceForeignChanges := true
	if f := v.FieldByName("ForceForeignChanges"); f.IsValid() && f.Kind() == reflect.Bool {
		forceForeignChanges = f.Bool()
	}

	activationWaitTime := 5
	if f := v.FieldByName("ActivationWaitTime"); f.IsValid() && f.Kind() == reflect.Int {
		activationWaitTime = int(f.Int())
	}

	strictResourceLocking := false
	if f := v.FieldByName("StrictResourceLocking"); f.IsValid() && f.Kind() == reflect.Bool {
		strictResourceLocking = f.Bool()
	}

	pd := &ProviderData{
		Client:                c,
		Activate:              activate,
		ForceForeignChanges:   forceForeignChanges,
		ActivationWaitTime:    activationWaitTime,
		StrictResourceLocking: strictResourceLocking,
	}

	// Store reference to original for change tracking
	if isCommonPD {
		pd.CommonProviderData = commonPD
	}

	return pd
}

// Create creates a new rule.
func (r *BaseRuleWrapper) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Get the value attribute dynamically
	valueAttrName := r.Config.ValueAttrName
	if valueAttrName == "" {
		valueAttrName = "value"
	}

	// Extract base model
	var id, apiID, folder types.String
	var properties, conditions types.Object
	var typedValue attr.Value

	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("api_id"), &apiID)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("folder"), &folder)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("properties"), &properties)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("conditions"), &conditions)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(valueAttrName), &typedValue)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Extract properties
	var props RulePropertiesModel
	resp.Diagnostics.Append(properties.As(ctx, &props, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert conditions to client format
	conditionsMap, err := ConvertConditionsToClient(ctx, conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions: %s", err))
		return
	}
	if conditionsMap == nil {
		conditionsMap = make(map[string]interface{})
	}

	// Validate conditions if validator is provided
	if r.Config.ValidateConditions != nil {
		validationErrors := r.Config.ValidateConditions(conditionsMap)
		for _, validationErr := range validationErrors {
			resp.Diagnostics.AddError("Validation Error", validationErr)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Convert typed value to value_raw
	valueRaw, diags := r.Config.ToValueRaw(ctx, typedValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create rule via API
	createReq := &client.RuleCreateRequest{
		Ruleset: r.Config.Ruleset,
		Folder:  folder.ValueString(),
		Properties: client.RuleProperties{
			Description: props.Description.ValueString(),
			Comment:     props.Comment.ValueString(),
			Disabled:    props.Disabled.ValueBool(),
		},
		ValueRaw:   valueRaw,
		Conditions: conditionsMap,
	}

	rule, err := r.ProviderData.Client.CreateRule(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create %s rule: %s", r.Config.TypeName, err))
		return
	}

	// Generate deterministic hash for ID
	hash := client.GenerateRuleHash(
		r.Config.Ruleset,
		props.Description.ValueString(),
		conditionsMap,
	)

	// Activate changes if configured
	if err := trackAndActivate(ctx, r.ProviderData, "rule_wrapper"); err != nil {
		resp.Diagnostics.AddWarning(
			"Activation Warning",
			fmt.Sprintf("Rule created but activation failed: %s. Changes may not be active.", err),
		)
	}

	// Update state
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(hash))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_id"), types.StringValue(rule.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringValue(rule.Extensions.Folder))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(valueAttrName), typedValue)...)

	// Update properties from API response
	updatedProps := RulePropertiesModel{
		Description: types.StringValue(rule.Extensions.Properties.Description),
		Comment:     types.StringValue(rule.Extensions.Properties.Comment),
		Disabled:    types.BoolValue(rule.Extensions.Properties.Disabled),
	}
	propsObj, diags := types.ObjectValueFrom(ctx, propertiesAttrTypes(), updatedProps)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), propsObj)...)

	// Preserve conditions as specified in config (don't normalize {} to null)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("conditions"), conditions)...)
}

// isEffectivelyEmptyConditions returns true if the conditions object represents empty conditions.
// This handles both null objects and objects with all-null attributes.
func isEffectivelyEmptyConditions(conditions types.Object) bool {
	if conditions.IsNull() || conditions.IsUnknown() {
		return true
	}

	attrs := conditions.Attributes()
	if len(attrs) == 0 {
		return true
	}

	for _, v := range attrs {
		if !v.IsNull() && !v.IsUnknown() {
			// For list types, also check if they're empty
			if listVal, ok := v.(types.List); ok {
				if len(listVal.Elements()) > 0 {
					return false
				}
				continue
			}
			return false
		}
	}

	return true
}

// Read reads the current state of the rule.
func (r *BaseRuleWrapper) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	valueAttrName := r.Config.ValueAttrName
	if valueAttrName == "" {
		valueAttrName = "value"
	}

	// Get current state
	var apiID types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("api_id"), &apiID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if apiID.IsNull() || apiID.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing API ID",
			"Rule API ID is missing from state. This should not happen. Please report this issue.",
		)
		return
	}

	// Get rule from API
	rule, err := r.ProviderData.Client.GetRule(ctx, apiID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read %s rule: %s", r.Config.TypeName, err))
		return
	}

	// Regenerate hash from API data
	hash := client.GenerateRuleHash(
		rule.Extensions.Ruleset,
		rule.Extensions.Properties.Description,
		rule.Extensions.Conditions,
	)

	// Convert value_raw back to typed value
	typedValue, diags := r.Config.FromValueRaw(ctx, rule.Extensions.ValueRaw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update state
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(hash))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_id"), types.StringValue(rule.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringValue(rule.Extensions.Folder))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(valueAttrName), typedValue)...)

	// Update properties
	props := RulePropertiesModel{
		Description: types.StringValue(rule.Extensions.Properties.Description),
		Comment:     types.StringValue(rule.Extensions.Properties.Comment),
		Disabled:    types.BoolValue(rule.Extensions.Properties.Disabled),
	}
	propsObj, diags := types.ObjectValueFrom(ctx, propertiesAttrTypes(), props)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), propsObj)...)

	// Convert conditions from API
	conditionsObj, err := ConvertConditionsFromClient(ctx, rule.Extensions.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions: %s", err))
		return
	}

	// If API returns empty/null conditions, preserve the prior state's representation
	// to avoid drift between null and empty object ({}).
	// This is critical: if user specified `conditions = {}`, we must return `{}`, not `null`.
	if conditionsObj.IsNull() || isEffectivelyEmptyConditions(conditionsObj) {
		var priorConditions types.Object
		req.State.GetAttribute(ctx, path.Root("conditions"), &priorConditions)
		// Preserve prior state's representation (whether null or empty object)
		conditionsObj = priorConditions
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("conditions"), conditionsObj)...)
}

// Update updates an existing rule.
func (r *BaseRuleWrapper) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	valueAttrName := r.Config.ValueAttrName
	if valueAttrName == "" {
		valueAttrName = "value"
	}

	// Extract plan data
	var apiID, folder types.String
	var properties, conditions types.Object
	var typedValue attr.Value

	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("api_id"), &apiID)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("api_id"), &apiID)...) // Use state for api_id
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("folder"), &folder)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("properties"), &properties)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("conditions"), &conditions)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(valueAttrName), &typedValue)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Extract properties
	var props RulePropertiesModel
	resp.Diagnostics.Append(properties.As(ctx, &props, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert conditions
	conditionsMap, err := ConvertConditionsToClient(ctx, conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions: %s", err))
		return
	}
	if conditionsMap == nil {
		conditionsMap = make(map[string]interface{})
	}

	// Validate conditions
	if r.Config.ValidateConditions != nil {
		validationErrors := r.Config.ValidateConditions(conditionsMap)
		for _, validationErr := range validationErrors {
			resp.Diagnostics.AddError("Validation Error", validationErr)
		}
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Convert typed value to value_raw
	valueRaw, diags := r.Config.ToValueRaw(ctx, typedValue)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch ETag if strict locking is enabled
	etag := ""
	if r.ProviderData.StrictResourceLocking {
		result, err := r.ProviderData.Client.GetRuleWithETag(ctx, apiID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch rule ETag: %s", err))
			return
		}
		etag = result.ETag
	}

	// Update rule
	updateReq := &client.RuleUpdateRequest{
		Properties: client.RuleProperties{
			Description: props.Description.ValueString(),
			Comment:     props.Comment.ValueString(),
			Disabled:    props.Disabled.ValueBool(),
		},
		ValueRaw:   valueRaw,
		Conditions: conditionsMap,
	}

	rule, err := r.ProviderData.Client.UpdateRule(ctx, apiID.ValueString(), updateReq, etag)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update %s rule: %s", r.Config.TypeName, err))
		return
	}

	// Regenerate hash
	hash := client.GenerateRuleHash(
		r.Config.Ruleset,
		props.Description.ValueString(),
		conditionsMap,
	)

	// Activate changes
	if err := trackAndActivate(ctx, r.ProviderData, "rule_wrapper"); err != nil {
		resp.Diagnostics.AddWarning(
			"Activation Warning",
			fmt.Sprintf("Rule updated but activation failed: %s. Changes may not be active.", err),
		)
	}

	// Get prior state ID to preserve it during update
	var priorID types.String
	req.State.GetAttribute(ctx, path.Root("id"), &priorID)

	// Use prior ID if available (update), otherwise use hash (create)
	idToUse := priorID.ValueString()
	if idToUse == "" || idToUse == "imported" {
		idToUse = hash
	}

	// Update state
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(idToUse))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_id"), types.StringValue(rule.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), types.StringValue(rule.Extensions.Folder))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(valueAttrName), typedValue)...)

	// Update properties from API response
	updatedProps := RulePropertiesModel{
		Description: types.StringValue(rule.Extensions.Properties.Description),
		Comment:     types.StringValue(rule.Extensions.Properties.Comment),
		Disabled:    types.BoolValue(rule.Extensions.Properties.Disabled),
	}
	propsObj, diags := types.ObjectValueFrom(ctx, propertiesAttrTypes(), updatedProps)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("properties"), propsObj)...)

	// Preserve conditions as specified in plan (don't normalize {} to null)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("conditions"), conditions)...)
}

// Delete deletes the rule.
func (r *BaseRuleWrapper) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Get current state
	var apiID types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("api_id"), &apiID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch ETag if strict locking is enabled
	etag := ""
	if r.ProviderData.StrictResourceLocking {
		result, err := r.ProviderData.Client.GetRuleWithETag(ctx, apiID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				// Already deleted
				return
			}
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch rule ETag: %s", err))
			return
		}
		etag = result.ETag
	}

	// Delete rule
	if err := r.ProviderData.Client.DeleteRule(ctx, apiID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete %s rule: %s", r.Config.TypeName, err))
		return
	}

	// Activate changes
	if err := trackAndActivate(ctx, r.ProviderData, "rule_wrapper"); err != nil {
		resp.Diagnostics.AddWarning(
			"Activation Warning",
			fmt.Sprintf("Rule deleted but activation failed: %s. Changes may not be active.", err),
		)
	}
}

// ImportState imports an existing rule by API ID.
func (r *BaseRuleWrapper) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "imported")...)
}

// trackAndActivate tracks a change for batch activation reporting and activates
// if the provider is configured for auto activation.
func trackAndActivate(ctx context.Context, providerData *ProviderData, resourceType string) error {
	// Track the change for batch activation reporting
	if providerData.CommonProviderData != nil {
		providerData.CommonProviderData.TrackChange(resourceType)
	}

	// Only activate if in auto mode
	if providerData.Activate != "auto" {
		return nil
	}
	return providerData.Client.ActivateChanges(
		ctx,
		nil, // sites - use all sites
		providerData.ForceForeignChanges,
		providerData.StrictResourceLocking,
		providerData.ActivationWaitTime,
	)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *BaseRuleWrapper) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Rule wrappers use the generic RuleObject schema for validation.
	// Most validation is handled by the ruleset-specific ValidateConditions callback.
	// The Terraform schema provides structural validation.
}
