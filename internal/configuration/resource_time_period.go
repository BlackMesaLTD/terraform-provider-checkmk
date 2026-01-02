package configuration

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/terraform-provider-checkmk/internal/client"
	"github.com/terraform-provider-checkmk/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &TimePeriodResource{}
	_ resource.ResourceWithImportState    = &TimePeriodResource{}
	_ resource.ResourceWithValidateConfig = &TimePeriodResource{}
)

func NewTimePeriodResource() resource.Resource {
	return &TimePeriodResource{}
}

// TimePeriodResource defines the resource implementation.
type TimePeriodResource struct {
	providerData *common.ProviderData
}

// TimePeriodResourceModel describes the resource data model.
type TimePeriodResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Alias            types.String `tfsdk:"alias"`
	ActiveTimeRanges types.List   `tfsdk:"active_time_ranges"`
	Exceptions       types.List   `tfsdk:"exceptions"`
	Exclude          types.List   `tfsdk:"exclude"`
}

func (r *TimePeriodResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_period"
}

func (r *TimePeriodResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a CheckMK time period. Time periods define when notifications and checks are active. Requires activation.",
		Attributes: map[string]schema.Attribute{
			"id":   common.ComputedIDAttribute("The unique identifier for the time period (same as name)."),
			"name": common.RequiredIDAttribute("Unique identifier for the time period. Cannot be changed after creation."),
			"alias": schema.StringAttribute{
				MarkdownDescription: "Human-readable alias/title for the time period.",
				Required:            true,
			},
			"active_time_ranges": schema.ListNestedAttribute{
				MarkdownDescription: "List of active time ranges per day of the week.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"day": schema.StringAttribute{
							MarkdownDescription: "Day of the week: monday, tuesday, wednesday, thursday, friday, saturday, sunday.",
							Required:            true,
						},
						"time_ranges": schema.ListNestedAttribute{
							MarkdownDescription: "List of time ranges for this day.",
							Required:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"start": schema.StringAttribute{
										MarkdownDescription: "Start time in HH:MM format (e.g., '09:00').",
										Required:            true,
									},
									"end": schema.StringAttribute{
										MarkdownDescription: "End time in HH:MM format (e.g., '17:00').",
										Required:            true,
									},
								},
							},
						},
					},
				},
			},
			"exceptions": schema.ListNestedAttribute{
				MarkdownDescription: "List of exception dates (e.g., holidays) with optional time ranges.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"date": schema.StringAttribute{
							MarkdownDescription: "Date in YYYY-MM-DD format (e.g., '2024-12-25').",
							Required:            true,
						},
						"time_ranges": schema.ListNestedAttribute{
							MarkdownDescription: "Time ranges for this exception date. Empty list means inactive all day.",
							Optional:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"start": schema.StringAttribute{
										MarkdownDescription: "Start time in HH:MM format.",
										Required:            true,
									},
									"end": schema.StringAttribute{
										MarkdownDescription: "End time in HH:MM format.",
										Required:            true,
									},
								},
							},
						},
					},
				},
			},
			"exclude": schema.ListAttribute{
				MarkdownDescription: "List of other time period names to exclude from this time period.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TimePeriodResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.providerData = common.ConfigureResource(req, resp)
}

// convertActiveTimeRangesToClient converts Terraform active_time_ranges to client format
func convertActiveTimeRangesToClient(ctx context.Context, rangesList types.List) ([]client.ActiveTimeDay, error) {
	if rangesList.IsNull() || rangesList.IsUnknown() {
		return nil, nil
	}

	var result []client.ActiveTimeDay
	var rangesData []struct {
		Day        types.String `tfsdk:"day"`
		TimeRanges types.List   `tfsdk:"time_ranges"`
	}

	diags := rangesList.ElementsAs(ctx, &rangesData, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert active_time_ranges: %v", diags)
	}

	for _, dayData := range rangesData {
		day := client.ActiveTimeDay{
			Day: dayData.Day.ValueString(),
		}

		// Convert time_ranges
		var timeRangesData []struct {
			Start types.String `tfsdk:"start"`
			End   types.String `tfsdk:"end"`
		}
		diags := dayData.TimeRanges.ElementsAs(ctx, &timeRangesData, false)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert time_ranges: %v", diags)
		}

		for _, tr := range timeRangesData {
			day.TimeRanges = append(day.TimeRanges, client.TimeRange{
				Start: tr.Start.ValueString(),
				End:   tr.End.ValueString(),
			})
		}

		result = append(result, day)
	}

	return result, nil
}

