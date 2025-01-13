package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/exoscale/egoscale/v2/oapi"

	"github.com/exoscale/terraform-provider-exoscale/pkg/validators"
)

type ResourceRedisModel struct {
	IpFilter types.Set    `tfsdk:"ip_filter"`
	Settings types.String `tfsdk:"redis_settings"`
}

var ResourceRedisSchema = schema.SingleNestedBlock{
	MarkdownDescription: "*redis* database service type specific arguments. Structure is documented below.",
	Attributes: map[string]schema.Attribute{
		"ip_filter": schema.SetAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "A list of CIDR blocks to allow incoming connections from.",
			Optional:            true,
			Computed:            true,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(validators.IsCIDRNetworkValidator{Min: 0, Max: 128}),
			},
		},
		"redis_settings": schema.StringAttribute{
			MarkdownDescription: "Redis configuration settings in JSON format (`exo dbaas type show redis --settings=redis` for reference).",
			Optional:            true,
			Computed:            true,
		},
	},
}

// createRedis function handles Redis specific part of database resource creation logic.
func (r *ServiceResource) createRedis(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	service := oapi.CreateDbaasServiceRedisJSONRequestBody{
		Plan:                  data.Plan.ValueString(),
		TerminationProtection: data.TerminationProtection.ValueBoolPointer(),
	}

	if !data.MaintenanceDOW.IsUnknown() && !data.MaintenanceTime.IsUnknown() {
		service.Maintenance = &struct {
			Dow  oapi.CreateDbaasServiceRedisJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.CreateDbaasServiceRedisJSONBodyMaintenanceDow(data.MaintenanceDOW.ValueString()),
			Time: data.MaintenanceTime.ValueString(),
		}
	}

	if data.Redis != nil {
		if !data.Redis.IpFilter.IsUnknown() {
			obj := []string{}
			if len(data.Redis.IpFilter.Elements()) > 0 {
				dg := data.Redis.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}

			service.IpFilter = &obj
		}

		if !data.Redis.Settings.IsUnknown() {
			settingsSchema, err := r.client.GetDbaasSettingsRedisWithResponse(ctx)
			if err != nil {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
				return
			}
			if settingsSchema.StatusCode() != http.StatusOK {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
				return
			}

			obj, err := validateSettings(data.Redis.Settings.ValueString(), settingsSchema.JSON200.Settings.Redis)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			service.RedisSettings = &obj
		}
	}

	res, err := r.client.CreateDbaasServiceRedisWithResponse(
		ctx,
		oapi.DbaasServiceName(data.Name.ValueString()),
		service,
	)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service redis, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service redis, unexpected status: %s", res.Status()))
		return
	}

	r.readRedis(ctx, data, diagnostics)
}

// readRedis function handles Redis specific part of database resource Read logic.
// It is used in the dedicated Read action but also as a finishing step of Create, Update and Import.
func (r *ServiceResource) readRedis(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	res, err := r.client.GetDbaasServiceRedisWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service redis, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service redis, unexpected status: %s", res.Status()))
		return
	}

	apiService := res.JSON200

	data.CreatedAt = types.StringValue(apiService.CreatedAt.String())
	data.DiskSize = types.Int64PointerValue(apiService.DiskSize)
	data.NodeCPUs = types.Int64PointerValue(apiService.NodeCpuCount)
	data.NodeMemory = types.Int64PointerValue(apiService.NodeMemory)
	data.Nodes = types.Int64PointerValue(apiService.NodeCount)
	data.State = types.StringPointerValue((*string)(apiService.State))
	data.TerminationProtection = types.BoolPointerValue(apiService.TerminationProtection)
	data.UpdatedAt = types.StringValue(apiService.UpdatedAt.String())

	data.MaintenanceDOW = types.StringNull()
	data.MaintenanceTime = types.StringNull()
	if apiService.Maintenance != nil {
		data.MaintenanceDOW = types.StringValue(string(apiService.Maintenance.Dow))
		data.MaintenanceTime = types.StringValue(apiService.Maintenance.Time)
	}

	// Database block is required but it may be nil during import.
	if data.Redis == nil {
		data.Redis = &ResourceRedisModel{}
	}

	data.Redis.IpFilter = types.SetNull(types.StringType)
	if apiService.IpFilter != nil {
		v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
		if dg.HasError() {
			diagnostics.Append(dg...)
			return
		}

		data.Redis.IpFilter = v
	}

	data.Redis.Settings = types.StringNull()
	if apiService.RedisSettings != nil {
		settings, err := json.Marshal(*apiService.RedisSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Redis.Settings = types.StringValue(string(settings))
	}
}

// updateRedis function handles Redis specific part of database resource Update logic.
func (r *ServiceResource) updateRedis(ctx context.Context, stateData *ServiceResourceModel, planData *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	var updated bool

	service := oapi.UpdateDbaasServiceRedisJSONRequestBody{}

	if (!planData.MaintenanceDOW.Equal(stateData.MaintenanceDOW) && !planData.MaintenanceDOW.IsUnknown()) ||
		(!planData.MaintenanceTime.Equal(stateData.MaintenanceTime) && !planData.MaintenanceTime.IsUnknown()) {
		service.Maintenance = &struct {
			Dow  oapi.UpdateDbaasServiceRedisJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.UpdateDbaasServiceRedisJSONBodyMaintenanceDow(planData.MaintenanceDOW.ValueString()),
			Time: planData.MaintenanceTime.ValueString(),
		}
		updated = true
	}

	if !planData.Plan.Equal(stateData.Plan) {
		service.Plan = planData.Plan.ValueStringPointer()
		updated = true
	}

	if !planData.TerminationProtection.Equal(stateData.TerminationProtection) {
		service.TerminationProtection = planData.TerminationProtection.ValueBoolPointer()
		updated = true
	}

	if planData.Redis != nil {
		if stateData.Redis == nil {
			stateData.Redis = &ResourceRedisModel{}
		}

		if !planData.Redis.IpFilter.Equal(stateData.Redis.IpFilter) {
			obj := []string{}
			if len(planData.Redis.IpFilter.Elements()) > 0 {
				dg := planData.Redis.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}
			service.IpFilter = &obj
			updated = true
		}

		if !planData.Redis.Settings.Equal(stateData.Redis.Settings) {
			settingsSchema, err := r.client.GetDbaasSettingsRedisWithResponse(ctx)
			if err != nil {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
				return
			}
			if settingsSchema.StatusCode() != http.StatusOK {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
				return
			}

			if planData.Redis.Settings.ValueString() != "" {
				obj, err := validateSettings(planData.Redis.Settings.ValueString(), settingsSchema.JSON200.Settings.Redis)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Redis settings: %s", err))
					return
				}
				service.RedisSettings = &obj
			}
			updated = true
		}
	}

	if !updated {
		tflog.Info(ctx, "no updates detected", map[string]interface{}{})
	} else {
		res, err := r.client.UpdateDbaasServiceRedisWithResponse(
			ctx,
			oapi.DbaasServiceName(planData.Id.ValueString()),
			service,
		)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service redis, got error: %s", err))
			return
		}
		if res.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service redis, unexpected status: %s", res.Status()))
			return
		}
	}

	r.readRedis(ctx, planData, diagnostics)
}
