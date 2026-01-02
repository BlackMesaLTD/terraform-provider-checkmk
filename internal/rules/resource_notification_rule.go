package rules

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &NotificationRuleResource{}
	_ resource.ResourceWithImportState    = &NotificationRuleResource{}
	_ resource.ResourceWithValidateConfig = &NotificationRuleResource{}
)

func NewNotificationRuleResource() resource.Resource {
	return &NotificationRuleResource{}
}

// NotificationRuleResource defines the resource implementation.
type NotificationRuleResource struct {
	providerData *common.ProviderData
}

// NotificationRuleResourceModel describes the resource data model.
type NotificationRuleResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	RuleConfig            types.String `tfsdk:"rule_config"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

func (r *NotificationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_rule"
}

func (r *NotificationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK notification rule. Notification rules control how and when alerts are sent to contacts. " +
			"Requires activation. Available in CheckMK 2.2.0p5+.\n\n" +
			"**Note:** The CheckMK notification rule API requires a complete JSON configuration. " +
			"You can get an example configuration by creating a rule in the UI and then using the API to retrieve it:\n" +
			"```\ncurl -u automation:secret http://checkmk/site/check_mk/api/1.0/objects/notification_rule/{rule_id}\n```",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "CheckMK's internal UUID for the notification rule.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"rule_config": schema.StringAttribute{
				MarkdownDescription: "The complete rule configuration as JSON. This must include all required fields: " +
					"`rule_properties`, `notification_method`, `contact_selection`, and `conditions`. " +
					"The easiest way to get a working configuration is to create a rule in the CheckMK UI, " +
					"then retrieve it via the API.",
				Required: true,
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *NotificationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *NotificationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NotificationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	// Parse the JSON config
	var ruleConfig json.RawMessage
	if err := json.Unmarshal([]byte(data.RuleConfig.ValueString()), &ruleConfig); err != nil {
		resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("rule_config is not valid JSON: %s", err))
		return
	}

	createReq := &client.NotificationRuleCreateRequest{
		RuleConfig: ruleConfig,
	}

	rule, err := r.providerData.Client.CreateNotificationRule(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create notification rule: %s", err))
		return
	}

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "notification_rule"); err != nil {
		common.AddActivationWarning(resp, "NotificationRule", "created", err)
	}

	// Set ID from API response
	data.ID = types.StringValue(rule.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotificationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.providerData.Client.GetNotificationRule(ctx, data.ID.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read notification rule: %s", err))
		return
	}

	// Update state from API response
	data.ID = types.StringValue(rule.ID)

	// Convert rule_config back to JSON string
	configJSON, err := json.Marshal(rule.Extensions.RuleConfig)
	if err != nil {
		resp.Diagnostics.AddError("JSON Error", fmt.Sprintf("Unable to marshal rule_config: %s", err))
		return
	}

	// Compare JSON semantically - if content matches, preserve state's ordering
	// This prevents unnecessary diffs due to key ordering differences
	if !data.RuleConfig.IsNull() && !data.RuleConfig.IsUnknown() {
		if jsonSemanticEquals(data.RuleConfig.ValueString(), string(configJSON)) {
			// Keep existing value - content is the same
		} else {
			data.RuleConfig = types.StringValue(string(configJSON))
		}
	} else {
		data.RuleConfig = types.StringValue(string(configJSON))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// jsonSemanticEquals compares two JSON strings for semantic equality,
// ignoring key ordering and whitespace differences
func jsonSemanticEquals(a, b string) bool {
	var objA, objB interface{}
	if err := json.Unmarshal([]byte(a), &objA); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(b), &objB); err != nil {
		return false
	}
	// Re-marshal to canonical form for comparison
	canonA, err := json.Marshal(objA)
	if err != nil {
		return false
	}
	canonB, err := json.Marshal(objB)
	if err != nil {
		return false
	}
	return string(canonA) == string(canonB)
}

func (r *NotificationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NotificationRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	// Fetch ETag if strict locking is enabled
	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetNotificationRuleWithETag(ctx, data.ID.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch notification rule ETag: %s", err))
		return
	}

	// Parse the JSON config
	var ruleConfig json.RawMessage
	if err := json.Unmarshal([]byte(data.RuleConfig.ValueString()), &ruleConfig); err != nil {
		resp.Diagnostics.AddError("Invalid JSON", fmt.Sprintf("rule_config is not valid JSON: %s", err))
		return
	}

	updateReq := &client.NotificationRuleUpdateRequest{
		RuleConfig: ruleConfig,
	}

	rule, err := r.providerData.Client.UpdateNotificationRule(ctx, data.ID.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.ID.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update notification rule: %s", err))
			return
		}
	}

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "notification_rule"); err != nil {
		common.AddActivationWarning(resp, "NotificationRule", "updated", err)
	}

	if rule != nil {
		data.ID = types.StringValue(rule.ID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NotificationRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	// Fetch ETag if strict locking is enabled
	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetNotificationRuleWithETag(ctx, data.ID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil // Already deleted
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch notification rule ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteNotificationRule(ctx, data.ID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete notification rule: %s", err))
		return
	}

	// Activate changes if configured
	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "notification_rule"); err != nil {
		common.AddActivationWarning(resp, "NotificationRule", "deleted", err)
	}
}

func (r *NotificationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *NotificationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Notification rule uses raw JSON config - schema validation is handled by the API.
	// The OpenAPI schema for notification rules is complex and varies by notification method.
}
