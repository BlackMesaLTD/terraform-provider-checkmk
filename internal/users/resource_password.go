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
	_ resource.Resource                   = &PasswordResource{}
	_ resource.ResourceWithImportState    = &PasswordResource{}
	_ resource.ResourceWithValidateConfig = &PasswordResource{}
)

func NewPasswordResource() resource.Resource {
	return &PasswordResource{}
}

// PasswordResource defines the resource implementation.
type PasswordResource struct {
	providerData *common.ProviderData
}

// PasswordResourceModel describes the resource data model.
type PasswordResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	PasswordID            types.String `tfsdk:"password_id"`
	Title                 types.String `tfsdk:"title"`
	Password              types.String `tfsdk:"password"`
	Owner                 types.String `tfsdk:"owner"`
	EditableBy            types.String `tfsdk:"editable_by"`
	ShareWith             types.List   `tfsdk:"share_with"`
	Comment               types.String `tfsdk:"comment"`
	DocumentationURL      types.String `tfsdk:"documentation_url"`
	CustomerID            types.String `tfsdk:"customer_id"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

func (r *PasswordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_password"
}

func (r *PasswordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK password. Passwords are stored credentials that can be used for various integrations. Does not require activation.",
		Attributes: map[string]schema.Attribute{
			"id":          common.ComputedIDAttribute("The unique identifier for the password (same as password_id)."),
			"password_id": common.RequiredIDAttribute("Unique identifier for the password. Cannot be changed after creation."),
			"title": schema.StringAttribute{
				MarkdownDescription: "Human-readable title for the password.",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The actual password/secret value. This is write-only and will never be returned by the API.",
				Required:            true,
				Sensitive:           true,
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "Deprecated: use editable_by instead. Owner of the password.",
				Optional:            true,
				Computed:            true,
				DeprecationMessage:  "Use editable_by instead. The owner field is deprecated.",
			},
			"editable_by": schema.StringAttribute{
				MarkdownDescription: "Contact group name that has edit access to this password. Defaults to 'admin' if neither owner nor editable_by is set.",
				Optional:            true,
				Computed:            true,
			},
			"share_with": schema.ListAttribute{
				MarkdownDescription: "List of contact group names that have read access to this password.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "Optional comment for the password.",
				Optional:            true,
			},
			"documentation_url": schema.StringAttribute{
				MarkdownDescription: "Optional URL to documentation for this password.",
				Optional:            true,
			},
			"customer_id": schema.StringAttribute{
				MarkdownDescription: "Customer ID (only relevant for managed services edition).",
				Optional:            true,
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *PasswordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *PasswordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PasswordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.PasswordCreateRequest{
		Ident:    data.PasswordID.ValueString(),
		Title:    data.Title.ValueString(),
		Password: data.Password.ValueString(),
	}

	// Version-specific field handling:
	// - CheckMK 2.2: Uses "owner" (required), doesn't know "editable_by"
	// - CheckMK 2.3+: Uses "editable_by" (preferred), "owner" is deprecated
	editableValue := ""
	if !data.EditableBy.IsNull() && data.EditableBy.ValueString() != "" {
		editableValue = data.EditableBy.ValueString()
	} else if !data.Owner.IsNull() && data.Owner.ValueString() != "" {
		editableValue = data.Owner.ValueString()
	}

	if r.providerData.Client.Version.AtLeast(2, 3) {
		// CheckMK 2.3+: Use editable_by
		if editableValue != "" {
			createReq.EditableBy = editableValue
		}
	} else {
		// CheckMK 2.2: Use owner (required field)
		if editableValue != "" {
			createReq.Owner = editableValue
		} else {
			// Default to "admin" if not specified (owner is required in 2.2)
			createReq.Owner = "admin"
		}
	}

	if !data.ShareWith.IsNull() {
		var shareWith []string
		resp.Diagnostics.Append(data.ShareWith.ElementsAs(ctx, &shareWith, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Shared = shareWith
	}

	if !data.Comment.IsNull() {
		createReq.Comment = data.Comment.ValueString()
	}

	if !data.DocumentationURL.IsNull() {
		createReq.DocumentationURL = data.DocumentationURL.ValueString()
	}

	if !data.CustomerID.IsNull() {
		createReq.CustomerID = data.CustomerID.ValueString()
	}

	password, err := r.providerData.Client.CreatePassword(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create password: %s", err))
		return
	}

	// Update state from API response
	updatePasswordState(&data, password)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PasswordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	password, err := r.providerData.Client.GetPassword(ctx, data.PasswordID.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read password: %s", err))
		return
	}

	// Update state from API response (preserving sensitive password field)
	updatePasswordState(&data, password)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PasswordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetPasswordWithETag(ctx, data.PasswordID.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch password ETag: %s", err))
		return
	}

	updateReq := &client.PasswordUpdateRequest{}

	// Always include title if it changed
	if !data.Title.IsNull() {
		updateReq.Title = data.Title.ValueString()
	}

	// Always include password if it changed (it's required in the schema)
	if !data.Password.IsNull() {
		updateReq.Password = data.Password.ValueString()
	}

	// Version-specific field handling (same as Create)
	editableValue := ""
	if !data.EditableBy.IsNull() && data.EditableBy.ValueString() != "" {
		editableValue = data.EditableBy.ValueString()
	} else if !data.Owner.IsNull() && data.Owner.ValueString() != "" {
		editableValue = data.Owner.ValueString()
	}

	if r.providerData.Client.Version.AtLeast(2, 3) {
		// CheckMK 2.3+: Use editable_by
		if editableValue != "" {
			updateReq.EditableBy = editableValue
		}
	} else {
		// CheckMK 2.2: Use owner
		if editableValue != "" {
			updateReq.Owner = editableValue
		}
	}

	if !data.ShareWith.IsNull() {
		var shareWith []string
		resp.Diagnostics.Append(data.ShareWith.ElementsAs(ctx, &shareWith, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Shared = shareWith
	}

	if !data.Comment.IsNull() {
		updateReq.Comment = data.Comment.ValueString()
	}

	if !data.DocumentationURL.IsNull() {
		updateReq.DocumentationURL = data.DocumentationURL.ValueString()
	}

	if !data.CustomerID.IsNull() {
		updateReq.CustomerID = data.CustomerID.ValueString()
	}

	password, err := r.providerData.Client.UpdatePassword(ctx, data.PasswordID.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.PasswordID.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update password: %s", err))
			return
		}
	}

	if password != nil {
		updatePasswordState(&data, password)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PasswordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PasswordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetPasswordWithETag(ctx, data.PasswordID.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil // Already deleted
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch password ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeletePassword(ctx, data.PasswordID.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete password: %s", err))
		return
	}
}

func (r *PasswordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("password_id"), req, resp)
}

// updatePasswordState updates the resource model from API response
// Preserves sensitive password field from plan/state since API never returns it
func updatePasswordState(data *PasswordResourceModel, password *client.Password) {
	data.ID = types.StringValue(password.ID)
	data.PasswordID = types.StringValue(password.ID)
	data.Title = types.StringValue(password.Title)

	// Note: password.Password is never returned by the API (write-only)
	// We preserve the value from state/plan

	// The API returns owned_by which maps to both owner and editable_by
	// Set both to keep them synchronized
	if password.Extensions.Owner != "" {
		data.Owner = types.StringValue(password.Extensions.Owner)
		data.EditableBy = types.StringValue(password.Extensions.Owner)
	}

	if password.Extensions.Comment != "" {
		data.Comment = types.StringValue(password.Extensions.Comment)
	}

	if password.Extensions.DocumentationURL != "" {
		data.DocumentationURL = types.StringValue(password.Extensions.DocumentationURL)
	}

	if password.Extensions.CustomerID != "" {
		data.CustomerID = types.StringValue(password.Extensions.CustomerID)
	}

	// Note: ShareWith from API response may be in different order
	// We preserve state to avoid unnecessary diffs unless we want to reconcile
}

// ValidateConfig validates the resource configuration using generated types.
func (r *PasswordResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data PasswordResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against PasswordObject schema
	resp.Diagnostics.Append(validator.ValidateStringField("PasswordObject", "title", data.Title, path.Root("title"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("PasswordObject", "owner", data.Owner, path.Root("owner"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("PasswordObject", "editable_by", data.EditableBy, path.Root("editable_by"))...)
}
