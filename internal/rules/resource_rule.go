package rules

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &RuleResource{}
	_ resource.ResourceWithImportState    = &RuleResource{}
	_ resource.ResourceWithValidateConfig = &RuleResource{}
)

func NewRuleResource() resource.Resource {
	return &RuleResource{}
}

// RuleResource defines the resource implementation.
type RuleResource struct {
	providerData *common.ProviderData
}

// RuleResourceModel describes the resource data model.
type RuleResourceModel struct {
	ID         types.String `tfsdk:"id"`
	APIID      types.String `tfsdk:"api_id"`
	Ruleset    types.String `tfsdk:"ruleset"`
	Folder     types.String `tfsdk:"folder"`
	ValueRaw   types.String `tfsdk:"value_raw"`
	Properties types.Object `tfsdk:"properties"`
	Conditions types.Object `tfsdk:"conditions"`
}

// RulePropertiesModel describes the properties nested object
type RulePropertiesModel struct {
	Description types.String `tfsdk:"description"`
	Comment     types.String `tfsdk:"comment"`
	Disabled    types.Bool   `tfsdk:"disabled"`
}

func (r *RuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rule"
}

func (r *RuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK rule. Rules control monitoring behavior through conditions and values. " +
			"Requires activation. " +
			"**Important:** Label rulesets (host_label_rules, service_label_rules) cannot use label conditions to avoid circular dependencies.",
		Attributes: map[string]schema.Attribute{
			// Note: We don't use UseStateForUnknown here because the ID can change
			// when description or conditions change. This will show "(known after apply)"
			// in plan output but correctly handles ID changes.
			"id": schema.StringAttribute{
				MarkdownDescription: "Computed hash based on ruleset, description, and conditions. Used for identity.",
				Computed:            true,
			},
			"api_id": schema.StringAttribute{
				MarkdownDescription: "CheckMK's internal UUID for the rule. Used for updates and deletes.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ruleset": schema.StringAttribute{
				MarkdownDescription: "The ruleset name (e.g., 'extra_host_conf:check_interval', 'custom_checks', 'host_label_rules').",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"folder": schema.StringAttribute{
				MarkdownDescription: "The folder path where the rule will be created. Default is '/' (root folder). " +
					"Path delimiters can be `~`, `/`, or `\\`.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"value_raw": schema.StringAttribute{
				MarkdownDescription: "The rule value as a raw string. Format depends on the ruleset. " +
					"For simple values, use the string directly (e.g., '60'). " +
					"For complex values, use JSON encoding.",
				Required: true,
			},
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
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"disabled": schema.BoolAttribute{
						MarkdownDescription: "Whether the rule is disabled. Default is false.",
						Optional:            true,
						Computed:            true,
					},
				},
			},
			"conditions": RuleConditionsSchema(),
		},
	}
}

