package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/terraform-provider-checkmk/internal/common"
)

var (
	_ resource.Resource                   = &ActivationResource{}
	_ resource.ResourceWithValidateConfig = &ActivationResource{}
)

func NewActivationResource() resource.Resource {
	return &ActivationResource{}
}

// ActivationResource triggers activation of pending CheckMK changes.
// This resource should be used with depends_on to ensure it runs after
// all other resources that make changes.
type ActivationResource struct {
	providerData *common.ProviderData
}

// ActivationResourceModel describes the resource data model.
type ActivationResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ForceForeignChanges types.Bool   `tfsdk:"force_foreign_changes"`
	WaitTime            types.Int64  `tfsdk:"wait_time"`
	Sites               types.List   `tfsdk:"sites"`

	// Computed outputs
	ActivationID    types.String `tfsdk:"activation_id"`
	ActivatedAt     types.String `tfsdk:"activated_at"`
	PendingChanges  types.Map    `tfsdk:"pending_changes"`
	TotalChanges    types.Int64  `tfsdk:"total_changes"`
	ActivationNotes types.String `tfsdk:"activation_notes"`
}

func (r *ActivationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_activation"
}

func (r *ActivationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Triggers activation of pending CheckMK configuration changes.

This resource should be used with ` + "`depends_on`" + ` to ensure it runs after all resources
that make changes. It provides feedback on how many changes were activated.

## Example Usage

` + "```hcl" + `
provider "checkmk" {
  activate = "manual"  # Don't auto-activate individual resources
}

resource "checkmk_folder" "all" {
  for_each = local.folders
  # ...
}

resource "checkmk_host" "all" {
  for_each = local.hosts
  # ...
}

# Activate after all folders and hosts are created
resource "checkmk_activation" "after_hosts" {
  depends_on = [
    checkmk_folder.all,
    checkmk_host.all,
  ]
}
` + "```" + `

## Staged Activations

You can use multiple activation resources to activate in stages:

` + "```hcl" + `
resource "checkmk_activation" "after_folders" {
  depends_on = [checkmk_folder.all]
}

resource "checkmk_activation" "after_hosts" {
  depends_on = [
    checkmk_activation.after_folders,
    checkmk_host.all,
  ]
}
` + "```" + `
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Unique identifier for this activation resource instance.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"force_foreign_changes": schema.BoolAttribute{
				Description: "Whether to activate changes made by other users/tools (default: true). " +
					"When true, activates ALL pending changes including those made outside Terraform.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"wait_time": schema.Int64Attribute{
				Description: "Time in seconds to wait after activation for changes to propagate (default: 5).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(5),
			},
			"sites": schema.ListAttribute{
				Description: "List of sites to activate. If empty, activates all sites.",
				ElementType: types.StringType,
				Optional:    true,
			},

			// Computed outputs
			"activation_id": schema.StringAttribute{
				Description: "The CheckMK activation run ID.",
				Computed:    true,
			},
			"activated_at": schema.StringAttribute{
				Description: "Timestamp when activation was triggered.",
				Computed:    true,
			},
			"pending_changes": schema.MapAttribute{
				Description: "Map of resource types to counts of changes that were pending before activation.",
				ElementType: types.Int64Type,
				Computed:    true,
			},
			"total_changes": schema.Int64Attribute{
				Description: "Total number of changes that were activated.",
				Computed:    true,
			},
			"activation_notes": schema.StringAttribute{
				Description: "Human-readable summary of the activation.",
				Computed:    true,
			},
		},
	}
}

