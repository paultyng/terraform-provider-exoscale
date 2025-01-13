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

type ResourceKafkaModel struct {
	EnableCertAuth         types.Bool   `tfsdk:"enable_cert_auth"`
	EnableKafkaConnect     types.Bool   `tfsdk:"enable_kafka_connect"`
	EnableKafkaREST        types.Bool   `tfsdk:"enable_kafka_rest"`
	EnableSASLAuth         types.Bool   `tfsdk:"enable_sasl_auth"`
	EnableSchemaRegistry   types.Bool   `tfsdk:"enable_schema_registry"`
	IpFilter               types.Set    `tfsdk:"ip_filter"`
	Settings               types.String `tfsdk:"kafka_settings"`
	ConnectSettings        types.String `tfsdk:"kafka_connect_settings"`
	RestSettings           types.String `tfsdk:"kafka_rest_settings"`
	SchemaRegistrySettings types.String `tfsdk:"schema_registry_settings"`
	Version                types.String `tfsdk:"version"`
}

var ResourceKafkaSchema = schema.SingleNestedBlock{
	MarkdownDescription: "*kafka* database service type specific arguments. Structure is documented below.",
	Attributes: map[string]schema.Attribute{
		"enable_cert_auth": schema.BoolAttribute{
			MarkdownDescription: "Enable certificate-based authentication method.",
			Optional:            true,
			Computed:            true,
		},
		"enable_kafka_connect": schema.BoolAttribute{
			MarkdownDescription: "Enable Kafka Connect.",
			Optional:            true,
			Computed:            true,
		},
		"enable_kafka_rest": schema.BoolAttribute{
			MarkdownDescription: "Enable Kafka REST.",
			Optional:            true,
			Computed:            true,
		},
		"enable_sasl_auth": schema.BoolAttribute{
			MarkdownDescription: "Enable SASL-based authentication method.",
			Optional:            true,
			Computed:            true,
		},
		"enable_schema_registry": schema.BoolAttribute{
			MarkdownDescription: "Enable Schema Registry.",
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
		"kafka_settings": schema.StringAttribute{
			MarkdownDescription: "Kafka configuration settings in JSON format (`exo dbaas type show kafka --settings=kafka` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"kafka_connect_settings": schema.StringAttribute{
			MarkdownDescription: "Kafka Connect configuration settings in JSON format (`exo dbaas type show kafka --settings=kafka-connect` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"kafka_rest_settings": schema.StringAttribute{
			MarkdownDescription: "Kafka REST configuration settings in JSON format (`exo dbaas type show kafka --settings=kafka-rest` for reference).",
			Optional:            true,
			Computed:            true,
		},
		"schema_registry_settings": schema.StringAttribute{
			MarkdownDescription: "Schema Registry configuration settings in JSON format (`exo dbaas type show kafka --settings=schema-registry` for reference)",
			Optional:            true,
			Computed:            true,
		},
		"version": schema.StringAttribute{
			MarkdownDescription: "Kafka major version (`exo dbaas type show kafka` for reference; may only be set at creation time).",
			Optional:            true,
			Computed:            true,
		},
	},
}

// createKafka function handles Kafka specific part of database resource creation logic.
func (r *ServiceResource) createKafka(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	service := oapi.CreateDbaasServiceKafkaJSONRequestBody{
		Plan:                  data.Plan.ValueString(),
		TerminationProtection: data.TerminationProtection.ValueBoolPointer(),
		KafkaConnectEnabled:   data.Kafka.EnableKafkaConnect.ValueBoolPointer(),
		KafkaRestEnabled:      data.Kafka.EnableKafkaREST.ValueBoolPointer(),
		SchemaRegistryEnabled: data.Kafka.EnableSchemaRegistry.ValueBoolPointer(),
	}

	if !data.MaintenanceDOW.IsUnknown() && !data.MaintenanceTime.IsUnknown() {
		service.Maintenance = &struct {
			Dow  oapi.CreateDbaasServiceKafkaJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.CreateDbaasServiceKafkaJSONBodyMaintenanceDow(data.MaintenanceDOW.ValueString()),
			Time: data.MaintenanceTime.ValueString(),
		}
	}

	if !data.Kafka.Version.IsUnknown() {
		service.Version = data.Kafka.Version.ValueStringPointer()
	}

	if !data.Kafka.EnableCertAuth.IsUnknown() || !data.Kafka.EnableSASLAuth.IsUnknown() {
		service.AuthenticationMethods = &struct {
			Certificate *bool `json:"certificate,omitempty"`
			Sasl        *bool `json:"sasl,omitempty"`
		}{
			Certificate: data.Kafka.EnableCertAuth.ValueBoolPointer(),
			Sasl:        data.Kafka.EnableSASLAuth.ValueBoolPointer(),
		}
	}

	if !data.Kafka.IpFilter.IsUnknown() {
		obj := []string{}
		if len(data.Kafka.IpFilter.Elements()) > 0 {
			dg := data.Kafka.IpFilter.ElementsAs(ctx, &obj, false)
			if dg.HasError() {
				diagnostics.Append(dg...)
				return
			}
		}

		service.IpFilter = &obj
	}

	settingsSchema, err := r.client.GetDbaasSettingsKafkaWithResponse(ctx)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
		return
	}
	if settingsSchema.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
		return
	}

	if !data.Kafka.Settings.IsUnknown() {
		obj, err := validateSettings(data.Kafka.Settings.ValueString(), settingsSchema.JSON200.Settings.Kafka)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		service.KafkaSettings = &obj
	}

	if !data.Kafka.ConnectSettings.IsUnknown() {
		obj, err := validateSettings(data.Kafka.ConnectSettings.ValueString(), settingsSchema.JSON200.Settings.KafkaConnect)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka Connect settings: %s", err))
			return
		}
		service.KafkaConnectSettings = &obj
	}

	if !data.Kafka.RestSettings.IsUnknown() {
		obj, err := validateSettings(data.Kafka.RestSettings.ValueString(), settingsSchema.JSON200.Settings.KafkaRest)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka REST settings: %s", err))
			return
		}
		service.KafkaRestSettings = &obj
	}

	if !data.Kafka.SchemaRegistrySettings.IsUnknown() {
		obj, err := validateSettings(data.Kafka.SchemaRegistrySettings.ValueString(), settingsSchema.JSON200.Settings.SchemaRegistry)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Schema Registry settings: %s", err))
			return
		}
		service.SchemaRegistrySettings = &obj
	}

	res, err := r.client.CreateDbaasServiceKafkaWithResponse(
		ctx,
		oapi.DbaasServiceName(data.Name.ValueString()),
		service,
	)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service kafka, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service kafka, unexpected status: %s", res.Status()))
		return
	}

	r.readKafka(ctx, data, diagnostics)
}

