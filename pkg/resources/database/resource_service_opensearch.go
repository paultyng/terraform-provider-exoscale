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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/exoscale/egoscale/v2/oapi"

	"github.com/exoscale/terraform-provider-exoscale/pkg/validators"
)

type ResourceOpensearchModel struct {
	ForkFromService          types.String `tfsdk:"fork_from_service"`
	RecoveryBackupName       types.String `tfsdk:"recovery_backup_name"`
	IpFilter                 types.Set    `tfsdk:"ip_filter"`
	KeepIndexRefreshInterval types.Bool   `tfsdk:"keep_index_refresh_interval"`
	MaxIndexCount            types.Int64  `tfsdk:"max_index_count"`
	Settings                 types.String `tfsdk:"settings"`
	Version                  types.String `tfsdk:"version"`

	IndexPatterns []ResourceOpensearchIndexPatternsModel `tfsdk:"index_pattern"`
	IndexTemplate *ResourceOpensearchIndexTemplateModel  `tfsdk:"index_template"`
	Dashboards    *ResourceOpensearchDashboardsModel     `tfsdk:"dashboards"`
}

type ResourceOpensearchIndexPatternsModel struct {
	MaxIndexCount    types.Int64  `tfsdk:"max_index_count"`
	Pattern          types.String `tfsdk:"pattern"`
	SortingAlgorithm types.String `tfsdk:"sorting_algorithm"`
}

type ResourceOpensearchIndexTemplateModel struct {
	MappingNestedObjectsLimit types.Int64 `tfsdk:"mapping_nested_objects_limit"`
	NumberOfReplicas          types.Int64 `tfsdk:"number_of_replicas"`
	NumberOfShards            types.Int64 `tfsdk:"number_of_shards"`
}

type ResourceOpensearchDashboardsModel struct {
	Enabled         types.Bool  `tfsdk:"enabled"`
	MaxOldSpaceSize types.Int64 `tfsdk:"max_old_space_size"`
	RequestTimeout  types.Int64 `tfsdk:"request_timeout"`
}

