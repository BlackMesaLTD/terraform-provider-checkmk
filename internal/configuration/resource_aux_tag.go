package configuration

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
	_ resource.Resource                   = &AuxTagResource{}
	_ resource.ResourceWithImportState    = &AuxTagResource{}
	_ resource.ResourceWithValidateConfig = &AuxTagResource{}
)

func NewAuxTagResource() resource.Resource {
	return &AuxTagResource{}
}

// AuxTagResource defines the resource implementation.
type AuxTagResource struct {
	providerData *common.ProviderData
}

// AuxTagResourceModel describes the resource data model.
type AuxTagResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Title                 types.String `tfsdk:"title"`
	Topic                 types.String `tfsdk:"topic"`
	Help                  types.String `tfsdk:"help"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

func (r *AuxTagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aux_tag"
}

func (r *AuxTagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK auxiliary tag configuration. Auxiliary tags do not require activation.\n\n" +
			"Note: Built-in aux tags (ip-v4, ip-v6, snmp, tcp, checkmk-agent, ping) cannot be deleted or modified.",
		Attributes: map[string]schema.Attribute{
			"id": common.RequiredIDAttribute("Unique identifier for the auxiliary tag. This serves as the aux tag ID."),
			"title": schema.StringAttribute{
				MarkdownDescription: "Human-readable title for the auxiliary tag.",
				Required:            true,
			},
			"topic": schema.StringAttribute{
				MarkdownDescription: "Topic for grouping auxiliary tags in the UI. If not specified, defaults to 'Tags'.",
				Optional:            true,
				Computed:            true,
			},
			"help": schema.StringAttribute{
				MarkdownDescription: "Help text describing the auxiliary tag. If not specified, defaults to empty string.",
				Optional:            true,
				Computed:            true,
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *AuxTagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *AuxTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AuxTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Topic is required by the API, default to "Tags" if not specified
	topic := "Tags"
	if !data.Topic.IsNull() && data.Topic.ValueString() != "" {
		topic = data.Topic.ValueString()
	}

	createReq := &client.AuxTagCreateRequest{
		AuxTagID: data.ID.ValueString(),
		Title:    data.Title.ValueString(),
		Topic:    topic,
	}

	if !data.Help.IsNull() {
		createReq.Help = data.Help.ValueString()
	}

	auxTag, err := r.providerData.Client.CreateAuxTag(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create aux tag: %s", err))
		return
	}

	data.ID = types.StringValue(auxTag.ID)
	data.Title = types.StringValue(auxTag.Title)
	data.Topic = types.StringValue(auxTag.Extensions.Topic)
	data.Help = types.StringValue(auxTag.Extensions.Help)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AuxTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AuxTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	auxTag, err := r.providerData.Client.GetAuxTag(ctx, data.ID.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read aux tag: %s", err))
		return
	}

	data.ID = types.StringValue(auxTag.ID)
	data.Title = types.StringValue(auxTag.Title)
	data.Topic = types.StringValue(auxTag.Extensions.Topic)
	data.Help = types.StringValue(auxTag.Extensions.Help)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AuxTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AuxTagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetAuxTagWithETag(ctx, data.ID.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch aux tag ETag: %s", err))
		return
	}

	updateReq := &client.AuxTagUpdateRequest{
		Title: data.Title.ValueString(),
	}

	if !data.Topic.IsNull() {
		updateReq.Topic = data.Topic.ValueString()
	}

	if !data.Help.IsNull() {
		updateReq.Help = data.Help.ValueString()
	}

	auxTag, err := r.providerData.Client.UpdateAuxTag(ctx, data.ID.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.ID.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update aux tag: %s", err))
			return
		}
	}

	if auxTag != nil {
		data.ID = types.StringValue(auxTag.ID)
		data.Title = types.StringValue(auxTag.Title)
		data.Topic = types.StringValue(auxTag.Extensions.Topic)
		data.Help = types.StringValue(auxTag.Extensions.Help)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AuxTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AuxTagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetAuxTagWithETag(ctx, data.ID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch aux tag ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteAuxTag(ctx, data.ID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete aux tag: %s", err))
		return
	}
}

func (r *AuxTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *AuxTagResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data AuxTagResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against AuxTagAttrsCreate schema
	resp.Diagnostics.Append(validator.ValidateStringField("AuxTagAttrsCreate", "title", data.Title, path.Root("title"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("AuxTagAttrsCreate", "topic", data.Topic, path.Root("topic"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("AuxTagAttrsCreate", "help", data.Help, path.Root("help"))...)
}