// readKafka function handles Kafka specific part of database resource Read logic.
// It is used in the dedicated Read action but also as a finishing step of Create, Update and Import.
func (r *ServiceResource) readKafka(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	res, err := r.client.GetDbaasServiceKafkaWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service kafka, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service kafka, unexpected status: %s", res.Status()))
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

	if data.Plan.IsNull() || data.Plan.IsUnknown() {
		data.Plan = types.StringValue(apiService.Plan)
	}

	data.MaintenanceDOW = types.StringNull()
	data.MaintenanceTime = types.StringNull()
	if apiService.Maintenance != nil {
		data.MaintenanceDOW = types.StringValue(string(apiService.Maintenance.Dow))
		data.MaintenanceTime = types.StringValue(apiService.Maintenance.Time)
	}

	// Database block is required but it may be nil during import.
	if data.Kafka == nil {
		data.Kafka = &ResourceKafkaModel{}
	}

	data.Kafka.EnableKafkaConnect = types.BoolPointerValue(apiService.KafkaConnectEnabled)
	data.Kafka.EnableKafkaREST = types.BoolPointerValue(apiService.KafkaRestEnabled)
	data.Kafka.EnableSchemaRegistry = types.BoolPointerValue(apiService.SchemaRegistryEnabled)

	if apiService.AuthenticationMethods != nil {
		data.Kafka.EnableSASLAuth = types.BoolPointerValue(apiService.AuthenticationMethods.Sasl)
		data.Kafka.EnableCertAuth = types.BoolPointerValue(apiService.AuthenticationMethods.Certificate)
	}

	data.Kafka.IpFilter = types.SetNull(types.StringType)
	if apiService.IpFilter != nil {
		v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
		if dg.HasError() {
			diagnostics.Append(dg...)
			return
		}

		data.Kafka.IpFilter = v
	}

	data.Kafka.Version = types.StringNull()
	if apiService.Version != nil {
		version := strings.SplitN(*apiService.Version, ".", 3)
		data.Kafka.Version = types.StringValue(version[0] + "." + version[1])
	}

	data.Kafka.Settings = types.StringNull()
	if apiService.KafkaSettings != nil {
		settings, err := json.Marshal(*apiService.KafkaSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Kafka.Settings = types.StringValue(string(settings))
	}

	data.Kafka.ConnectSettings = types.StringNull()
	if apiService.KafkaConnectSettings != nil {
		settings, err := json.Marshal(*apiService.KafkaConnectSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka Connect settings: %s", err))
			return
		}
		data.Kafka.ConnectSettings = types.StringValue(string(settings))
	}

	data.Kafka.RestSettings = types.StringNull()
	if apiService.KafkaRestSettings != nil {
		settings, err := json.Marshal(*apiService.KafkaRestSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka REST settings: %s", err))
			return
		}
		data.Kafka.RestSettings = types.StringValue(string(settings))
	}

	data.Kafka.SchemaRegistrySettings = types.StringNull()
	if apiService.SchemaRegistrySettings != nil {
		settings, err := json.Marshal(*apiService.SchemaRegistrySettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid Schema Registry settings: %s", err))
			return
		}
		data.Kafka.SchemaRegistrySettings = types.StringValue(string(settings))
	}
}

// updateKafka function handles Kafka specific part of database resource Update logic.
func (r *ServiceResource) updateKafka(ctx context.Context, stateData *ServiceResourceModel, planData *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	var updated bool

	service := oapi.UpdateDbaasServiceKafkaJSONRequestBody{}

	if (!planData.MaintenanceDOW.Equal(stateData.MaintenanceDOW) && !planData.MaintenanceDOW.IsUnknown()) ||
		(!planData.MaintenanceTime.Equal(stateData.MaintenanceTime) && !planData.MaintenanceTime.IsUnknown()) {
		service.Maintenance = &struct {
			Dow  oapi.UpdateDbaasServiceKafkaJSONBodyMaintenanceDow `json:"dow"`
			Time string                                             `json:"time"`
		}{
			Dow:  oapi.UpdateDbaasServiceKafkaJSONBodyMaintenanceDow(planData.MaintenanceDOW.ValueString()),
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

	if planData.Kafka != nil {
		if stateData.Kafka == nil {
			stateData.Kafka = &ResourceKafkaModel{}
		}

		if !planData.Kafka.IpFilter.Equal(stateData.Kafka.IpFilter) {
			obj := []string{}
			if len(planData.Kafka.IpFilter.Elements()) > 0 {
				dg := planData.Kafka.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}
			service.IpFilter = &obj
			updated = true
		}

		if !planData.Kafka.EnableKafkaConnect.Equal(stateData.Kafka.EnableKafkaConnect) {
			service.KafkaConnectEnabled = planData.Kafka.EnableKafkaConnect.ValueBoolPointer()
			updated = true
		}

		if !planData.Kafka.EnableKafkaREST.Equal(stateData.Kafka.EnableKafkaREST) {
			service.KafkaRestEnabled = planData.Kafka.EnableKafkaREST.ValueBoolPointer()
			updated = true
		}

		if !planData.Kafka.EnableSchemaRegistry.Equal(stateData.Kafka.EnableSchemaRegistry) {
			service.SchemaRegistryEnabled = planData.Kafka.EnableSchemaRegistry.ValueBoolPointer()
			updated = true
		}

		if !planData.Kafka.EnableCertAuth.Equal(stateData.Kafka.EnableCertAuth) || !planData.Kafka.EnableSASLAuth.Equal(stateData.Kafka.EnableSASLAuth) {
			service.AuthenticationMethods = &struct {
				Certificate *bool `json:"certificate,omitempty"`
				Sasl        *bool `json:"sasl,omitempty"`
			}{
				Certificate: planData.Kafka.EnableCertAuth.ValueBoolPointer(),
				Sasl:        planData.Kafka.EnableSASLAuth.ValueBoolPointer(),
			}
			updated = true
		}

		settingsSchema, err := r.client.GetDbaasSettingsKafkaWithResponse(ctx)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
			return
		}
		if settingsSchema.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
			return
		}

		if !planData.Kafka.Settings.Equal(stateData.Kafka.Settings) {
			if planData.Kafka.Settings.ValueString() != "" {
				obj, err := validateSettings(planData.Kafka.Settings.ValueString(), settingsSchema.JSON200.Settings.Kafka)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka settings: %s", err))
					return
				}
				service.KafkaSettings = &obj
			}
			updated = true
		}

		if !planData.Kafka.ConnectSettings.Equal(stateData.Kafka.ConnectSettings) {
			if planData.Kafka.ConnectSettings.ValueString() != "" {
				obj, err := validateSettings(planData.Kafka.ConnectSettings.ValueString(), settingsSchema.JSON200.Settings.KafkaConnect)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka Connect settings: %s", err))
					return
				}
				service.KafkaConnectSettings = &obj
			}
			updated = true
		}

		if !planData.Kafka.RestSettings.Equal(stateData.Kafka.RestSettings) {
			if planData.Kafka.RestSettings.ValueString() != "" {
				obj, err := validateSettings(planData.Kafka.RestSettings.ValueString(), settingsSchema.JSON200.Settings.KafkaRest)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka settings: %s", err))
					return
				}
				service.KafkaRestSettings = &obj
			}
			updated = true
		}

		if !planData.Kafka.SchemaRegistrySettings.Equal(stateData.Kafka.SchemaRegistrySettings) {
			if planData.Kafka.SchemaRegistrySettings.ValueString() != "" {
				obj, err := validateSettings(planData.Kafka.SchemaRegistrySettings.ValueString(), settingsSchema.JSON200.Settings.SchemaRegistry)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Kafka settings: %s", err))
					return
				}
				service.SchemaRegistrySettings = &obj
			}
			updated = true
		}
	}

	if !updated {
		tflog.Info(ctx, "no updates detected", map[string]interface{}{})
	} else {
		res, err := r.client.UpdateDbaasServiceKafkaWithResponse(
			ctx,
			oapi.DbaasServiceName(planData.Id.ValueString()),
			service,
		)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service kafka, got error: %s", err))
			return
		}
		if res.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service kafka, unexpected status: %s", res.Status()))
			return
		}
	}

	r.readKafka(ctx, planData, diagnostics)
}
