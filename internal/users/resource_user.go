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
	_ resource.Resource                   = &UserResource{}
	_ resource.ResourceWithImportState    = &UserResource{}
	_ resource.ResourceWithValidateConfig = &UserResource{}
)

func NewUserResource() resource.Resource {
	return &UserResource{}
}

// UserResource defines the resource implementation.
type UserResource struct {
	providerData *common.ProviderData
}

// UserResourceModel describes the resource data model.
type UserResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Username              types.String `tfsdk:"username"`
	Fullname              types.String `tfsdk:"fullname"`
	Email                 types.String `tfsdk:"email"`
	ContactGroups         types.List   `tfsdk:"contact_groups"`
	Roles                 types.List   `tfsdk:"roles"`
	DisableLogin          types.Bool   `tfsdk:"disable_login"`
	AuthType              types.String `tfsdk:"auth_type"`
	Password              types.String `tfsdk:"password"`
	AutomationSecret      types.String `tfsdk:"automation_secret"`
	EnforcePasswordChange types.Bool   `tfsdk:"enforce_password_change"`
	StrictResourceLocking types.Bool   `tfsdk:"strict_resource_locking"`
}

func (r *UserResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK user. Users can authenticate via password or automation secret. Does not require activation.",
		Attributes: map[string]schema.Attribute{
			"id":       common.ComputedIDAttribute("The unique identifier for the user (same as username)."),
			"username": common.RequiredIDAttribute("Unique username for the user. Cannot be changed after creation."),
			"fullname": schema.StringAttribute{
				MarkdownDescription: "Full name of the user.",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address of the user.",
				Optional:            true,
			},
			"contact_groups": schema.ListAttribute{
				MarkdownDescription: "List of contact group names the user belongs to.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "List of roles assigned to the user. Common roles: 'admin', 'user', 'guest'.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"disable_login": schema.BoolAttribute{
				MarkdownDescription: "If true, the user cannot log in to the GUI.",
				Optional:            true,
			},
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "Authentication type: 'password' or 'automation'. Default is 'password'.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for the user. Required if auth_type is 'password'. Sensitive.",
				Optional:            true,
				Sensitive:           true,
			},
			"automation_secret": schema.StringAttribute{
				MarkdownDescription: "Automation secret for the user. Required if auth_type is 'automation'. Sensitive.",
				Optional:            true,
				Sensitive:           true,
			},
			"enforce_password_change": schema.BoolAttribute{
				MarkdownDescription: "If true, user must change password on next login. Only applies to password auth.",
				Optional:            true,
			},
			"strict_resource_locking": common.StrictResourceLockingAttribute(),
		},
	}
}

