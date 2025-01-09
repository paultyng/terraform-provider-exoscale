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

type ResourceMysqlModel struct {
	AdminPassword  types.String `tfsdk:"admin_password"`
	AdminUsername  types.String `tfsdk:"admin_username"`
	BackupSchedule types.String `tfsdk:"backup_schedule"`
	IpFilter       types.Set    `tfsdk:"ip_filter"`
	Settings       types.String `tfsdk:"mysql_settings"`
	Version        types.String `tfsdk:"version"`
}

var ResourceMysqlSchema = schema.SingleNestedBlock{
	MarkdownDescription: "*mysql* database service type specific arguments. Structure is documented below.",
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
		"mysql_settings": schema.StringAttribute{
			MarkdownDescription: "MySQL configuration settings in JSON format (`exo dbaas type show mysql --settings=mysql` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"version": schema.StringAttribute{
			MarkdownDescription: "MySQL major version (`exo dbaas type show mysql` for reference; may only be set at creation time).",
			Optional:            true,
			Computed:            true,
		},
	},
}

// createMysql function handles MySQL specific part of database resource creation logic.
func (r *ServiceResource) createMysql(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	mysqlData := &ResourceMysqlModel{}
	if data.Mysql != nil {
		mysqlData = data.Mysql
	}

	service := oapi.CreateDbaasServiceMysqlJSONRequestBody{
		Plan:                  data.Plan.ValueString(),
		TerminationProtection: data.TerminationProtection.ValueBoolPointer(),
	}

	if !mysqlData.Version.IsUnknown() {
		service.Version = mysqlData.Version.ValueStringPointer()
	}

	if !mysqlData.AdminPassword.IsNull() {
		service.AdminPassword = mysqlData.AdminPassword.ValueStringPointer()
	}

	if !mysqlData.AdminUsername.IsNull() {
		service.AdminUsername = mysqlData.AdminUsername.ValueStringPointer()
	}

	if !mysqlData.IpFilter.IsUnknown() {
		obj := []string{}
		if len(mysqlData.IpFilter.Elements()) > 0 {
			dg := mysqlData.IpFilter.ElementsAs(ctx, &obj, false)
			if dg.HasError() {
				diagnostics.Append(dg...)
				return
			}
		}

		service.IpFilter = &obj
	}

	if !data.MaintenanceDOW.IsUnknown() && !data.MaintenanceTime.IsUnknown() {
		service.Maintenance = &struct {
			Dow  oapi.CreateDbaasServiceMysqlJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.CreateDbaasServiceMysqlJSONBodyMaintenanceDow(data.MaintenanceDOW.ValueString()),
			Time: data.MaintenanceTime.ValueString(),
		}
	}

	if !mysqlData.BackupSchedule.IsUnknown() {
		bh, bm, err := parseBackupSchedule(mysqlData.BackupSchedule.ValueString())
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

	settingsSchema, err := r.client.GetDbaasSettingsMysqlWithResponse(ctx)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
		return
	}
	if settingsSchema.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
		return
	}

	if !mysqlData.Settings.IsUnknown() {
		obj, err := validateSettings(mysqlData.Settings.ValueString(), settingsSchema.JSON200.Settings.Mysql)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		service.MysqlSettings = &obj
	}

	res, err := r.client.CreateDbaasServiceMysqlWithResponse(
		ctx,
		oapi.DbaasServiceName(data.Name.ValueString()),
		service,
	)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service mysql, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database settings schema, unexpected status: %s", res.Status()))
		return
	}

	r.readMysql(ctx, data, diagnostics)
}