var ResourceOpensearchSchema = schema.SingleNestedBlock{
	MarkdownDescription: "*opensearch* database service type specific arguments. Structure is documented below.",
	Attributes: map[string]schema.Attribute{
		"fork_from_service": schema.StringAttribute{
			MarkdownDescription: "❗ Service name",
			Optional:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"ip_filter": schema.SetAttribute{
			ElementType:         types.StringType,
			MarkdownDescription: "Allow incoming connections from this list of CIDR address block, e.g. `[\"10.20.0.0/16\"]",
			Optional:            true,
			Computed:            true,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(validators.IsCIDRNetworkValidator{Min: 0, Max: 128}),
			},
		},
		"keep_index_refresh_interval": schema.BoolAttribute{
			MarkdownDescription: "Aiven automation resets index.refresh_interval to default value for every index to be sure that indices are always visible to search. If it doesn't fit your case, you can disable this by setting up this flag to true.",
			Optional:            true,
		},
		"max_index_count": schema.Int64Attribute{
			MarkdownDescription: "Maximum number of indexes to keep (Minimum value is `0`)",
			Optional:            true,
		},
		"settings": schema.StringAttribute{
			MarkdownDescription: "OpenSearch-specific settings, in json. e.g.`jsonencode({thread_pool_search_size: 64})`. Use `exo x get-dbaas-settings-opensearch` to get a list of available settings.",
			Optional:            true,
			Computed:            true,
		},
		"recovery_backup_name": schema.StringAttribute{
			MarkdownDescription: "❗ Name of a backup to recover from",
			Optional:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"version": schema.StringAttribute{
			MarkdownDescription: "❗ OpenSearch major version (`exo dbaas type show opensearch` for reference)",
			Optional:            true,
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplaceIfConfigured(),
			},
		},
	},
	Blocks: map[string]schema.Block{
		"index_pattern": schema.ListNestedBlock{
			MarkdownDescription: "(can be used multiple times) Allows you to create glob style patterns and set a max number of indexes matching this pattern you want to keep. Creating indexes exceeding this value will cause the oldest one to get deleted. You could for example create a pattern looking like 'logs.?' and then create index logs.1, logs.2 etc, it will delete logs.1 once you create logs.6. Do note 'logs.?' does not apply to logs.10. Note: Setting max_index_count to 0 will do nothing and the pattern gets ignored.",
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"max_index_count": schema.Int64Attribute{
						MarkdownDescription: "Maximum number of indexes to keep before deleting the oldest one (Minimum value is `0`)",
						Optional:            true,
					},
					"pattern": schema.StringAttribute{
						MarkdownDescription: "fnmatch pattern",
						Optional:            true,
					},
					"sorting_algorithm": schema.StringAttribute{
						MarkdownDescription: "`alphabetical` or `creation_date`.",
						Optional:            true,
					},
				},
			},
		},
		"index_template": schema.SingleNestedBlock{
			MarkdownDescription: "Template settings for all new indexes",
			Attributes: map[string]schema.Attribute{
				"mapping_nested_objects_limit": schema.Int64Attribute{
					MarkdownDescription: "The maximum number of nested JSON objects that a single document can contain across all nested types. This limit helps to prevent out of memory errors when a document contains too many nested objects. (Default is 10000. Minimum value is `0`, maximum value is `100000`.)",
					Optional:            true,
				},
				"number_of_replicas": schema.Int64Attribute{
					MarkdownDescription: "The number of replicas each primary shard has. (Minimum value is `0`, maximum value is `29`)",
					Optional:            true,
				},
				"number_of_shards": schema.Int64Attribute{
					MarkdownDescription: "The number of primary shards that an index should have. (Minimum value is `1`, maximum value is `1024`.)",
					Optional:            true,
				},
			},
		},
		"dashboards": schema.SingleNestedBlock{
			MarkdownDescription: "OpenSearch Dashboards settings",
			Attributes: map[string]schema.Attribute{
				"enabled": schema.BoolAttribute{
					MarkdownDescription: "Enable or disable OpenSearch Dashboards (default: true).",
					Optional:            true,
				},
				"max_old_space_size": schema.Int64Attribute{
					MarkdownDescription: "Limits the maximum amount of memory (in MiB) the OpenSearch Dashboards process can use. This sets the max_old_space_size option of the nodejs running the OpenSearch Dashboards. Note: the memory reserved by OpenSearch Dashboards is not available for OpenSearch. (default: 128).",
					Optional:            true,
				},
				"request_timeout": schema.Int64Attribute{
					MarkdownDescription: "Timeout in milliseconds for requests made by OpenSearch Dashboards towards OpenSearch (default: 30000)",
					Optional:            true,
				},
			},
		},
	},
}