// convertExceptionsToClient converts Terraform exceptions to client format
func convertExceptionsToClient(ctx context.Context, exceptionsList types.List) ([]client.TimePeriodDate, error) {
	if exceptionsList.IsNull() || exceptionsList.IsUnknown() {
		return nil, nil
	}

	var result []client.TimePeriodDate
	var exceptionsData []struct {
		Date       types.String `tfsdk:"date"`
		TimeRanges types.List   `tfsdk:"time_ranges"`
	}

	diags := exceptionsList.ElementsAs(ctx, &exceptionsData, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert exceptions: %v", diags)
	}

	for _, excData := range exceptionsData {
		exc := client.TimePeriodDate{
			Date: excData.Date.ValueString(),
		}

		// Convert time_ranges if present
		if !excData.TimeRanges.IsNull() && !excData.TimeRanges.IsUnknown() {
			var timeRangesData []struct {
				Start types.String `tfsdk:"start"`
				End   types.String `tfsdk:"end"`
			}
			diags := excData.TimeRanges.ElementsAs(ctx, &timeRangesData, false)
			if diags.HasError() {
				return nil, fmt.Errorf("failed to convert exception time_ranges: %v", diags)
			}

			for _, tr := range timeRangesData {
				exc.TimeRanges = append(exc.TimeRanges, client.TimeRange{
					Start: tr.Start.ValueString(),
					End:   tr.End.ValueString(),
				})
			}
		}

		result = append(result, exc)
	}

	return result, nil
}

// convertExcludeToClient converts Terraform exclude list to client format
func convertExcludeToClient(ctx context.Context, excludeList types.List) ([]string, error) {
	if excludeList.IsNull() || excludeList.IsUnknown() {
		return nil, nil
	}

	var result []string
	diags := excludeList.ElementsAs(ctx, &result, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert exclude: %v", diags)
	}

	return result, nil
}

// Time range nested object type for Terraform
func timeRangeObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"start": types.StringType,
			"end":   types.StringType,
		},
	}
}

// Active time day nested object type for Terraform
func activeTimeDayObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"day":         types.StringType,
			"time_ranges": types.ListType{ElemType: timeRangeObjectType()},
		},
	}
}

// Exception date nested object type for Terraform
func exceptionDateObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"date":        types.StringType,
			"time_ranges": types.ListType{ElemType: timeRangeObjectType()},
		},
	}
}

// convertActiveTimeRangesFromClient converts client active_time_ranges to Terraform format
func convertActiveTimeRangesFromClient(ctx context.Context, ranges []client.ActiveTimeDay) (types.List, error) {
	if len(ranges) == 0 {
		return types.ListNull(activeTimeDayObjectType()), nil
	}

	dayObjects := make([]attr.Value, len(ranges))
	for i, day := range ranges {
		// Convert time ranges
		trObjects := make([]attr.Value, len(day.TimeRanges))
		for j, tr := range day.TimeRanges {
			trObj, diags := types.ObjectValue(
				timeRangeObjectType().AttrTypes,
				map[string]attr.Value{
					"start": types.StringValue(tr.Start),
					"end":   types.StringValue(tr.End),
				},
			)
			if diags.HasError() {
				return types.ListNull(activeTimeDayObjectType()), fmt.Errorf("failed to create time_range object: %v", diags)
			}
			trObjects[j] = trObj
		}

		trList, diags := types.ListValue(timeRangeObjectType(), trObjects)
		if diags.HasError() {
			return types.ListNull(activeTimeDayObjectType()), fmt.Errorf("failed to create time_ranges list: %v", diags)
		}

		dayObj, diags := types.ObjectValue(
			activeTimeDayObjectType().AttrTypes,
			map[string]attr.Value{
				"day":         types.StringValue(day.Day),
				"time_ranges": trList,
			},
		)
		if diags.HasError() {
			return types.ListNull(activeTimeDayObjectType()), fmt.Errorf("failed to create day object: %v", diags)
		}
		dayObjects[i] = dayObj
	}

	result, diags := types.ListValue(activeTimeDayObjectType(), dayObjects)
	if diags.HasError() {
		return types.ListNull(activeTimeDayObjectType()), fmt.Errorf("failed to create active_time_ranges list: %v", diags)
	}

	return result, nil
}