func (r *ActivationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *ActivationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ActivationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate unique ID for this activation instance
	data.ID = types.StringValue(fmt.Sprintf("activation-%d", time.Now().UnixNano()))

	// Get pending changes from provider tracking
	pendingChanges := r.providerData.GetPendingChanges()
	totalChanges := r.providerData.TotalPendingChanges()

	// Convert pending changes to Terraform map
	pendingMap := make(map[string]int64)
	for k, v := range pendingChanges {
		pendingMap[k] = int64(v)
	}
	pendingMapValue, diags := types.MapValueFrom(ctx, types.Int64Type, pendingMap)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.PendingChanges = pendingMapValue
	data.TotalChanges = types.Int64Value(int64(totalChanges))

	// Build activation notes
	notes := r.buildActivationNotes(pendingChanges, totalChanges)
	data.ActivationNotes = types.StringValue(notes)

	// Log activation details
	tflog.Info(ctx, "Activating CheckMK changes", map[string]interface{}{
		"total_changes":   totalChanges,
		"pending_changes": pendingChanges,
	})

	// Get sites list
	var sites []string
	if !data.Sites.IsNull() {
		resp.Diagnostics.Append(data.Sites.ElementsAs(ctx, &sites, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Perform activation
	forceForeign := data.ForceForeignChanges.ValueBool()
	waitTime := int(data.WaitTime.ValueInt64())

	err := r.providerData.Client.ActivateChanges(
		ctx,
		sites,
		forceForeign,
		r.providerData.StrictResourceLocking,
		waitTime,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Activation Failed",
			fmt.Sprintf("Failed to activate CheckMK changes: %s\n\n%s", err, notes),
		)
		return
	}

	// Record activation time
	data.ActivatedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))
	data.ActivationID = types.StringValue(data.ID.ValueString()) // Use our ID as activation ID

	// Clear tracked changes after successful activation
	r.providerData.ClearPendingChanges()

	tflog.Info(ctx, "Activation completed successfully", map[string]interface{}{
		"activation_id": data.ActivationID.ValueString(),
		"notes":         notes,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ActivationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ActivationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Activation is a point-in-time operation - state doesn't change after creation
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ActivationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ActivationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get pending changes from provider tracking
	pendingChanges := r.providerData.GetPendingChanges()
	totalChanges := r.providerData.TotalPendingChanges()

	// Only activate if there are pending changes
	if totalChanges == 0 {
		tflog.Info(ctx, "No pending changes to activate")
		data.ActivationNotes = types.StringValue("No pending changes to activate")
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Convert pending changes to Terraform map
	pendingMap := make(map[string]int64)
	for k, v := range pendingChanges {
		pendingMap[k] = int64(v)
	}
	pendingMapValue, diags := types.MapValueFrom(ctx, types.Int64Type, pendingMap)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.PendingChanges = pendingMapValue
	data.TotalChanges = types.Int64Value(int64(totalChanges))

	// Build activation notes
	notes := r.buildActivationNotes(pendingChanges, totalChanges)
	data.ActivationNotes = types.StringValue(notes)

	tflog.Info(ctx, "Activating CheckMK changes (update)", map[string]interface{}{
		"total_changes":   totalChanges,
		"pending_changes": pendingChanges,
	})

	// Get sites list
	var sites []string
	if !data.Sites.IsNull() {
		resp.Diagnostics.Append(data.Sites.ElementsAs(ctx, &sites, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Perform activation
	forceForeign := data.ForceForeignChanges.ValueBool()
	waitTime := int(data.WaitTime.ValueInt64())

	err := r.providerData.Client.ActivateChanges(
		ctx,
		sites,
		forceForeign,
		r.providerData.StrictResourceLocking,
		waitTime,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Activation Failed",
			fmt.Sprintf("Failed to activate CheckMK changes: %s\n\n%s", err, notes),
		)
		return
	}

	// Record activation time
	data.ActivatedAt = types.StringValue(time.Now().UTC().Format(time.RFC3339))

	// Clear tracked changes after successful activation
	r.providerData.ClearPendingChanges()

	tflog.Info(ctx, "Activation completed successfully (update)", map[string]interface{}{
		"activation_id": data.ActivationID.ValueString(),
		"notes":         notes,
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ActivationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Activation resources don't need cleanup - they're just triggers
	// State is automatically removed
}

// buildActivationNotes creates a human-readable summary of pending changes
func (r *ActivationResource) buildActivationNotes(pendingChanges map[string]int, total int) string {
	if total == 0 {
		return "No pending changes tracked by Terraform"
	}

	var parts []string
	// Sort keys for consistent output
	keys := make([]string, 0, len(pendingChanges))
	for k := range pendingChanges {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := pendingChanges[k]
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%d %s(s)", v, k))
		}
	}

	return fmt.Sprintf("Activating %d change(s): %s", total, strings.Join(parts, ", "))
}

// ValidateConfig validates the resource configuration using generated types.
func (r *ActivationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Activation resource uses provider-specific attributes, not CheckMK API schemas.
	// No OpenAPI schema validation needed - Terraform schema handles validation.
}