// createOpensearch function handles OpenSearch specific part of database resource creation logic.
func (r *ServiceResource) createOpensearch(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	if data.Opensearch == nil {
		data.Opensearch = &ResourceOpensearchModel{}
	}

	service := oapi.CreateDbaasServiceOpensearchJSONRequestBody{
		Plan:                     data.Plan.ValueString(),
		TerminationProtection:    data.TerminationProtection.ValueBoolPointer(),
		MaxIndexCount:            data.Opensearch.MaxIndexCount.ValueInt64Pointer(),
		ForkFromService:          (*oapi.DbaasServiceName)(data.Opensearch.ForkFromService.ValueStringPointer()),
		RecoveryBackupName:       data.Opensearch.RecoveryBackupName.ValueStringPointer(),
		KeepIndexRefreshInterval: data.Opensearch.KeepIndexRefreshInterval.ValueBoolPointer(),
	}

	if !data.MaintenanceDOW.IsUnknown() && !data.MaintenanceTime.IsUnknown() {
		service.Maintenance = &struct {
			Dow  oapi.CreateDbaasServiceOpensearchJSONBodyMaintenanceDow `json:"dow"`
			Time string                                                  `json:"time"`
		}{
			Dow:  oapi.CreateDbaasServiceOpensearchJSONBodyMaintenanceDow(data.MaintenanceDOW.ValueString()),
			Time: data.MaintenanceTime.ValueString(),
		}
	}

	if !data.Opensearch.Version.IsUnknown() {
		service.Version = data.Opensearch.Version.ValueStringPointer()
	}

	if !data.Opensearch.IpFilter.IsUnknown() {
		obj := []string{}
		if len(data.Opensearch.IpFilter.Elements()) > 0 {
			dg := data.Opensearch.IpFilter.ElementsAs(ctx, &obj, false)
			if dg.HasError() {
				diagnostics.Append(dg...)
				return
			}
		}

		service.IpFilter = &obj
	}

	if len(data.Opensearch.IndexPatterns) > 0 {
		patterns := []struct {
			MaxIndexCount    *int64                                                                  `json:"max-index-count,omitempty"`
			Pattern          *string                                                                 `json:"pattern,omitempty"`
			SortingAlgorithm *oapi.CreateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm `json:"sorting-algorithm,omitempty"`
		}{}
		for _, pattern := range data.Opensearch.IndexPatterns {
			obj := struct {
				MaxIndexCount    *int64                                                                  `json:"max-index-count,omitempty"`
				Pattern          *string                                                                 `json:"pattern,omitempty"`
				SortingAlgorithm *oapi.CreateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm `json:"sorting-algorithm,omitempty"`
			}{}

			if !pattern.MaxIndexCount.IsNull() {
				obj.MaxIndexCount = pattern.MaxIndexCount.ValueInt64Pointer()
			}
			if !pattern.Pattern.IsNull() {
				obj.Pattern = pattern.Pattern.ValueStringPointer()
			}
			if !pattern.SortingAlgorithm.IsNull() {
				obj.SortingAlgorithm = (*oapi.CreateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm)(pattern.SortingAlgorithm.ValueStringPointer())
			}

			patterns = append(patterns, obj)
		}

		service.IndexPatterns = &patterns
	}

	if data.Opensearch.IndexTemplate != nil {
		service.IndexTemplate = &struct {
			MappingNestedObjectsLimit *int64 "json:\"mapping-nested-objects-limit,omitempty\""
			NumberOfReplicas          *int64 "json:\"number-of-replicas,omitempty\""
			NumberOfShards            *int64 "json:\"number-of-shards,omitempty\""
		}{}

		if !data.Opensearch.IndexTemplate.MappingNestedObjectsLimit.IsNull() {
			service.IndexTemplate.MappingNestedObjectsLimit = data.Opensearch.IndexTemplate.MappingNestedObjectsLimit.ValueInt64Pointer()
		}
		if !data.Opensearch.IndexTemplate.NumberOfReplicas.IsNull() {
			service.IndexTemplate.NumberOfReplicas = data.Opensearch.IndexTemplate.NumberOfReplicas.ValueInt64Pointer()
		}
		if !data.Opensearch.IndexTemplate.NumberOfShards.IsNull() {
			service.IndexTemplate.NumberOfShards = data.Opensearch.IndexTemplate.NumberOfShards.ValueInt64Pointer()
		}
	}

	if data.Opensearch.Dashboards != nil {
		service.OpensearchDashboards = &struct {
			Enabled                  *bool  "json:\"enabled,omitempty\""
			MaxOldSpaceSize          *int64 "json:\"max-old-space-size,omitempty\""
			OpensearchRequestTimeout *int64 "json:\"opensearch-request-timeout,omitempty\""
		}{}
		if !data.Opensearch.Dashboards.Enabled.IsNull() {
			service.OpensearchDashboards.Enabled = data.Opensearch.Dashboards.Enabled.ValueBoolPointer()
		}
		if !data.Opensearch.Dashboards.MaxOldSpaceSize.IsNull() {
			service.OpensearchDashboards.MaxOldSpaceSize = data.Opensearch.Dashboards.MaxOldSpaceSize.ValueInt64Pointer()
		}
		if !data.Opensearch.Dashboards.RequestTimeout.IsNull() {
			service.OpensearchDashboards.OpensearchRequestTimeout = data.Opensearch.Dashboards.RequestTimeout.ValueInt64Pointer()
		}
	}

	settingsSchema, err := r.client.GetDbaasSettingsOpensearchWithResponse(ctx)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
		return
	}
	if settingsSchema.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
		return
	}

	if !data.Opensearch.Settings.IsUnknown() {
		obj, err := validateSettings(data.Opensearch.Settings.ValueString(), settingsSchema.JSON200.Settings.Opensearch)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		service.OpensearchSettings = &obj
	}

	res, err := r.client.CreateDbaasServiceOpensearchWithResponse(
		ctx,
		oapi.DbaasServiceName(data.Name.ValueString()),
		service,
	)
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service opensearch, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create database service opensearch, unexpected status: %s", res.Status()))
		return
	}

	r.readOpensearch(ctx, data, diagnostics)
}

