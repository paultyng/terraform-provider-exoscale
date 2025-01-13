package database

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/exoscale/egoscale/v2/oapi"

	"github.com/exoscale/terraform-provider-exoscale/pkg/validators"
)

type ResourcePgModel struct {
	AdminPassword     types.String `tfsdk:"admin_password"`
	AdminUsername     types.String `tfsdk:"admin_username"`
	BackupSchedule    types.String `tfsdk:"backup_schedule"`
	IpFilter          types.Set    `tfsdk:"ip_filter"`
	Settings          types.String `tfsdk:"pg_settings"`
	Version           types.String `tfsdk:"version"`
	PgbouncerSettings types.String `tfsdk:"pgbouncer_settings"`
	PglookoutSettings types.String `tfsdk:"pglookout_settings"`
}

var ResourcePgSchema = schema.SingleNestedBlock{
	MarkdownDescription: "*pg* database service type specific arguments. Structure is documented below.",
	Attributes: map[string]schema.Attribute{
		"admin_password": schema.StringAttribute{
			MarkdownDescription: "A custom administrator account password (may only be set at creation time).",
			Optional:            true,
			Sensitive:           true,
		},
		"admin_username": schema.StringAttribute{
			MarkdownDescription: "A custom administrator account username (may only be set at creation time).",
			Optional:            true,
		},
		"backup_schedule": schema.StringAttribute{
			MarkdownDescription: "The automated backup schedule (`HH:MM`).",
			Optional:            true,
			Computed:            true,
		},
		"ip_filter": schema.SetAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "A list of CIDR blocks to allow incoming connections from.",
			Optional:            true,
			Computed:            true,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(validators.IsCIDRNetworkValidator{Min: 0, Max: 128}),
			},
		},
		"pg_settings": schema.StringAttribute{
			MarkdownDescription: "PostgreSQL configuration settings in JSON format (`exo dbaas type show pg --settings=pg` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"version": schema.StringAttribute{
			MarkdownDescription: "PostgreSQL major version (`exo dbaas type show pg` for reference; may only be set at creation time).",
			Optional:            true,
			Computed:            true,
		},
		"pgbouncer_settings": schema.StringAttribute{
			MarkdownDescription: "PgBouncer configuration settings in JSON format (`exo dbaas type show pg --settings=pgbouncer` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"pglookout_settings": schema.StringAttribute{
			MarkdownDescription: "pglookout configuration settings in JSON format (`exo dbaas type show pg --settings=pglookout` for reference).",
			Optional:            true,
			Computed:            true,
		},
	},
}