// convertExceptionsFromClient converts client exceptions to Terraform format
func convertExceptionsFromClient(ctx context.Context, exceptions []client.TimePeriodDate) (types.List, error) {
	if len(exceptions) == 0 {
		return types.ListNull(exceptionDateObjectType()), nil
	}

	excObjects := make([]attr.Value, len(exceptions))
	for i, exc := range exceptions {
		// Convert time ranges
		trObjects := make([]attr.Value, len(exc.TimeRanges))
		for j, tr := range exc.TimeRanges {
			trObj, diags := types.ObjectValue(
				timeRangeObjectType().AttrTypes,
				map[string]attr.Value{
					"start": types.StringValue(tr.Start),
					"end":   types.StringValue(tr.End),
				},
			)
			if diags.HasError() {
				return types.ListNull(exceptionDateObjectType()), fmt.Errorf("failed to create time_range object: %v", diags)
			}
			trObjects[j] = trObj
		}

		trList, diags := types.ListValue(timeRangeObjectType(), trObjects)
		if diags.HasError() {
			return types.ListNull(exceptionDateObjectType()), fmt.Errorf("failed to create time_ranges list: %v", diags)
		}

		excObj, diags := types.ObjectValue(
			exceptionDateObjectType().AttrTypes,
			map[string]attr.Value{
				"date":        types.StringValue(exc.Date),
				"time_ranges": trList,
			},
		)
		if diags.HasError() {
			return types.ListNull(exceptionDateObjectType()), fmt.Errorf("failed to create exception object: %v", diags)
		}
		excObjects[i] = excObj
	}

	result, diags := types.ListValue(exceptionDateObjectType(), excObjects)
	if diags.HasError() {
		return types.ListNull(exceptionDateObjectType()), fmt.Errorf("failed to create exceptions list: %v", diags)
	}

	return result, nil
}