func (r *RuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *RuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	// Extract properties
	var properties RulePropertiesModel
	resp.Diagnostics.Append(data.Properties.As(ctx, &properties, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract conditions
	conditions, err := ConvertConditionsToClient(ctx, data.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions: %s", err))
		return
	}

	// Validate ruleset-specific conditions
	validationErrors := ValidateRulesetConditions(data.Ruleset.ValueString(), conditions)
	if len(validationErrors) > 0 {
		for _, validationErr := range validationErrors {
			resp.Diagnostics.AddError("Validation Error", validationErr)
		}
		return
	}

	// Set default folder if not specified
	folder := "/"
	if !data.Folder.IsNull() {
		folder = data.Folder.ValueString()
	}

	// Create rule via API
	createReq := &client.RuleCreateRequest{
		Ruleset: data.Ruleset.ValueString(),
		Folder:  folder,
		Properties: client.RuleProperties{
			Description: properties.Description.ValueString(),
			Comment:     properties.Comment.ValueString(),
			Disabled:    properties.Disabled.ValueBool(),
		},
		ValueRaw:   data.ValueRaw.ValueString(),
		Conditions: conditions,
	}

	rule, err := r.providerData.Client.CreateRule(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create rule: %s", err))
		return
	}

	// Generate deterministic hash for ID
	hash := client.GenerateRuleHash(
		data.Ruleset.ValueString(),
		properties.Description.ValueString(),
		conditions,
	)

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "rule"); err != nil {
		common.AddActivationWarning(resp, "Rule", "created", err)
	}

	// Set computed values
	data.ID = types.StringValue(hash)
	data.APIID = types.StringValue(rule.ID)
	if data.Folder.IsNull() {
		data.Folder = types.StringValue(rule.Extensions.Folder)
	}

	// Update properties from API response
	properties.Comment = types.StringValue(rule.Extensions.Properties.Comment)
	if rule.Extensions.Properties.Disabled {
		properties.Disabled = types.BoolValue(true)
	} else {
		properties.Disabled = types.BoolValue(false)
	}

	propertiesObj, diags := types.ObjectValueFrom(ctx, data.Properties.AttributeTypes(ctx), properties)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Properties = propertiesObj

	// Update conditions from API response
	conditionsObj, err := ConvertConditionsFromClient(ctx, rule.Extensions.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions from API: %s", err))
		return
	}
	data.Conditions = conditionsObj

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read by API ID if available
	if data.APIID.IsNull() || data.APIID.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing API ID",
			"Rule API ID is missing from state. This should not happen. Please report this issue.",
		)
		return
	}

	rule, err := r.providerData.Client.GetRule(ctx, data.APIID.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read rule: %s", err))
		return
	}

	// Extract properties from state to get description for hash generation
	var properties RulePropertiesModel
	resp.Diagnostics.Append(data.Properties.As(ctx, &properties, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Regenerate hash from API data
	hash := client.GenerateRuleHash(
		rule.Extensions.Ruleset,
		rule.Extensions.Properties.Description,
		rule.Extensions.Conditions,
	)

	// Update state with API data
	data.ID = types.StringValue(hash)
	data.APIID = types.StringValue(rule.ID)
	data.Ruleset = types.StringValue(rule.Extensions.Ruleset)
	data.Folder = types.StringValue(rule.Extensions.Folder)
	data.ValueRaw = types.StringValue(rule.Extensions.ValueRaw)

	// Update properties
	properties.Description = types.StringValue(rule.Extensions.Properties.Description)
	properties.Comment = types.StringValue(rule.Extensions.Properties.Comment)
	properties.Disabled = types.BoolValue(rule.Extensions.Properties.Disabled)

	propertiesObj, diags := types.ObjectValueFrom(ctx, data.Properties.AttributeTypes(ctx), properties)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Properties = propertiesObj

	// Update conditions from API response
	conditionsObj, err := ConvertConditionsFromClient(ctx, rule.Extensions.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions from API: %s", err))
		return
	}
	data.Conditions = conditionsObj

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	// Extract properties
	var properties RulePropertiesModel
	resp.Diagnostics.Append(data.Properties.As(ctx, &properties, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract conditions
	conditions, err := ConvertConditionsToClient(ctx, data.Conditions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions: %s", err))
		return
	}

	// Validate ruleset-specific conditions
	validationErrors := ValidateRulesetConditions(data.Ruleset.ValueString(), conditions)
	if len(validationErrors) > 0 {
		for _, validationErr := range validationErrors {
			resp.Diagnostics.AddError("Validation Error", validationErr)
		}
		return
	}

	// Fetch ETag if strict locking is enabled
	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetRuleWithETag(ctx, data.APIID.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch rule ETag: %s", err))
		return
	}

	updateReq := &client.RuleUpdateRequest{
		Properties: client.RuleProperties{
			Description: properties.Description.ValueString(),
			Comment:     properties.Comment.ValueString(),
			Disabled:    properties.Disabled.ValueBool(),
		},
		ValueRaw:   data.ValueRaw.ValueString(),
		Conditions: conditions,
	}

	rule, err := r.providerData.Client.UpdateRule(ctx, data.APIID.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.APIID.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update rule: %s", err))
			return
		}
	}

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "rule"); err != nil {
		common.AddActivationWarning(resp, "Rule", "updated", err)
	}

	// Update state with API response
	if rule != nil {
		// Regenerate hash from API data (in case identity fields changed)
		hash := client.GenerateRuleHash(
			rule.Extensions.Ruleset,
			rule.Extensions.Properties.Description,
			rule.Extensions.Conditions,
		)

		data.ID = types.StringValue(hash)
		data.APIID = types.StringValue(rule.ID)
		data.Ruleset = types.StringValue(rule.Extensions.Ruleset)
		data.Folder = types.StringValue(rule.Extensions.Folder)
		data.ValueRaw = types.StringValue(rule.Extensions.ValueRaw)

		// Update properties
		properties.Description = types.StringValue(rule.Extensions.Properties.Description)
		properties.Comment = types.StringValue(rule.Extensions.Properties.Comment)
		properties.Disabled = types.BoolValue(rule.Extensions.Properties.Disabled)

		propertiesObj, diags := types.ObjectValueFrom(ctx, data.Properties.AttributeTypes(ctx), properties)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Properties = propertiesObj

		// Update conditions from API response
		conditionsObj, err := ConvertConditionsFromClient(ctx, rule.Extensions.Conditions)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert conditions from API: %s", err))
			return
		}
		data.Conditions = conditionsObj
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	// Fetch ETag if strict locking is enabled
	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetRuleWithETag(ctx, data.APIID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch rule ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteRule(ctx, data.APIID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete rule: %s", err))
		return
	}

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "rule"); err != nil {
		common.AddActivationWarning(resp, "Rule", "deleted", err)
	}
}

func (r *RuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import accepts CheckMK's API UUID
	// Set both api_id and a placeholder ID (will be recalculated on first read)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), "imported")...)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *RuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data RuleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against RuleObject schema
	resp.Diagnostics.Append(validator.ValidateStringField("RuleObject", "ruleset", data.Ruleset, path.Root("ruleset"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("RuleObject", "folder", data.Folder, path.Root("folder"))...)
}