// createPg function handles PostgreSQL specific part of database resource creation logic.
func (r *ServiceResource) createPg(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	service := oapi.CreateDbaasServicePgJSONRequestBody{
		Plan:                  data.Plan.ValueString(),
		TerminationProtection: data.TerminationProtection.ValueBoolPointer(),
	}

	if !data.MaintenanceDOW.IsUnknown() && !data.MaintenanceTime.IsUnknown() {
		service.Maintenance = &struct {
			Dow  oapi.CreateDbaasServicePgJSONBodyMaintenanceDow `json:"dow"`
			Time string                                          `json:"time"`
		}{
			Dow:  oapi.CreateDbaasServicePgJSONBodyMaintenanceDow(data.MaintenanceDOW.ValueString()),
			Time: data.MaintenanceTime.ValueString(),
		}
	}

	if data.Pg != nil {
		if !data.Pg.Version.IsUnknown() {
			service.Version = data.Pg.Version.ValueStringPointer()
		}

		if !data.Pg.AdminPassword.IsNull() {
			service.AdminPassword = data.Pg.AdminPassword.ValueStringPointer()
		}

		if !data.Pg.AdminUsername.IsNull() {
			service.AdminUsername = data.Pg.AdminUsername.ValueStringPointer()
		}

		if !data.Pg.IpFilter.IsUnknown() {
			obj := []string{}
			if len(data.Pg.IpFilter.Elements()) > 0 {
				dg := data.Pg.IpFilter.ElementsAs(ctx, &obj, true)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}

			service.IpFilter = &obj
		}
		if !data.Pg.BackupSchedule.IsUnknown() {
			bh, bm, err := parseBackupSchedule(data.Pg.BackupSchedule.ValueString())
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("Unable to parse backup schedule, got error: %s", err))
				return
			}

			service.BackupSchedule = &struct {
				BackupHour   *int64 `json:"backup-hour,omitempty"`
				BackupMinute *int64 `json:"backup-minute,omitempty"`
			}{
				BackupHour:   &bh,
				BackupMinute: &bm,
			}
		}

		settingsSchema, err := r.client.GetDbaasSettingsPgWithResponse(ctx)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
			return
		}
		if settingsSchema.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
			return
		}

		if !data.Pg.Settings.IsUnknown() {
			obj, err := validateSettings(data.Pg.Settings.ValueString(), settingsSchema.JSON200.Settings.Pg)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			service.PgSettings = &obj
		}

		if !data.Pg.PgbouncerSettings.IsUnknown() {
			obj, err := validateSettings(data.Pg.PgbouncerSettings.ValueString(), settingsSchema.JSON200.Settings.Pgbouncer)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			service.PgbouncerSettings = &obj
		}

		if !data.Pg.PglookoutSettings.IsUnknown() {
			obj, err := validateSettings(data.Pg.PglookoutSettings.ValueString(), settingsSchema.JSON200.Settings.Pglookout)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			service.PglookoutSettings = &obj
		}
	}

	resp, err := r.client.CreateDbaasServicePgWithResponse(
		ctx,
		oapi.DbaasServiceName(data.Name.ValueString()),
		service,
	)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service pg, got error: %s", err))
		return
	}
	if resp.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable create database service pg, unexpected status: %s", resp.Status()))
		return
	}

	res, err := r.client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service pg, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service pg, unexpected status: %s", res.Status()))
		return
	}

	apiService := res.JSON200

	// Set computed attributes.
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	data.CreatedAt = types.StringValue(apiService.CreatedAt.String())
	data.DiskSize = types.Int64PointerValue(apiService.DiskSize)
	data.NodeCPUs = types.Int64PointerValue(apiService.NodeCpuCount)
	data.NodeMemory = types.Int64PointerValue(apiService.NodeMemory)
	data.Nodes = types.Int64PointerValue(apiService.NodeCount)
	data.State = types.StringPointerValue((*string)(apiService.State))
	data.UpdatedAt = types.StringValue(apiService.UpdatedAt.String())

	if data.TerminationProtection.IsUnknown() {
		data.TerminationProtection = types.BoolPointerValue(apiService.TerminationProtection)
	}

	if data.MaintenanceDOW.IsUnknown() && data.MaintenanceTime.IsUnknown() {
		data.MaintenanceDOW = types.StringNull()
		data.MaintenanceTime = types.StringNull()
		if apiService.Maintenance != nil {
			data.MaintenanceDOW = types.StringValue(string(apiService.Maintenance.Dow))
			data.MaintenanceTime = types.StringValue(apiService.Maintenance.Time)
		}
	}

	if data.Pg.BackupSchedule.IsUnknown() {
		data.Pg.BackupSchedule = types.StringNull()
		if apiService.BackupSchedule != nil {
			backupHour := types.Int64PointerValue(apiService.BackupSchedule.BackupHour)
			backupMinute := types.Int64PointerValue(apiService.BackupSchedule.BackupMinute)
			data.Pg.BackupSchedule = types.StringValue(fmt.Sprintf(
				"%02d:%02d",
				backupHour.ValueInt64(),
				backupMinute.ValueInt64(),
			))
		}
	}

	if data.Pg.IpFilter.IsUnknown() {
		data.Pg.IpFilter = types.SetNull(types.StringType)
		if apiService.IpFilter != nil {
			v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
			if dg.HasError() {
				diagnostics.Append(dg...)
				return
			}

			data.Pg.IpFilter = v
		}
	}

	if data.Pg.Version.IsUnknown() {
		data.Pg.Version = types.StringNull()
		if apiService.Version != nil {
			data.Pg.Version = types.StringValue(strings.SplitN(*apiService.Version, ".", 2)[0])
		}
	}

	if data.Pg.Settings.IsUnknown() {
		data.Pg.Settings = types.StringNull()
		if apiService.PgSettings != nil {
			settings, err := json.Marshal(*apiService.PgSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			data.Pg.Settings = types.StringValue(string(settings))
		}
	}

	if data.Pg.PgbouncerSettings.IsUnknown() {
		data.Pg.PgbouncerSettings = types.StringNull()
		if apiService.PgbouncerSettings != nil {
			settings, err := json.Marshal(*apiService.PgbouncerSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			data.Pg.PgbouncerSettings = types.StringValue(string(settings))
		}
	}

	if data.Pg.PglookoutSettings.IsUnknown() {
		data.Pg.PglookoutSettings = types.StringNull()
		if apiService.PglookoutSettings != nil {
			settings, err := json.Marshal(*apiService.PglookoutSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}
			data.Pg.PglookoutSettings = types.StringValue(string(settings))
		}
	}
}

// readPg function handles PostgreSQL specific part of database resource Read logic.
// It is used in the dedicated Read action but also as a finishing step of Create, Update and Import.
func (r *ServiceResource) readPg(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	res, err := r.client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service pg, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service pg, unexpected status: %s", res.Status()))
		return
	}

	apiService := res.JSON200

	data.CreatedAt = types.StringValue(apiService.CreatedAt.String())
	data.DiskSize = types.Int64PointerValue(apiService.DiskSize)
	data.NodeCPUs = types.Int64PointerValue(apiService.NodeCpuCount)
	data.NodeMemory = types.Int64PointerValue(apiService.NodeMemory)
	data.Nodes = types.Int64PointerValue(apiService.NodeCount)
	data.Plan = types.StringValue(apiService.Plan)
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
	if data.Pg == nil {
		data.Pg = &ResourcePgModel{}
	}

	data.Pg.BackupSchedule = types.StringNull()
	if apiService.BackupSchedule != nil {
		backupHour := types.Int64PointerValue(apiService.BackupSchedule.BackupHour)
		backupMinute := types.Int64PointerValue(apiService.BackupSchedule.BackupMinute)
		data.Pg.BackupSchedule = types.StringValue(fmt.Sprintf(
			"%02d:%02d",
			backupHour.ValueInt64(),
			backupMinute.ValueInt64(),
		))
	}

	data.Pg.IpFilter = types.SetNull(types.StringType)
	if apiService.IpFilter != nil {
		v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
		if dg.HasError() {
			diagnostics.Append(dg...)
			return
		}

		data.Pg.IpFilter = v
	}

	data.Pg.Version = types.StringNull()
	if apiService.Version != nil {
		data.Pg.Version = types.StringValue(strings.SplitN(*apiService.Version, ".", 2)[0])
	}

	// For database settings, we have a special behaviour:
	// - If not set in plan we handle it as normal computed field;
	// - If set in plan we only read (manage) key(s) that were set!
	//   This prevents showing drift due to Aiven setting default values for keys not specified in plan.
	if data.Pg.Settings.IsUnknown() || apiService.PgSettings == nil {
		data.Pg.Settings = types.StringNull()
		if apiService.PgSettings != nil {
			settings, err := json.Marshal(*apiService.PgSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}

			data.Pg.Settings = types.StringValue(string(settings))
		}
	} else if data.Pg.Settings.ValueString() != "" {
		var userSettings map[string]interface{}

		if err := json.Unmarshal([]byte(data.Pg.Settings.ValueString()), &userSettings); err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("unable to unmarshal JSON: %s", err))
			return
		}

		PartialSettingsPatch(userSettings, *apiService.PgSettings)
		settings, err := json.Marshal(userSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Pg.Settings = types.StringValue(string(settings))
	}

	if data.Pg.PgbouncerSettings.IsUnknown() || apiService.PgbouncerSettings == nil {
		data.Pg.PgbouncerSettings = types.StringNull()
		if apiService.PgbouncerSettings != nil {
			settings, err := json.Marshal(*apiService.PgbouncerSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}

			data.Pg.PgbouncerSettings = types.StringValue(string(settings))
		}
	} else if data.Pg.PgbouncerSettings.ValueString() != "" {
		var userSettings map[string]interface{}

		if err := json.Unmarshal([]byte(data.Pg.PgbouncerSettings.ValueString()), &userSettings); err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("unable to unmarshal JSON: %s", err))
			return
		}

		PartialSettingsPatch(userSettings, *apiService.PgbouncerSettings)
		settings, err := json.Marshal(userSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Pg.PgbouncerSettings = types.StringValue(string(settings))
	}

	if data.Pg.PglookoutSettings.IsUnknown() || apiService.PglookoutSettings == nil {
		data.Pg.PglookoutSettings = types.StringNull()
		if apiService.PglookoutSettings != nil {
			settings, err := json.Marshal(*apiService.PglookoutSettings)
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
				return
			}

			data.Pg.PglookoutSettings = types.StringValue(string(settings))
		}
	} else if data.Pg.PglookoutSettings.ValueString() != "" {
		var userSettings map[string]interface{}

		if err := json.Unmarshal([]byte(data.Pg.PglookoutSettings.ValueString()), &userSettings); err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("unable to unmarshal JSON: %s", err))
			return
		}

		PartialSettingsPatch(userSettings, *apiService.PglookoutSettings)
		settings, err := json.Marshal(userSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Pg.PglookoutSettings = types.StringValue(string(settings))
	}
}

