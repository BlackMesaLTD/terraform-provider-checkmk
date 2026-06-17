package configuration

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

var (
	_ resource.Resource                   = &FolderResource{}
	_ resource.ResourceWithImportState    = &FolderResource{}
	_ resource.ResourceWithValidateConfig = &FolderResource{}
)

func NewFolderResource() resource.Resource {
	return &FolderResource{}
}

type FolderResource struct {
	providerData *common.ProviderData
}

type FolderResourceModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Parent     types.String `tfsdk:"parent"`
	Title      types.String `tfsdk:"title"`
	Path       types.String `tfsdk:"path"`
	Attributes types.Map    `tfsdk:"attributes"`
}

func (r *FolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (r *FolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a folder in CheckMK. Folders provide hierarchical organization for hosts and support attribute inheritance.",

		Attributes: map[string]schema.Attribute{
			"id": common.ComputedIDAttribute("The unique identifier for the folder (same as path)."),
			"name": schema.StringAttribute{
				Description: "The name of the folder. This is the folder's identifier within its parent.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent": schema.StringAttribute{
				Description: "The parent folder path. Use '/' for root level folders.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"title": schema.StringAttribute{
				Description: "The display title of the folder. If not set, defaults to the folder name.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description: "The full path of the folder. Computed from parent and name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"attributes": schema.MapAttribute{
				Description: "Folder attributes that will be inherited by hosts.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *FolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *FolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	attributes := make(map[string]interface{})
	if !data.Attributes.IsNull() && !data.Attributes.IsUnknown() {
		elements := make(map[string]types.String)
		resp.Diagnostics.Append(data.Attributes.ElementsAs(ctx, &elements, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		promoter := common.NewAttributePromoter(r.providerData, "checkmk_folder")
		for k, v := range elements {
			attributes[promoter.APIKey(k)] = v.ValueString()
		}
	}

	title := data.Name.ValueString()
	if !data.Title.IsNull() && !data.Title.IsUnknown() {
		title = data.Title.ValueString()
	}

	createReq := &client.FolderCreateRequest{
		Name:       data.Name.ValueString(),
		Title:      title,
		Parent:     data.Parent.ValueString(),
		Attributes: attributes,
	}

	folder, err := r.providerData.Client.CreateFolder(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create folder: %s", err))
		return
	}

	folderPath := computeFolderPath(data.Parent.ValueString(), data.Name.ValueString())
	data.ID = types.StringValue(folderPath)
	data.Path = types.StringValue(folderPath)
	data.Title = types.StringValue(folder.Title)

	// Keep the configured attribute keys in state (full replacement). The API
	// stores promoted tag groups under their "tag_" form, but state mirrors what
	// the user wrote so there's no perpetual diff. Only fall back to the API
	// response when the attribute map was left unset (Computed/unknown).
	if data.Attributes.IsNull() || data.Attributes.IsUnknown() {
		data.Attributes = attributesFromAPI(ctx, folder.Extensions.Attributes, &resp.Diagnostics)
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "folder"); err != nil {
		common.AddActivationWarning(resp, "Folder", "created", err)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	folder, err := r.providerData.Client.GetFolder(ctx, data.Path.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read folder: %s", err))
		return
	}

	data.Title = types.StringValue(folder.Title)

	// Reconcile attributes from the API response. When attributes are managed
	// (configured or previously stored), look each key up under its API form
	// (promoting unprefixed tag groups) and store it back under the configured
	// key; unmanaged attributes are ignored to avoid drift. When nothing is
	// managed (e.g. a fresh import), mirror the API response verbatim.
	if data.Attributes.IsNull() || data.Attributes.IsUnknown() {
		data.Attributes = attributesFromAPI(ctx, folder.Extensions.Attributes, &resp.Diagnostics)
	} else {
		promoter := common.NewAttributePromoter(r.providerData, "checkmk_folder")
		attrMap := make(map[string]string)
		for key := range data.Attributes.Elements() {
			if v, ok := folder.Extensions.Attributes[promoter.APIKey(key)]; ok {
				if str, ok := v.(string); ok {
					attrMap[key] = str
				}
			}
		}
		attrValue, diags := types.MapValueFrom(ctx, types.StringType, attrMap)
		resp.Diagnostics.Append(diags...)
		data.Attributes = attrValue
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FolderResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	attributes := make(map[string]interface{})
	if !data.Attributes.IsNull() && !data.Attributes.IsUnknown() {
		elements := make(map[string]types.String)
		resp.Diagnostics.Append(data.Attributes.ElementsAs(ctx, &elements, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		promoter := common.NewAttributePromoter(r.providerData, "checkmk_folder")
		for k, v := range elements {
			attributes[promoter.APIKey(k)] = v.ValueString()
		}
	}

	title := data.Name.ValueString()
	if !data.Title.IsNull() && !data.Title.IsUnknown() {
		title = data.Title.ValueString()
	}

	updateReq := &client.FolderUpdateRequest{
		Title:      title,
		Attributes: attributes,
	}

	folder, err := r.providerData.Client.UpdateFolder(ctx, data.Path.ValueString(), updateReq, "")
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update folder: %s", err))
		return
	}

	data.Title = types.StringValue(folder.Title)

	// Keep the configured attribute keys in state (full replacement); only fall
	// back to the API response when the attribute map was left unset.
	if data.Attributes.IsNull() || data.Attributes.IsUnknown() {
		data.Attributes = attributesFromAPI(ctx, folder.Extensions.Attributes, &resp.Diagnostics)
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "folder"); err != nil {
		common.AddActivationWarning(resp, "Folder", "updated", err)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FolderResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	if err := r.providerData.Client.DeleteFolder(ctx, data.Path.ValueString(), ""); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete folder: %s", err))
		return
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "folder"); err != nil {
		common.AddActivationWarning(resp, "Folder", "deleted", err)
	}
}

func (r *FolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	folderPath := req.ID
	if !strings.HasPrefix(folderPath, "/") {
		folderPath = "/" + folderPath
	}

	folder, err := r.providerData.Client.GetFolder(ctx, folderPath)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to import folder '%s': %s", req.ID, err))
		return
	}

	parent, name := splitFolderPath(folderPath)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), folderPath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("path"), folderPath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parent"), parent)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("title"), folder.Title)...)

	if len(folder.Extensions.Attributes) > 0 {
		attrMap := make(map[string]string)
		for k, v := range folder.Extensions.Attributes {
			if str, ok := v.(string); ok {
				attrMap[k] = str
			}
		}
		attrValue, diags := types.MapValueFrom(ctx, types.StringType, attrMap)
		resp.Diagnostics.Append(diags...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("attributes"), attrValue)...)
	}
}

// attributesFromAPI converts a CheckMK API attribute map into a Terraform map,
// preserving the API keys verbatim. It returns a null map when there are no
// string attributes to store.
func attributesFromAPI(ctx context.Context, apiAttrs map[string]interface{}, diags *diag.Diagnostics) types.Map {
	attrMap := make(map[string]string)
	for k, v := range apiAttrs {
		if str, ok := v.(string); ok {
			attrMap[k] = str
		}
	}
	if len(attrMap) == 0 {
		return types.MapNull(types.StringType)
	}
	attrValue, d := types.MapValueFrom(ctx, types.StringType, attrMap)
	diags.Append(d...)
	return attrValue
}

func computeFolderPath(parent, name string) string {
	if parent == "/" {
		return "/" + name
	}
	return parent + "/" + name
}

func splitFolderPath(folderPath string) (parent, name string) {
	folderPath = strings.TrimPrefix(folderPath, "/")
	parts := strings.Split(folderPath, "/")

	if len(parts) == 1 {
		return "/", parts[0]
	}

	name = parts[len(parts)-1]
	parent = "/" + strings.Join(parts[:len(parts)-1], "/")
	return parent, name
}

// ValidateConfig validates the resource configuration before apply.
// This uses the generated types to check attribute names and enum values.
func (r *FolderResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// Provider data may not be available during early validation
	if r.providerData == nil {
		return
	}

	var data FolderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate folder attributes using generated types
	validator := common.NewAttributeValidator(r.providerData)
	resp.Diagnostics.Append(validator.ValidateFolderAttributes(ctx, data.Attributes, path.Root("attributes"))...)
}