// readOpensearch function handles OpenSearch specific part of database resource Read logic.
// It is used in the dedicated Read action but also as a finishing step of Create, Update and Import.
// NOTE: For optional but not computed attributes we only read remote value if they are defined in the plan.
func (r *ServiceResource) readOpensearch(ctx context.Context, data *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	caCert, err := r.client.GetDatabaseCACertificate(ctx, data.Zone.ValueString())
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get CA Certificate: %s", err))
		return
	}
	data.CA = types.StringValue(caCert)

	res, err := r.client.GetDbaasServiceOpensearchWithResponse(ctx, oapi.DbaasServiceName(data.Id.ValueString()))
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service opensearch, got error: %s", err))
		return
	}
	if res.StatusCode() != http.StatusOK {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database service opensearch, unexpected status: %s", res.Status()))
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
	var isImport bool
	if data.Opensearch == nil {
		isImport = true
		data.Opensearch = &ResourceOpensearchModel{
			IndexPatterns: []ResourceOpensearchIndexPatternsModel{},
			IndexTemplate: &ResourceOpensearchIndexTemplateModel{},
			Dashboards:    &ResourceOpensearchDashboardsModel{},
		}
	}

	data.Opensearch.IpFilter = types.SetNull(types.StringType)
	if apiService.IpFilter != nil {
		v, dg := types.SetValueFrom(ctx, types.StringType, *apiService.IpFilter)
		if dg.HasError() {
			diagnostics.Append(dg...)
			return
		}

		data.Opensearch.IpFilter = v
	}

	if apiService.IndexPatterns != nil && len(*apiService.IndexPatterns) > 0 {
		data.Opensearch.IndexPatterns = []ResourceOpensearchIndexPatternsModel{}
		for _, pattern := range *apiService.IndexPatterns {
			model := ResourceOpensearchIndexPatternsModel{
				MaxIndexCount:    types.Int64PointerValue(pattern.MaxIndexCount),
				Pattern:          types.StringPointerValue(pattern.Pattern),
				SortingAlgorithm: types.StringPointerValue((*string)(pattern.SortingAlgorithm)),
			}
			data.Opensearch.IndexPatterns = append(
				data.Opensearch.IndexPatterns,
				model,
			)
		}
	}

	if data.Opensearch.IndexTemplate != nil && apiService.IndexTemplate != nil {
		if !data.Opensearch.IndexTemplate.MappingNestedObjectsLimit.IsNull() || isImport {
			data.Opensearch.IndexTemplate.MappingNestedObjectsLimit = types.Int64PointerValue(apiService.IndexTemplate.MappingNestedObjectsLimit)
		}
		if !data.Opensearch.IndexTemplate.NumberOfReplicas.IsNull() || isImport {
			data.Opensearch.IndexTemplate.NumberOfReplicas = types.Int64PointerValue(apiService.IndexTemplate.NumberOfReplicas)
		}
		if !data.Opensearch.IndexTemplate.NumberOfShards.IsNull() || isImport {
			data.Opensearch.IndexTemplate.NumberOfShards = types.Int64PointerValue(apiService.IndexTemplate.NumberOfShards)
		}
	}

	if data.Opensearch.Dashboards != nil && apiService.OpensearchDashboards != nil {
		if !data.Opensearch.Dashboards.Enabled.IsNull() || isImport {
			data.Opensearch.Dashboards.Enabled = types.BoolPointerValue(apiService.OpensearchDashboards.Enabled)
		}
		if !data.Opensearch.Dashboards.MaxOldSpaceSize.IsNull() || isImport {
			data.Opensearch.Dashboards.MaxOldSpaceSize = types.Int64PointerValue(apiService.OpensearchDashboards.MaxOldSpaceSize)
		}
		if !data.Opensearch.Dashboards.RequestTimeout.IsNull() || isImport {
			data.Opensearch.Dashboards.RequestTimeout = types.Int64PointerValue(apiService.OpensearchDashboards.OpensearchRequestTimeout)

		}
	}

	if !data.Opensearch.KeepIndexRefreshInterval.IsNull() || isImport {
		data.Opensearch.KeepIndexRefreshInterval = types.BoolPointerValue(apiService.KeepIndexRefreshInterval)
	}

	if !data.Opensearch.MaxIndexCount.IsNull() || isImport {
		data.Opensearch.MaxIndexCount = types.Int64PointerValue(apiService.MaxIndexCount)
	}

	data.Opensearch.Version = types.StringNull()
	if apiService.Version != nil {
		data.Opensearch.Version = types.StringValue(strings.SplitN(*apiService.Version, ".", 2)[0])
	}

	data.Opensearch.Settings = types.StringNull()
	if apiService.OpensearchSettings != nil {
		settings, err := json.Marshal(*apiService.OpensearchSettings)
		if err != nil {
			diagnostics.AddError("Validation error", fmt.Sprintf("invalid settings: %s", err))
			return
		}
		data.Opensearch.Settings = types.StringValue(string(settings))
	}
}