func (r *TimePeriodResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TimePeriodResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	// Convert nested structures
	activeTimeRanges, err := convertActiveTimeRangesToClient(ctx, data.ActiveTimeRanges)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert active_time_ranges: %s", err))
		return
	}

	exceptions, err := convertExceptionsToClient(ctx, data.Exceptions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exceptions: %s", err))
		return
	}

	exclude, err := convertExcludeToClient(ctx, data.Exclude)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exclude: %s", err))
		return
	}

	createReq := &client.TimePeriodCreateRequest{
		Name:             data.Name.ValueString(),
		Alias:            data.Alias.ValueString(),
		ActiveTimeRanges: activeTimeRanges,
		Exceptions:       exceptions,
		Exclude:          exclude,
	}

	timePeriod, err := r.providerData.Client.CreateTimePeriod(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create time period: %s", err))
		return
	}

	// Update state from API response
	data.ID = types.StringValue(timePeriod.ID)
	data.Name = types.StringValue(timePeriod.ID)
	data.Alias = types.StringValue(timePeriod.Extensions.Alias)

	activeRangesList, err := convertActiveTimeRangesFromClient(ctx, timePeriod.Extensions.ActiveTimeRanges)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert active_time_ranges from API: %s", err))
		return
	}
	data.ActiveTimeRanges = activeRangesList

	exceptionsList, err := convertExceptionsFromClient(ctx, timePeriod.Extensions.Exceptions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exceptions from API: %s", err))
		return
	}
	data.Exceptions = exceptionsList

	if len(timePeriod.Extensions.Exclude) > 0 {
		excludeList, diags := types.ListValueFrom(ctx, types.StringType, timePeriod.Extensions.Exclude)
		resp.Diagnostics.Append(diags...)
		data.Exclude = excludeList
	} else {
		data.Exclude = types.ListNull(types.StringType)
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "time_period"); err != nil {
		common.AddActivationWarning(resp, "Time period", "created", err)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TimePeriodResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TimePeriodResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timePeriod, err := r.providerData.Client.GetTimePeriod(ctx, data.Name.ValueString())
	if err != nil {
		if common.HandleNotFoundOnRead(err, resp) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read time period: %s", err))
		return
	}

	// Update state from API response
	data.ID = types.StringValue(timePeriod.ID)
	data.Name = types.StringValue(timePeriod.ID)
	data.Alias = types.StringValue(timePeriod.Extensions.Alias)

	activeRangesList, err := convertActiveTimeRangesFromClient(ctx, timePeriod.Extensions.ActiveTimeRanges)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert active_time_ranges from API: %s", err))
		return
	}
	data.ActiveTimeRanges = activeRangesList

	exceptionsList, err := convertExceptionsFromClient(ctx, timePeriod.Extensions.Exceptions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exceptions from API: %s", err))
		return
	}
	data.Exceptions = exceptionsList

	if len(timePeriod.Extensions.Exclude) > 0 {
		excludeList, diags := types.ListValueFrom(ctx, types.StringType, timePeriod.Extensions.Exclude)
		resp.Diagnostics.Append(diags...)
		data.Exclude = excludeList
	} else {
		data.Exclude = types.ListNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TimePeriodResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TimePeriodResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetTimePeriodWithETag(ctx, data.Name.ValueString())
		if err != nil {
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch time period ETag: %s", err))
		return
	}

	// Convert nested structures
	activeTimeRanges, err := convertActiveTimeRangesToClient(ctx, data.ActiveTimeRanges)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert active_time_ranges: %s", err))
		return
	}

	exceptions, err := convertExceptionsToClient(ctx, data.Exceptions)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exceptions: %s", err))
		return
	}

	exclude, err := convertExcludeToClient(ctx, data.Exclude)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exclude: %s", err))
		return
	}

	updateReq := &client.TimePeriodUpdateRequest{
		Alias:            data.Alias.ValueString(),
		ActiveTimeRanges: activeTimeRanges,
		Exceptions:       exceptions,
		Exclude:          exclude,
	}

	timePeriod, err := r.providerData.Client.UpdateTimePeriod(ctx, data.Name.ValueString(), updateReq, etag)
	if err != nil {
		if !common.HandleDriftWarning(err, resp, data.Name.ValueString()) {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update time period: %s", err))
			return
		}
	}

	// Update state from API response
	if timePeriod != nil {
		data.ID = types.StringValue(timePeriod.ID)
		data.Name = types.StringValue(timePeriod.ID)
		data.Alias = types.StringValue(timePeriod.Extensions.Alias)

		activeRangesList, err := convertActiveTimeRangesFromClient(ctx, timePeriod.Extensions.ActiveTimeRanges)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert active_time_ranges from API: %s", err))
			return
		}
		data.ActiveTimeRanges = activeRangesList

		exceptionsList, err := convertExceptionsFromClient(ctx, timePeriod.Extensions.Exceptions)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert exceptions from API: %s", err))
			return
		}
		data.Exceptions = exceptionsList

		if len(timePeriod.Extensions.Exclude) > 0 {
			excludeList, diags := types.ListValueFrom(ctx, types.StringType, timePeriod.Extensions.Exclude)
			resp.Diagnostics.Append(diags...)
			data.Exclude = excludeList
		} else {
			data.Exclude = types.ListNull(types.StringType)
		}
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "time_period"); err != nil {
		common.AddActivationWarning(resp, "Time period", "updated", err)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TimePeriodResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TimePeriodResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check for builtin time period
	if data.Name.ValueString() == "24X7" {
		resp.Diagnostics.AddError(
			"Cannot Delete Builtin Time Period",
			"The '24X7' time period is a builtin time period and cannot be deleted.",
		)
		return
	}

	cfg := common.BuildSimpleBaseConfig(r.providerData, types.BoolNull())

	etag, err := common.FetchETagIfStrict(ctx, cfg.StrictResourceLocking, func(ctx context.Context) (string, error) {
		result, err := r.providerData.Client.GetTimePeriodWithETag(ctx, data.Name.ValueString())
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Status == 404 {
				return "", nil
			}
			return "", err
		}
		return result.ETag, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch time period ETag: %s", err))
		return
	}

	if err := r.providerData.Client.DeleteTimePeriod(ctx, data.Name.ValueString(), etag); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete time period: %s", err))
		return
	}

	if err := common.TrackAndActivate(ctx, r.providerData, cfg, "time_period"); err != nil {
		common.AddActivationWarning(resp, "Time period", "deleted", err)
	}
}

func (r *TimePeriodResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by time period name - set both id and name
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// ValidateConfig validates the resource configuration using generated types.
func (r *TimePeriodResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if r.providerData == nil {
		return
	}

	var data TimePeriodResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validator := common.NewAttributeValidator(r.providerData)

	// Validate fields against TimePeriod schema
	resp.Diagnostics.Append(validator.ValidateStringField("TimePeriod", "alias", data.Alias, path.Root("alias"))...)
}