// readMysql function handles MySQL specific part of database resource Read logic.
// It is used in the dedicated Read action but also as a finishing step of Create, Update and Import.
func (r *ServiceResource) readMysql(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	res, err := r.client.GetDbaasServiceMysqlWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service mysql, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service mysql, unexpected status: %s", res.Status()))
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

	if data.Mysql == nil {
		data.Mysql = &ResourceMysqlModel{}
	}

	data.Mysql.BackupSchedule = types.StringNull()
	if apiService.BackupSchedule != nil {
		backupHour := types.Int64PointerValue(apiService.BackupSchedule.BackupHour)
		backupMinute := types.Int64PointerValue(apiService.BackupSchedule.BackupMinute)
		data.Mysql.BackupSchedule = types.StringValue(fmt.Sprintf(
			"%02d:%02d",
			backupHour.ValueInt64(),
			backupMinute.ValueInt64(),
		))
	}

	data.Mysql.IpFilter = types.SetNull(types.StringType)
	if apiService.IpFilter != nil {
		v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
		if dg.HasError() {
			diagnostics.Append(dg...)
			return
		}

		data.Mysql.IpFilter = v
	}

	data.Mysql.Version = types.StringNull()
	if apiService.Version != nil {
		data.Mysql.Version = types.StringValue(strings.SplitN(*apiService.Version, ".", 2)[0])
	}

	data.Mysql.Settings = types.StringNull()
	if apiService.MysqlSettings != nil {
		settings, err := json.Marshal(*apiService.MysqlSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Mysql.Settings = types.StringValue(string(settings))
	}
}

// updateMysql function handles MySQL specific part of database resource Update logic.
func (r *ServiceResource) updateMysql(ctx context.Context, stateData *ServiceResourceModel, planData *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	var updated bool

	service := oapi.UpdateDbaasServiceMysqlJSONRequestBody{}

	if (!planData.MaintenanceDOW.Equal(stateData.MaintenanceDOW) && !planData.MaintenanceDOW.IsUnknown()) ||
		(!planData.MaintenanceTime.Equal(stateData.MaintenanceTime) && !planData.MaintenanceTime.IsUnknown()) {
		service.Maintenance = &struct {
			Dow  oapi.UpdateDbaasServiceMysqlJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.UpdateDbaasServiceMysqlJSONBodyMaintenanceDow(planData.MaintenanceDOW.ValueString()),
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

	if planData.Mysql != nil {
		if stateData.Mysql == nil {
			stateData.Mysql = &ResourceMysqlModel{}
		}

		if !planData.Mysql.BackupSchedule.Equal(stateData.Mysql.BackupSchedule) {
			bh, bm, err := parseBackupSchedule(planData.Mysql.BackupSchedule.ValueString())
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

		if !planData.Mysql.IpFilter.Equal(stateData.Mysql.IpFilter) {
			obj := []string{}
			if len(planData.Mysql.IpFilter.Elements()) > 0 {
				dg := planData.Mysql.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}
			service.IpFilter = &obj
			updated = true
		}

		if !planData.Mysql.Settings.Equal(stateData.Mysql.Settings) {
			settingsSchema, err := r.client.GetDbaasSettingsMysqlWithResponse(ctx)
			if err != nil {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
				return
			}
			if settingsSchema.StatusCode() != http.StatusOK {
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
				return
			}

			if planData.Mysql.Settings.ValueString() != "" {
				obj, err := validateSettings(planData.Mysql.Settings.ValueString(), settingsSchema.JSON200.Settings.Mysql)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Mysql settings: %s", err))
					return
				}
				service.MysqlSettings = &obj
			}
			updated = true
		}

		// Aiven would overwrite the backup schedule with random value if we don't specify it explicitly every time.
		if service.BackupSchedule == nil {
			bh, bm, err := parseBackupSchedule(planData.Mysql.BackupSchedule.ValueString())
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
		res, err := r.client.UpdateDbaasServiceMysqlWithResponse(
			ctx,
			oapi.DbaasServiceName(planData.Id.ValueString()),
			service,
		)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service mysql, got error: %s", err))
			return
		}
		if res.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service mysql, unexpected status: %s", res.Status()))
			return
		}
	}

	r.readMysql(ctx, planData, diagnostics)
}
