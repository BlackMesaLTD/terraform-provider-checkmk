package users

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &ContactGroupResource{}
	_ resource.ResourceWithImportState    = &ContactGroupResource{}
	_ resource.ResourceWithValidateConfig = &ContactGroupResource{}
)

func NewContactGroupResource() resource.Resource {
	return &ContactGroupResource{}
}

// ContactGroupResource defines the resource implementation.
type ContactGroupResource struct {
	providerData *common.ProviderData
}

// ContactGroupResourceModel describes the resource data model.
type ContactGroupResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Alias                 types.String `tfsdk:"alias"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

func (r *ContactGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_group"
}

func (r *ContactGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK contact group configuration. Contact groups do not require activation.",
		Attributes: map[string]schema.Attribute{
			"id":   common.ComputedIDAttribute("The unique identifier for the contact group (same as name)."),
			"name": common.RequiredIDAttribute("Unique identifier for the contact group. This serves as the contact group name."),
			"alias": schema.StringAttribute{
				MarkdownDescription: "Human-readable alias/title for the contact group.",
				Required:            true,
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *ContactGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *ContactGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContactGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.ContactGroupCreateRequest{
		Name:  data.Name.ValueString(),
		Alias: data.Alias.ValueString(),
	}

	contactGroup, err := r.providerData.Client.CreateContactGroup(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create contact group: %s", err))
		return
	}

	data.ID = types.StringValue(contactGroup.ID)
	data.Name = types.StringValue(contactGroup.ID)
	data.Alias = types.StringValue(contactGroup.Title)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContactGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contactGroup, err := r.providerData.Client.GetContactGroup(ctx, data.Name.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read contact group: %s", err))
		return
	}

	data.ID = types.StringValue(contactGroup.ID)
	data.Name = types.StringValue(contactGroup.ID)
	data.Alias = types.StringValue(contactGroup.Title)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ContactGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetContactGroupWithETag(ctx, data.Name.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch contact group ETag: %s", err))
		return
	}

	updateReq := &client.ContactGroupUpdateRequest{
		Alias: data.Alias.ValueString(),
	}

	contactGroup, err := r.providerData.Client.UpdateContactGroup(ctx, data.Name.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.Name.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update contact group: %s", err))
			return
		}
	}

	if contactGroup != nil {
		data.ID = types.StringValue(contactGroup.ID)
		data.Name = types.StringValue(contactGroup.ID)
		data.Alias = types.StringValue(contactGroup.Title)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContactGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Name.ValueString() == "all" {
		resp.Diagnostics.AddError(
			"Cannot Delete Builtin Contact Group",
			"The contact group 'all' is a builtin contact group and cannot be deleted.",
		)
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetContactGroupWithETag(ctx, data.Name.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil // Already deleted
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch contact group ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteContactGroup(ctx, data.Name.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete contact group: %s", err))
		return
	}
}

func (r *ContactGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *ContactGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data ContactGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against ContactGroup schema
	resp.Diagnostics.Append(validator.ValidateStringField("ContactGroup", "alias", data.Alias, path.Root("alias"))...)
}