// updateOpensearch function handles OpenSearch specific part of database resource Update logic.
func (r *ServiceResource) updateOpensearch(ctx context.Context, stateData *ServiceResourceModel, planData *ServiceResourceModel, diagnostics *diag.Diagnostics) {
	var updated bool

	service := oapi.UpdateDbaasServiceOpensearchJSONRequestBody{}

	if (!planData.MaintenanceDOW.Equal(stateData.MaintenanceDOW) && !planData.MaintenanceDOW.IsUnknown()) ||
		(!planData.MaintenanceTime.Equal(stateData.MaintenanceTime) && !planData.MaintenanceTime.IsUnknown()) {
		service.Maintenance = &struct {
			Dow  oapi.UpdateDbaasServiceOpensearchJSONBodyMaintenanceDow `json:"dow"`
			Time string                                                  `json:"time"`
		}{
			Dow:  oapi.UpdateDbaasServiceOpensearchJSONBodyMaintenanceDow(planData.MaintenanceDOW.ValueString()),
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

	if planData.Opensearch != nil {
		if stateData.Opensearch == nil {
			stateData.Opensearch = &ResourceOpensearchModel{}
		}

		if !planData.Opensearch.IpFilter.Equal(stateData.Opensearch.IpFilter) {
			obj := []string{}
			if len(planData.Opensearch.IpFilter.Elements()) > 0 {
				dg := planData.Opensearch.IpFilter.ElementsAs(ctx, &obj, false)
				if dg.HasError() {
					diagnostics.Append(dg...)
					return
				}
			}
			service.IpFilter = &obj
			updated = true
		}

		if len(planData.Opensearch.IndexPatterns) > 0 {
			patterns := []struct {
				MaxIndexCount    *int64                                                                  `json:"max-index-count,omitempty"`
				Pattern          *string                                                                 `json:"pattern,omitempty"`
				SortingAlgorithm *oapi.UpdateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm `json:"sorting-algorithm,omitempty"`
			}{}
			for _, pattern := range planData.Opensearch.IndexPatterns {
				patterns = append(patterns, struct {
					MaxIndexCount    *int64                                                                  `json:"max-index-count,omitempty"`
					Pattern          *string                                                                 `json:"pattern,omitempty"`
					SortingAlgorithm *oapi.UpdateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm `json:"sorting-algorithm,omitempty"`
				}{
					pattern.MaxIndexCount.ValueInt64Pointer(),
					pattern.Pattern.ValueStringPointer(),
					(*oapi.UpdateDbaasServiceOpensearchJSONBodyIndexPatternsSortingAlgorithm)(pattern.SortingAlgorithm.ValueStringPointer()),
				})
			}

			service.IndexPatterns = &patterns
			updated = true
		}

		if planData.Opensearch.IndexTemplate != nil {
			service.IndexTemplate = &struct {
				MappingNestedObjectsLimit *int64 "json:\"mapping-nested-objects-limit,omitempty\""
				NumberOfReplicas          *int64 "json:\"number-of-replicas,omitempty\""
				NumberOfShards            *int64 "json:\"number-of-shards,omitempty\""
			}{}
			if !planData.Opensearch.IndexTemplate.MappingNestedObjectsLimit.Equal(stateData.Opensearch.IndexTemplate.MappingNestedObjectsLimit) {
				service.IndexTemplate.MappingNestedObjectsLimit = planData.Opensearch.IndexTemplate.MappingNestedObjectsLimit.ValueInt64Pointer()
				updated = true
			}
			if !planData.Opensearch.IndexTemplate.NumberOfReplicas.Equal(stateData.Opensearch.IndexTemplate.NumberOfReplicas) {
				service.IndexTemplate.NumberOfReplicas = planData.Opensearch.IndexTemplate.NumberOfReplicas.ValueInt64Pointer()
				updated = true
			}
			if !planData.Opensearch.IndexTemplate.NumberOfShards.Equal(stateData.Opensearch.IndexTemplate.NumberOfShards) {
				service.IndexTemplate.NumberOfShards = planData.Opensearch.IndexTemplate.NumberOfShards.ValueInt64Pointer()
				updated = true
			}
		}

		if planData.Opensearch.Dashboards != nil {
			service.OpensearchDashboards = &struct {
				Enabled                  *bool  "json:\"enabled,omitempty\""
				MaxOldSpaceSize          *int64 "json:\"max-old-space-size,omitempty\""
				OpensearchRequestTimeout *int64 "json:\"opensearch-request-timeout,omitempty\""
			}{}
			if !planData.Opensearch.Dashboards.Enabled.Equal(stateData.Opensearch.Dashboards.Enabled) {
				service.OpensearchDashboards.Enabled = planData.Opensearch.Dashboards.Enabled.ValueBoolPointer()
			}
			if !planData.Opensearch.Dashboards.MaxOldSpaceSize.Equal(stateData.Opensearch.Dashboards.MaxOldSpaceSize) {
				service.OpensearchDashboards.MaxOldSpaceSize = planData.Opensearch.Dashboards.MaxOldSpaceSize.ValueInt64Pointer()
			}
			if !planData.Opensearch.Dashboards.RequestTimeout.Equal(stateData.Opensearch.Dashboards.RequestTimeout) {
				service.OpensearchDashboards.OpensearchRequestTimeout = planData.Opensearch.Dashboards.RequestTimeout.ValueInt64Pointer()
			}
		}

		if !planData.Opensearch.KeepIndexRefreshInterval.IsNull() && !planData.Opensearch.KeepIndexRefreshInterval.Equal(stateData.Opensearch.KeepIndexRefreshInterval) {
			service.KeepIndexRefreshInterval = planData.Opensearch.KeepIndexRefreshInterval.ValueBoolPointer()
			updated = true
		}

		if !planData.Opensearch.MaxIndexCount.IsNull() && !planData.Opensearch.MaxIndexCount.Equal(stateData.Opensearch.MaxIndexCount) {
			service.MaxIndexCount = planData.Opensearch.MaxIndexCount.ValueInt64Pointer()
			updated = true
		}

		if !planData.Opensearch.Settings.Equal(stateData.Opensearch.Settings) {
			if planData.Opensearch.Settings.ValueString() != "" {
				settingsSchema, err := r.client.GetDbaasSettingsOpensearchWithResponse(ctx)
				if err != nil {
					diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, got error: %s", err))
					return
				}
				if settingsSchema.StatusCode() != http.StatusOK {
					diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read database settings schema, unexpected status: %s", settingsSchema.Status()))
					return
				}

				obj, err := validateSettings(planData.Opensearch.Settings.ValueString(), settingsSchema.JSON200.Settings.Opensearch)
				if err != nil {
					diagnostics.AddError("Validation error", fmt.Sprintf("invalid Opensearch settings: %s", err))
					return
				}
				service.OpensearchSettings = &obj
			}
			updated = true
		}
	}

	if !updated {
		tflog.Info(ctx, "no updates detected", map[string]interface{}{})
	} else {
		res, err := r.client.UpdateDbaasServiceOpensearchWithResponse(
			ctx,
			oapi.DbaasServiceName(planData.Id.ValueString()),
			service,
		)
		if err != nil {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update database service opensearch, got error: %s", err))
			return
		}
		if res.StatusCode() != http.StatusOK {
			diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update database service opensearch, unexpected status: %s", res.Status()))
			return
		}
	}

	r.readOpensearch(ctx, planData, diagnostics)
}
