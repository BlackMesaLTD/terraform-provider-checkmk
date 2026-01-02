package configuration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &TagGroupResource{}
	_ resource.ResourceWithImportState    = &TagGroupResource{}
	_ resource.ResourceWithValidateConfig = &TagGroupResource{}
)

func NewTagGroupResource() resource.Resource {
	return &TagGroupResource{}
}

// TagGroupResource defines the resource implementation.
type TagGroupResource struct {
	providerData *common.ProviderData
}

// TagGroupResourceModel describes the resource data model.
type TagGroupResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Title                 types.String `tfsdk:"title"`
	Topic                 types.String `tfsdk:"topic"`
	Help                  types.String `tfsdk:"help"`
	Tags                  types.List   `tfsdk:"tags"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

// TagModel describes the tag data model.
type TagModel struct {
	ID      types.String `tfsdk:"id"`
	Title   types.String `tfsdk:"title"`
	AuxTags types.List   `tfsdk:"aux_tags"`
}

func (r *TagGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_group"
}

func (r *TagGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK host tag group configuration. Tag groups do not require activation.",
		Attributes: map[string]schema.Attribute{
			"id": common.RequiredIDAttribute("Unique identifier for the tag group. This serves as the tag group ID."),
			"title": schema.StringAttribute{
				MarkdownDescription: "Human-readable title for the tag group.",
				Required:            true,
			},
			"topic": schema.StringAttribute{
				MarkdownDescription: "Topic for grouping tag groups in the UI. If not specified, CheckMK defaults to 'Tags'.",
				Optional:            true,
				Computed:            true,
			},
			"help": schema.StringAttribute{
				MarkdownDescription: "Help text describing the tag group. If not specified, defaults to empty string.",
				Optional:            true,
				Computed:            true,
			},
			"tags": schema.ListNestedAttribute{
				MarkdownDescription: "List of tags within this tag group.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Unique identifier for the tag.",
							Required:            true,
						},
						"title": schema.StringAttribute{
							MarkdownDescription: "Human-readable title for the tag.",
							Required:            true,
						},
						"aux_tags": schema.ListAttribute{
							MarkdownDescription: "List of auxiliary tag IDs associated with this tag. Optional, defaults to empty list.",
							ElementType:         types.StringType,
							Optional:            true,
							Computed:            true,
						},
					},
				},
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *TagGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

// convertTagsToClient converts Terraform tag list to client tag slice
func convertTagsToClient(ctx context.Context, tagsList types.List) ([]client.Tag, error) {
	if tagsList.IsNull() || tagsList.IsUnknown() {
		return nil, fmt.Errorf("tags list is null or unknown")
	}

	var tagsData []TagModel
	diags := tagsList.ElementsAs(ctx, &tagsData, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert tags list: %v", diags)
	}

	tags := make([]client.Tag, len(tagsData))
	for i, tagData := range tagsData {
		tag := client.Tag{
			ID:    tagData.ID.ValueString(),
			Title: tagData.Title.ValueString(),
		}

		if !tagData.AuxTags.IsNull() && !tagData.AuxTags.IsUnknown() {
			var auxTags []string
			diags := tagData.AuxTags.ElementsAs(ctx, &auxTags, false)
			if diags.HasError() {
				return nil, fmt.Errorf("failed to convert aux_tags: %v", diags)
			}
			tag.AuxTags = auxTags
		}

		tags[i] = tag
	}

	return tags, nil
}

// convertTagsFromClient converts client tag slice to Terraform tag list
func convertTagsFromClient(ctx context.Context, tags []client.Tag) (types.List, error) {
	tagType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"id":       types.StringType,
			"title":    types.StringType,
			"aux_tags": types.ListType{ElemType: types.StringType},
		},
	}

	tagObjects := make([]attr.Value, len(tags))
	for i, tag := range tags {
		// Always return a list for aux_tags (empty list if no aux_tags)
		// This avoids null vs empty list inconsistencies between config and state
		auxTags := tag.AuxTags
		if auxTags == nil {
			auxTags = []string{}
		}
		auxTagsList, diags := types.ListValueFrom(ctx, types.StringType, auxTags)
		if diags.HasError() {
			return types.ListNull(tagType), fmt.Errorf("failed to convert aux_tags: %v", diags)
		}
		auxTagsValue := auxTagsList

		tagObject, diags := types.ObjectValue(
			tagType.AttrTypes,
			map[string]attr.Value{
				"id":       types.StringValue(tag.ID),
				"title":    types.StringValue(tag.Title),
				"aux_tags": auxTagsValue,
			},
		)
		if diags.HasError() {
			return types.ListNull(tagType), fmt.Errorf("failed to create tag object: %v", diags)
		}

		tagObjects[i] = tagObject
	}

	tagsList, diags := types.ListValue(tagType, tagObjects)
	if diags.HasError() {
		return types.ListNull(tagType), fmt.Errorf("failed to create tags list: %v", diags)
	}

	return tagsList, nil
}

func (r *TagGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tags, err := convertTagsToClient(ctx, data.Tags)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert tags: %s", err))
		return
	}

	createReq := &client.TagGroupCreateRequest{
		ID:    data.ID.ValueString(),
		Title: data.Title.ValueString(),
		Tags:  tags,
	}

	if !data.Topic.IsNull() {
		createReq.Topic = data.Topic.ValueString()
	}

	if !data.Help.IsNull() {
		createReq.Help = data.Help.ValueString()
	}

	tagGroup, err := r.providerData.Client.CreateTagGroup(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create tag group: %s", err))
		return
	}

	data.ID = types.StringValue(tagGroup.ID)
	data.Title = types.StringValue(tagGroup.Title)
	data.Topic = types.StringValue(tagGroup.Extensions.Topic)
	data.Help = types.StringValue(tagGroup.Extensions.Help)

	tagsList, err := convertTagsFromClient(ctx, tagGroup.Extensions.Tags)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert tags from API: %s", err))
		return
	}
	data.Tags = tagsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagGroup, err := r.providerData.Client.GetTagGroup(ctx, data.ID.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read tag group: %s", err))
		return
	}

	data.ID = types.StringValue(tagGroup.ID)
	data.Title = types.StringValue(tagGroup.Title)
	data.Topic = types.StringValue(tagGroup.Extensions.Topic)
	data.Help = types.StringValue(tagGroup.Extensions.Help)

	tagsList, err := convertTagsFromClient(ctx, tagGroup.Extensions.Tags)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert tags from API: %s", err))
		return
	}
	data.Tags = tagsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TagGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	tags, err := convertTagsToClient(ctx, data.Tags)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert tags: %s", err))
		return
	}

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetTagGroupWithETag(ctx, data.ID.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch tag group ETag: %s", err))
		return
	}

	updateReq := &client.TagGroupUpdateRequest{
		Title: data.Title.ValueString(),
		Tags:  tags,
	}

	if !data.Topic.IsNull() {
		updateReq.Topic = data.Topic.ValueString()
	}

	if !data.Help.IsNull() {
		updateReq.Help = data.Help.ValueString()
	}

	tagGroup, err := r.providerData.Client.UpdateTagGroup(ctx, data.ID.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.ID.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update tag group: %s", err))
			return
		}
	}

	if tagGroup != nil {
		data.ID = types.StringValue(tagGroup.ID)
		data.Title = types.StringValue(tagGroup.Title)
		data.Topic = types.StringValue(tagGroup.Extensions.Topic)
		data.Help = types.StringValue(tagGroup.Extensions.Help)

		tagsList, err := convertTagsFromClient(ctx, tagGroup.Extensions.Tags)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert tags from API: %s", err))
			return
		}
		data.Tags = tagsList
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetTagGroupWithETag(ctx, data.ID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch tag group ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteTagGroup(ctx, data.ID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete tag group: %s", err))
		return
	}
}

func (r *TagGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *TagGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data TagGroupResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against HostTag schema (tag group schema)
	resp.Diagnostics.Append(validator.ValidateStringField("HostTag", "title", data.Title, path.Root("title"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("HostTag", "topic", data.Topic, path.Root("topic"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("HostTag", "help", data.Help, path.Root("help"))...)
}
