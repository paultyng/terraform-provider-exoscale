package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	exoscale "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/xeipuuv/gojsonschema"
)

// validateSettings validates user-provided JSON-formatted
// Database Service settings against a reference JSON Schema.
func validateSettings(in string, schema interface{}) (map[string]interface{}, error) {
	var userSettings map[string]interface{}

	if err := json.Unmarshal([]byte(in), &userSettings); err != nil {
		return nil, fmt.Errorf("unable to unmarshal JSON: %w", err)
	}

	res, err := gojsonschema.Validate(
		gojsonschema.NewGoLoader(schema),
		gojsonschema.NewStringLoader(in),
	)

	if err != nil {
		// JSON Schema is provided by API and if loading fails there is nothing a user can to to fix the issue.
		// One example is incompatible regex engines for pattern validation that will prevent loading JSON schema.
		// When that happens we should still allow running the command as API would validate request.
		return userSettings, nil
	}

	if !res.Valid() {
		return nil, errors.New(strings.Join(
			func() []string {
				errs := make([]string, len(res.Errors()))
				for i, err := range res.Errors() {
					errs[i] = err.String()
				}
				return errs
			}(),
			"\n",
		))
	}

	return userSettings, nil
}

// parseBackupSchedule parses a Database Service backup
// schedule value expressed in HH:MM format and returns the discrete values
// for hour and minute, or an error if the parsing failed.
func parseBackupSchedule(v string) (int64, int64, error) {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid value %q for backup schedule, expecting HH:MM", v)
	}

	backupHour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid value %q for backup schedule hour, must be between 0 and 23", v)
	}

	backupMinute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid value %q for backup schedule minute, must be between 0 and 59", v)
	}

	return int64(backupHour), int64(backupMinute), nil
}

// PartialSettingsPatch updates all keys in `data` that exist in `patch`.
// If key from `data` is not present in `patch` then removes the key from `data`.
func PartialSettingsPatch(data, patch map[string]interface{}) {
	for key := range data {
		if v, found := patch[key]; found {
			data[key] = v
		} else {
			delete(data, key)
		}
	}
}

// ResourceModelInterface defines necessary functions for interacting with resources through abstraction
type ResourceModelInterface interface {
	// ReadResource reads resource from remote and populate the model accordingly
	ReadResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics)
	// CreateResource creates the resource according to the model, and then
	// update computed fields if applicable
	CreateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics)
	// DeleteResource deletes the resource
	DeleteResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics)
	// UpdateResource updates the remote resource w/ the new model
	UpdateResource(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics)

	// WaitForService waits for the service to be be available for resource updates.
	WaitForService(ctx context.Context, client *exoscale.Client, diagnostics *diag.Diagnostics)

	// Accessing and setting attributes
	GetTimeouts() timeouts.Value
	SetTimeouts(timeouts.Value)
	GetID() basetypes.StringValue
	GetZone() basetypes.StringValue

	// Should set the return value of .GetID() to service/username
	GenerateID()
}

func ReadResource[T ResourceModelInterface](ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse, data T, client *exoscale.Client) {

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.GetTimeouts().Read(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	data.GenerateID()

	client, err := utils.SwitchClientZone(
		ctx,
		client,
		exoscale.ZoneName(data.GetZone().ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	data.ReadResource(ctx, client, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource read done", map[string]interface{}{
		"id": data.GetID(),
	})

}

func ReadResourceForImport[T ResourceModelInterface](ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse, data T, client *exoscale.Client) {

	// Set timeout
	t, diags := data.GetTimeouts().Read(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	data.GenerateID()

	client, err := utils.SwitchClientZone(
		ctx,
		client,
		exoscale.ZoneName(data.GetZone().ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	data.ReadResource(ctx, client, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource read done", map[string]interface{}{
		"id": data.GetID(),
	})

}

func CreateResource[T ResourceModelInterface](ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse, data T, client *exoscale.Client) {

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.GetTimeouts().Create(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	data.GenerateID()

	client, err := utils.SwitchClientZone(
		ctx,
		client,
		exoscale.ZoneName(data.GetZone().ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	data.WaitForService(ctx, client, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	data.CreateResource(ctx, client, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "resource created", map[string]interface{}{
		"id": data.GetID(),
	})

}

func UpdateResource[T ResourceModelInterface](ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse, stateData, planData T, client *exoscale.Client) {
	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	// Read Terraform state data (for comparison) into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := stateData.GetTimeouts().Update(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	client, err := utils.SwitchClientZone(
		ctx,
		client,
		exoscale.ZoneName(planData.GetZone().ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	planData.WaitForService(ctx, client, &resp.Diagnostics)
	planData.UpdateResource(ctx, client, &diags)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)

	tflog.Trace(ctx, "resource updated", map[string]interface{}{
		"id": planData.GetID(),
	})
}

func DeleteResource[T ResourceModelInterface](ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse, data T, client *exoscale.Client) {
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Set timeout
	t, diags := data.GetTimeouts().Delete(ctx, config.DefaultTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	data.GenerateID()

	client, err := utils.SwitchClientZone(
		ctx,
		client,
		exoscale.ZoneName(data.GetZone().ValueString()),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"unable to change exoscale client zone",
			err.Error(),
		)
		return
	}

	data.DeleteResource(ctx, client, &diags)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "resource deleted", map[string]interface{}{
		"id": data.GetID(),
	})

}