// updatePg function handles PostgreSQL specific part of database resource Update logic.
func (r *ServiceResource) updatePg(ctx context.Context, stateData *ServiceResourceModel, planData *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	var updated bool

	service := oapi.UpdateDbaasServicePgJSONRequestBody{}

	if (!planData.MaintenanceDOW.Equal(stateData.MaintenanceDOW) && !planData.MaintenanceDOW.IsUnknown()) ||
		(!planData.MaintenanceTime.Equal(stateData.MaintenanceTime) && !planData.MaintenanceTime.IsUnknown()) {
		service.Maintenance = &struct {
			Dow  oapi.UpdateDbaasServicePgJSONBodyMaintenanceDow `json:"dow"`
			Time string                                          `json:"time"`
		}{
			Dow:  oapi.UpdateDbaasServicePgJSONBodyMaintenanceDow(planData.MaintenanceDOW.ValueString()),
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

	if planData.Pg != nil {
		if stateData.Pg == nil {
			stateData.Pg = &ResourcePgModel{}
		}

		if !planData.Pg.BackupSchedule.Equal(stateData.Pg.BackupSchedule) {
			bh, bm, err := parseBackupSchedule(planData.Pg.BackupSchedule.ValueString())
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("Unable to parse backup schedule, got error: %s", err))
				return
			}

			service.BackupSchedule = &struct {
				BackupHour   *int64 `json:"backup-hour,omitempty"`
				BackupMinute *int64 `json:"backup-minute,omitempty"`
			}{
				BackupHour:   &bh,
				BackupMinute: &bm,
			}
			updated = true
		}

		if !planData.Pg.IpFilter.Equal(stateData.Pg.IpFilter) {
			obj := []string{}
			if len(planData.Pg.IpFilter.Elements()) > 0 {
				dg := planData.Pg.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}
			service.IpFilter = &obj
			updated = true
		}

		settingsSchema, err := r.client.GetDbaasSettingsPgWithResponse(ctx)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
			return
		}
		if settingsSchema.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
			return
		}

		if !planData.Pg.Settings.Equal(stateData.Pg.Settings) {
			if planData.Pg.Settings.ValueString() != "" {
				obj, err := validateSettings(planData.Pg.Settings.ValueString(), settingsSchema.JSON200.Settings.Pg)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Pg settings: %s", err))
					return
				}
				service.PgSettings = &obj
			}
			updated = true
		}

		if !planData.Pg.PgbouncerSettings.Equal(stateData.Pg.PgbouncerSettings) {
			if planData.Pg.PgbouncerSettings.ValueString() != "" {
				obj, err := validateSettings(planData.Pg.PgbouncerSettings.ValueString(), settingsSchema.JSON200.Settings.Pgbouncer)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Pgbouncer settings: %s", err))
					return
				}
				service.PgbouncerSettings = &obj
			}
			updated = true
		}

		if !planData.Pg.PglookoutSettings.Equal(stateData.Pg.PglookoutSettings) {
			if planData.Pg.PglookoutSettings.ValueString() != "" {
				obj, err := validateSettings(planData.Pg.PglookoutSettings.ValueString(), settingsSchema.JSON200.Settings.Pglookout)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Pglookout settings: %s", err))
					return
				}
				service.PglookoutSettings = &obj
			}
			updated = true
		}

		// Aiven would overwrite the backup schedule with random value if we don't specify it explicitly every time.
		if service.BackupSchedule == nil {
			bh, bm, err := parseBackupSchedule(planData.Pg.BackupSchedule.ValueString())
			if err != nil {
				diagnostics.AddError("Validation error", fmt.Sprintf("Unable to parse backup schedule, got error: %s", err))
				return
			}

			service.BackupSchedule = &struct {
				BackupHour   *int64 `json:"backup-hour,omitempty"`
				BackupMinute *int64 `json:"backup-minute,omitempty"`
			}{
				BackupHour:   &bh,
				BackupMinute: &bm,
			}
		}
	}

	if !updated {
		tflog.Info(ctx, "no updates detected", map[string]interface{}{})
	} else {
		res, err := r.client.UpdateDbaasServicePgWithResponse(
			ctx,
			oapi.DbaasServiceName(planData.Id.ValueString()),
			service,
		)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service pg, got error: %s", err))
			return
		}
		if res.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service pg, unexpected status: %s", res.Status()))
			return
		}
	}

	r.readPg(ctx, planData, diagnostics)
}