func (r *UserResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.UserCreateRequest{
		Username: data.Username.ValueString(),
		Fullname: data.Fullname.ValueString(),
	}

	if !data.Email.IsNull() {
		createReq.ContactOptions = &client.ContactOptions{Email: data.Email.ValueString()}
	}

	if !data.ContactGroups.IsNull() {
		var contactGroups []string
		resp.Diagnostics.Append(data.ContactGroups.ElementsAs(ctx, &contactGroups, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.ContactGroups = contactGroups
	}

	if !data.Roles.IsNull() {
		var roles []string
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Roles = roles
	}

	if !data.DisableLogin.IsNull() {
		createReq.DisableLogin = data.DisableLogin.ValueBool()
	}

	// Handle authentication
	authOption := buildAuthOption(data)
	if authOption != nil {
		createReq.AuthOption = authOption
	}

	user, err := r.providerData.Client.CreateUser(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user: %s", err))
		return
	}

	// Update state from API response
	updateUserState(&data, user)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.providerData.Client.GetUser(ctx, data.Username.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user: %s", err))
		return
	}

	// Update state from API response (preserving sensitive fields)
	updateUserState(&data, user)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetUserWithETag(ctx, data.Username.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch user ETag: %s", err))
		return
	}

	updateReq := &client.UserUpdateRequest{
		Fullname: data.Fullname.ValueString(),
	}

	if !data.Email.IsNull() {
		updateReq.ContactOptions = &client.ContactOptions{Email: data.Email.ValueString()}
	}

	if !data.ContactGroups.IsNull() {
		var contactGroups []string
		resp.Diagnostics.Append(data.ContactGroups.ElementsAs(ctx, &contactGroups, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.ContactGroups = contactGroups
	}

	if !data.Roles.IsNull() {
		var roles []string
		resp.Diagnostics.Append(data.Roles.ElementsAs(ctx, &roles, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Roles = roles
	}

	if !data.DisableLogin.IsNull() {
		disableLogin := data.DisableLogin.ValueBool()
		updateReq.DisableLogin = &disableLogin
	}

	// Handle authentication updates
	authOption := buildAuthOption(data)
	if authOption != nil {
		updateReq.AuthOption = authOption
	}

	user, err := r.providerData.Client.UpdateUser(ctx, data.Username.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.Username.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update user: %s", err))
			return
		}
	}

	if user != nil {
		updateUserState(&data, user)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prevent deletion of built-in users
	builtinUsers := map[string]bool{
		"cmkadmin":   true,
		"automation": true,
	}
	if builtinUsers[data.Username.ValueString()] {
		resp.Diagnostics.AddError(
			"Cannot Delete Built-in User",
			fmt.Sprintf("The '%s' user is a built-in user and cannot be deleted.", data.Username.ValueString()),
		)
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, data.StrictResourceLocking)

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetUserWithETag(ctx, data.Username.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch user ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteUser(ctx, data.Username.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user: %s", err))
		return
	}
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)
}

// buildAuthOption constructs an AuthOption from the resource model
func buildAuthOption(data UserResourceModel) *client.AuthOption {
	authType := data.AuthType.ValueString()
	if authType == "" && data.Password.IsNull() && data.AutomationSecret.IsNull() {
		return nil
	}

	authOption := &client.AuthOption{}

	if authType == "automation" || !data.AutomationSecret.IsNull() {
		authOption.AuthType = "automation"
		if !data.AutomationSecret.IsNull() {
			authOption.AutomationSecret = data.AutomationSecret.ValueString()
		}
	} else {
		authOption.AuthType = "password"
		if !data.Password.IsNull() {
			authOption.Password = data.Password.ValueString()
		}
		if !data.EnforcePasswordChange.IsNull() {
			authOption.EnforceChange = data.EnforcePasswordChange.ValueBool()
		}
	}

	return authOption
}

// updateUserState updates the resource model from API response
// Preserves sensitive fields (password, automation_secret) from plan/state
func updateUserState(data *UserResourceModel, user *client.User) {
	data.ID = types.StringValue(user.ID)
	data.Username = types.StringValue(user.ID)
	data.Fullname = types.StringValue(user.Extensions.Fullname)

	if user.Extensions.ContactOptions.Email != "" {
		data.Email = types.StringValue(user.Extensions.ContactOptions.Email)
	}

	// Note: Contact groups and roles from API may be in different order
	// We preserve state to avoid unnecessary diffs
	// The API returns these, but we only update if they were set

	if user.Extensions.AuthOption.AuthType != "" {
		data.AuthType = types.StringValue(user.Extensions.AuthOption.AuthType)
	}

	// Only set DisableLogin if it was in the config (not null)
	// to avoid showing diffs for default API values
	if !data.DisableLogin.IsNull() {
		data.DisableLogin = types.BoolValue(user.Extensions.DisableLogin)
	}
}

// ValidateConfig validates the resource configuration using generated types.
func (r *UserResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data UserResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against UserObject schema
	resp.Diagnostics.Append(validator.ValidateStringField("UserObject", "fullname", data.Fullname, path.Root("fullname"))...)
	resp.Diagnostics.Append(validator.ValidateStringField("UserObject", "auth_type", data.AuthType, path.Root("auth_type"))...)
}
