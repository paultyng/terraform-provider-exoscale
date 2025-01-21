package instance_pool

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	v3 "github.com/exoscale/egoscale/v3"

	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

const (
	DefaultInstancePrefix = "pool"
)

func Resource() *schema.Resource {
	s := map[string]*schema.Schema{
		AttrAntiAffinityGroupIDs: {
			Description:   "A list of [exoscale_anti_affinity_group](./anti_affinity_group.md) (IDs; may only be set at creation time).",
			Type:          schema.TypeSet,
			Optional:      true,
			Set:           schema.HashString,
			Elem:          &schema.Schema{Type: schema.TypeString},
			ConflictsWith: []string{AttrAffinityGroupIDs},
		},
		AttrAffinityGroupIDs: {
			Description:   "A list of [exoscale_anti_affinity_group](./anti_affinity_group.md) (IDs; may only be set at creation time).",
			Type:          schema.TypeSet,
			Optional:      true,
			Set:           schema.HashString,
			Elem:          &schema.Schema{Type: schema.TypeString},
			Deprecated:    "Use anti_affinity_group_ids instead.",
			ConflictsWith: []string{AttrAntiAffinityGroupIDs},
		},
		AttrDeployTargetID: {
			Description: "A deploy target ID.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrDescription: {
			Description: "A free-form text describing the pool.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrDiskSize: {
			Description: "The managed instances disk size (GiB).",
			Type:        schema.TypeInt,
			Computed:    true,
			Optional:    true,
		},
		AttrElasticIPIDs: {
			Description: "A list of [exoscale_elastic_ip](./elastic_ip.md) (IDs).",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrInstancePrefix: {
			Description: "The string used to prefix managed instances name (default: `pool`).",
			Type:        schema.TypeString,
			Optional:    true,
			Default:     DefaultInstancePrefix,
		},
		AttrInstanceType: {
			// TODO: as long as "service_offering" is still deprecated but supported,
			//  we cannot make "instance_type" required as it'd break existing configurations.
			//  As soon as the "service_offering" parameter is phased out, the schema must be changed:
			//  - Optional:true must become Required:true
			//  - Computed:true must be removed
			Description:      "The managed compute instances type (`<family>.<size>`, e.g. `standard.medium`; use the [Exoscale CLI](https://github.com/exoscale/cli/) - `exo compute instance-type list` - for the list of available types).",
			Type:             schema.TypeString,
			Optional:         true,
			Computed:         true,
			ConflictsWith:    []string{AttrServiceOffering},
			ValidateDiagFunc: utils.ValidateComputeInstanceType,
			// Ignore case differences
			DiffSuppressFunc: utils.SuppressCaseDiff,
		},
		AttrIPv6: {
			Description: "Enable IPv6 on managed instances (boolean; default: `false`).",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		AttrKeyPair: {
			Description: "The [exoscale_ssh_key](./ssh_key.md) (name) to authorize in the managed instances.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrLabels: {
			Description: "A map of key/value labels.",
			Type:        schema.TypeMap,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Optional:    true,
		},
		AttrName: {
			Description: "The instance pool name.",
			Type:        schema.TypeString,
			Required:    true,
		},
		AttrNetworkIDs: {
			Description: "A list of [exoscale_private_network](./private_network.md) (IDs).",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrSecurityGroupIDs: {
			Description: "A list of [exoscale_security_group](./security_group.md) (IDs).",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrServiceOffering: {
			Description:   "The managed instances type. Please use the `instance_type` argument instead.",
			Type:          schema.TypeString,
			Optional:      true,
			Computed:      true,
			Deprecated:    `This attribute has been replaced by "instance_type".`,
			ConflictsWith: []string{AttrInstanceType},
			ValidateFunc:  utils.ValidateLowercaseString,
		},
		AttrSize: {
			Description:  "The number of managed instances.",
			Type:         schema.TypeInt,
			Required:     true,
			ValidateFunc: validation.IntAtLeast(1),
		},
		AttrMinAvailable: {
			Description:  "Minimum number of running Instances.",
			Type:         schema.TypeInt,
			Computed:     true,
			Optional:     true,
			ValidateFunc: validation.IntAtLeast(0),
		},
		AttrState: {
			Type:     schema.TypeString,
			Optional: true,
			Computed: true,
		},
		AttrTemplateID: {
			Description: "The [exoscale_template](../data-sources/template.md) (ID) to use when creating the managed instances.",
			Type:        schema.TypeString,
			Required:    true,
		},
		AttrUserData: {
			Description: "[cloud-init](http://cloudinit.readthedocs.io/) configuration to apply to the managed instances.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrVirtualMachines: {
			Description: "The list of managed instances (IDs). Please use the `instances.*.id` attribute instead.",
			Type:        schema.TypeSet,
			Optional:    true,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Deprecated:  "Use the instances exported attribute instead.",
		},
		AttrInstances: {
			Description: "The list of managed instances. Structure is documented below.",
			Type:        schema.TypeSet,
			Optional:    true,
			Computed:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					AttrInstanceID: {
						Type:     schema.TypeString,
						Optional: true,
					},
					AttrInstanceIPv6Address: {
						Description: "The instance (main network interface) IPv6 address.",
						Type:        schema.TypeString,
						Computed:    true,
					},
					AttrInstanceName: {
						Description: "The instance name.",
						Type:        schema.TypeString,
						Optional:    true,
					},
					AttrInstancePublicIPAddress: {
						Description: "The instance (main network interface) IPv4 address.",
						Type:        schema.TypeString,
						Computed:    true,
					},
				},
			},
		},
		AttrZone: {
			Description: "The Exoscale [Zone](https://www.exoscale.com/datacenters/) name.",
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
		},
	}

	return &schema.Resource{
		Description: `Manage Exoscale [Instance Pools](https://community.exoscale.com/documentation/compute/instance-pools/).

Corresponding data sources: [exoscale_instance_pool](../data-sources/instance_pool.md), [exoscale_instance_pool_list](../data-sources/instance_pool_list.md).`,
		Schema: s,

		CreateContext: rCreate,
		ReadContext:   rRead,
		UpdateContext: rUpdate,
		DeleteContext: rDelete,

		Importer: &schema.ResourceImporter{
			StateContext: utils.ZonedStateContextFunc,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(config.DefaultTimeout),
			Read:   schema.DefaultTimeout(config.DefaultTimeout),
			Update: schema.DefaultTimeout(config.DefaultTimeout),
			Delete: schema.DefaultTimeout(config.DefaultTimeout),
		},
	}
}

func rCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics { //nolint:gocyclo
	tflog.Debug(ctx, "beginning create", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	zone := d.Get(AttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutCreate))
	defer cancel()

	defaultClientV3, err := config.GetClientV3(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	client, err := utils.SwitchClientZone(
		ctx,
		defaultClientV3,
		v3.ZoneName(zone),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	createPoolRequest := new(v3.CreateInstancePoolRequest)

	if v, ok := d.GetOk(AttrDeployTargetID); ok {
		s := v.(string)
		createPoolRequest.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(s),
		}
	}

	if v, ok := d.GetOk(AttrDescription); ok {
		s := v.(string)
		createPoolRequest.Description = s
	}

	if v, ok := d.GetOk(AttrDiskSize); ok {
		i := int64(v.(int))
		createPoolRequest.DiskSize = i
	}

	if v, ok := d.GetOk(AttrName); ok {
		s := v.(string)
		createPoolRequest.Name = s
	}

	if v, ok := d.GetOk(AttrInstancePrefix); ok {
		s := v.(string)
		createPoolRequest.InstancePrefix = s
	}

	if v, ok := d.GetOk(AttrKeyPair); ok {
		s := v.(string)
		createPoolRequest.SSHKey = &v3.SSHKey{Name: s}
	}

	if l, ok := d.GetOk(AttrLabels); ok {
		labels := make(map[string]string)
		for k, v := range l.(map[string]interface{}) {
			labels[k] = v.(string)
		}
		createPoolRequest.Labels = labels
	}

	if v, ok := d.GetOk(AttrSize); ok {
		i := int64(v.(int))
		createPoolRequest.Size = i
	}
	if v, ok := d.GetOk(AttrMinAvailable); ok {
		i := int64(v.(int))
		createPoolRequest.MinAvailable = i
	}

	if v, ok := d.GetOk(AttrTemplateID); ok {
		s := v.(string)
		createPoolRequest.Template = &v3.Template{
			ID: v3.UUID(s),
		}
	}

	// FIXME: once the "instance_type" attribute has been made required, this check can be removed.
	if d.Get(AttrServiceOffering).(string) == "" &&
		d.Get(AttrInstanceType).(string) == "" {
		return diag.Errorf(
			"either %s or %s must be set",
			AttrServiceOffering,
			AttrInstanceType,
		)
	}
	instanceTypeCompoundName := d.Get(AttrServiceOffering).(string)
	if v, ok := d.GetOk(AttrInstanceType); ok {
		instanceTypeCompoundName = v.(string)
	}

	createPoolRequest.InstanceType, err = utils.FindInstanceTypeByNameV3(ctx, client, instanceTypeCompoundName)
	if err != nil {
		return diag.Errorf("error retrieving instance type: %s", err)
	}

	if v, ok := d.GetOk(AttrAffinityGroupIDs); ok {
		if set, ok := v.(*schema.Set); ok {
			createPoolRequest.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
		}
	}

	if v, ok := d.GetOk(AttrAntiAffinityGroupIDs); ok {
		if set, ok := v.(*schema.Set); ok {
			createPoolRequest.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
		}
	}

	if set, ok := d.Get(AttrSecurityGroupIDs).(*schema.Set); ok {
		createPoolRequest.SecurityGroups = utils.SecurityGroupIDsToSecurityGroups(set.List())
	}

	if set, ok := d.Get(AttrNetworkIDs).(*schema.Set); ok {
		createPoolRequest.PrivateNetworks = utils.PrivateNetworkIDsToPrivateNetworks(set.List())
	}

	if set, ok := d.Get(AttrElasticIPIDs).(*schema.Set); ok {
		createPoolRequest.ElasticIPS = utils.ElasticIPIDsToElasticIPs(set.List())
	}

	enableIPv6 := d.Get(AttrIPv6).(bool)
	createPoolRequest.Ipv6Enabled = &enableIPv6

	if v := d.Get(AttrUserData).(string); v != "" {
		userData, _, err := utils.EncodeUserData(v)
		if err != nil {
			return diag.FromErr(err)
		}
		createPoolRequest.UserData = userData
	}

	op, err := client.CreateInstancePool(ctx, *createPoolRequest)
	if err != nil {
		return diag.FromErr(err)
	}

	op, err = client.Wait(ctx, op, v3.OperationStateSuccess)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(op.Reference.ID.String())

	tflog.Debug(ctx, "create finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return rRead(ctx, d, meta)
}

func rRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning read", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	zone := d.Get(AttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutRead))
	defer cancel()

	defaultClientV3, err := config.GetClientV3(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	client, err := utils.SwitchClientZone(
		ctx,
		defaultClientV3,
		v3.ZoneName(zone),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	pool, err := client.GetInstancePool(ctx, v3.UUID(d.Id()))
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			// Resource doesn't exist anymore, signaling the core to remove it from the state.
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "read finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return rApply(ctx, client, d, pool)
}

func rUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning update", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	zone := d.Get(AttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutUpdate))
	defer cancel()

	defaultClientV3, err := config.GetClientV3(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	client, err := utils.SwitchClientZone(
		ctx,
		defaultClientV3,
		v3.ZoneName(zone),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	updateRequest := v3.UpdateInstancePoolRequest{}
	var updated bool

	// We need to explicitely specify the AntiaffinityGroups on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of AAGs
	set := d.Get(AttrAffinityGroupIDs).(*schema.Set)
	updateRequest.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
	if d.HasChange(AttrAffinityGroupIDs) {
		updated = true
	} else {

	}

	// We need to explicitely specify the AntiaffinityGroups on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of AAGs
	set = d.Get(AttrAntiAffinityGroupIDs).(*schema.Set)
	updateRequest.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
	if d.HasChange(AttrAntiAffinityGroupIDs) {
		updated = true
	}

	// We need to explicitely specify the DeployTarget on
	// update otherwise the orchestrator will interpret that as
	// clearing the DeployTarget
	if v, getSuccess := d.GetOk(AttrDeployTargetID); getSuccess {
		updateRequest.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(v.(string)),
		}
	}
	if d.HasChange(AttrDeployTargetID) {
		updated = true
	}

	if d.HasChange(AttrDescription) {
		v := d.Get(AttrDescription).(string)
		updateRequest.Description = v
		updated = true
	}

	if d.HasChange(AttrDiskSize) {
		v := int64(d.Get(AttrDiskSize).(int))
		updateRequest.DiskSize = v
		updated = true
	}

	// We need to explicitely specify the Elastic IPs on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of assocaited EIPs
	set = d.Get(AttrElasticIPIDs).(*schema.Set)
	updateRequest.ElasticIPS = utils.ElasticIPIDsToElasticIPs(set.List())
	if d.HasChange(AttrElasticIPIDs) {
		updated = true
	}

	if d.HasChange(AttrInstancePrefix) {
		v := d.Get(AttrInstancePrefix).(string)
		updateRequest.InstancePrefix = &v
		updated = true
	}

	if d.HasChange(AttrIPv6) {
		v := d.Get(AttrIPv6).(bool)
		updateRequest.Ipv6Enabled = &v
		updated = true
	}

	// We need to explicitely specify the SSHKey on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of associated SSH key
	v := d.Get(AttrKeyPair).(string)
	updateRequest.SSHKey = &v3.SSHKey{
		Name: v,
	}
	if d.HasChange(AttrKeyPair) {
		updated = true
	}

	if d.HasChange(AttrLabels) {
		labels := make(map[string]string)
		for k, v := range d.Get(AttrLabels).(map[string]interface{}) {
			labels[k] = v.(string)
		}
		updateRequest.Labels = labels
		updated = true
	}

	if d.HasChange(AttrName) {
		v := d.Get(AttrName).(string)
		updateRequest.Name = v
		updated = true
	}

	// We need to explicitely specify the PrivateNetworks on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of PrivNets
	set = d.Get(AttrNetworkIDs).(*schema.Set)
	updateRequest.PrivateNetworks = utils.PrivateNetworkIDsToPrivateNetworks(set.List())
	if d.HasChange(AttrNetworkIDs) {
		updated = true
	}

	// We need to explicitely specify the SecurityGroups on
	// update otherwise the orchestrator will interpret that as
	// clearing the list of SecurityGroups
	set = d.Get(AttrSecurityGroupIDs).(*schema.Set)
	updateRequest.SecurityGroups = utils.SecurityGroupIDsToSecurityGroups(set.List())
	if d.HasChange(AttrSecurityGroupIDs) {
		updated = true
	}

	if d.HasChange(AttrInstanceType) {

		instanceType, err := utils.FindInstanceTypeByNameV3(ctx, client, d.Get(AttrInstanceType).(string))
		if err != nil {
			return diag.Errorf("error retrieving instance type: %s", err)
		}
		updateRequest.InstanceType = &v3.InstanceType{
			ID: instanceType.ID,
		}
		updated = true
	}

	if d.HasChange(AttrTemplateID) {
		v := d.Get(AttrTemplateID).(string)
		updateRequest.Template = &v3.Template{
			ID: v3.UUID(v),
		}
		updated = true
	}

	if d.HasChange(AttrUserData) {
		v, _, err := utils.EncodeUserData(d.Get(AttrUserData).(string))
		if err != nil {
			return diag.FromErr(err)
		}
		updateRequest.UserData = &v
		updated = true
	}

	if d.HasChange(AttrMinAvailable) {
		v := int64(d.Get(AttrMinAvailable).(int))
		updateRequest.MinAvailable = &v
		updated = true
	}

	if updated {
		op, err := client.UpdateInstancePool(ctx, v3.UUID(d.Id()), updateRequest)
		if err != nil {
			return diag.FromErr(err)
		}
		_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange(AttrSize) {

		op, err := client.ScaleInstancePool(ctx, v3.UUID(d.Id()), v3.ScaleInstancePoolRequest{
			Size: int64(d.Get(AttrSize).(int)),
		})
		if err != nil {
			return diag.FromErr(err)
		}
		_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	tflog.Debug(ctx, "update finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return rRead(ctx, d, meta)
}

func rDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning delete", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	zone := d.Get(AttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutDelete))
	defer cancel()

	defaultClientV3, err := config.GetClientV3(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	client, err := utils.SwitchClientZone(
		ctx,
		defaultClientV3,
		v3.ZoneName(zone),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	op, err := client.DeleteInstancePool(ctx, v3.UUID(d.Id()))
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
	if err != nil {
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "delete finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return nil
}

func rApply(ctx context.Context, client *v3.Client, d *schema.ResourceData, pool *v3.InstancePool) diag.Diagnostics { //nolint:gocyclo

	if pool.AntiAffinityGroups != nil {
		antiAffinityGroupIDs := make([]string, len(pool.AntiAffinityGroups))
		copy(antiAffinityGroupIDs, utils.AntiAffiniGroupsToAntiAffinityGroupIDs(pool.AntiAffinityGroups))

		if _, ok := d.GetOk(AttrAffinityGroupIDs); ok {
			if err := d.Set(AttrAffinityGroupIDs, antiAffinityGroupIDs); err != nil {
				return diag.FromErr(err)
			}
		}

		if _, ok := d.GetOk(AttrAntiAffinityGroupIDs); ok {
			if err := d.Set(AttrAntiAffinityGroupIDs, antiAffinityGroupIDs); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	if pool.DeployTarget != nil {
		dpId := pool.DeployTarget.ID.String()
		if err := d.Set(AttrDeployTargetID, utils.DefaultString(&dpId, "")); err != nil {
			return diag.FromErr(err)
		}

	}

	if err := d.Set(AttrDescription, utils.DefaultString(&pool.Description, "")); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrDiskSize, pool.DiskSize); err != nil {
		return diag.FromErr(err)
	}

	if pool.ElasticIPS != nil {
		elasticIPIDs := make([]string, len(pool.ElasticIPS))
		copy(elasticIPIDs, utils.ElasticIPsToElasticIPIDs(pool.ElasticIPS))

		if err := d.Set(AttrElasticIPIDs, elasticIPIDs); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrInstancePrefix, utils.DefaultString(&pool.InstancePrefix, "")); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrIPv6, utils.DefaultBool(pool.Ipv6Enabled, false)); err != nil {
		return diag.FromErr(err)
	}

	if pool.SSHKey != nil {
		if err := d.Set(AttrKeyPair, pool.SSHKey.Name); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrLabels, pool.Labels); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrName, pool.Name); err != nil {
		return diag.FromErr(err)
	}

	if pool.PrivateNetworks != nil {
		if err := d.Set(AttrNetworkIDs, utils.PrivateNetworksToPrivateNetworkIDs(pool.PrivateNetworks)); err != nil {
			return diag.FromErr(err)
		}
	}

	if pool.SecurityGroups != nil {
		if err := d.Set(AttrSecurityGroupIDs, utils.SecurityGroupsToSecurityGroupIDs(pool.SecurityGroups)); err != nil {
			return diag.FromErr(err)
		}
	}

	instanceType, err := client.GetInstanceType(
		ctx,
		pool.InstanceType.ID,
	)
	if err != nil {
		return diag.Errorf("error retrieving instance type: %s", err)
	}
	if err := d.Set(AttrInstanceType, fmt.Sprintf(
		"%s.%s",
		strings.ToLower(string(instanceType.Family)),
		strings.ToLower(string(instanceType.Size)),
	)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set(AttrServiceOffering, strings.ToLower(string(instanceType.Size))); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrSize, pool.Size); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrMinAvailable, pool.MinAvailable); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrState, pool.State); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrTemplateID, pool.Template.ID); err != nil {
		return diag.FromErr(err)
	}

	if pool.UserData != "" {
		userData, err := utils.DecodeUserData(pool.UserData)
		if err != nil {
			return diag.Errorf("error decoding user data: %s", err)
		}
		if err := d.Set(AttrUserData, userData); err != nil {
			return diag.FromErr(err)
		}
	}

	if pool.Instances != nil {
		instanceIDs := make([]string, len(pool.Instances))
		instanceDetails := make([]interface{}, len(pool.Instances))

		for k, i := range pool.Instances {
			instanceIDs[k] = i.ID.String()

			// instance details
			instance, err := client.GetInstance(ctx, i.ID)
			if err != nil {
				return diag.FromErr(err)
			}

			instanceDetails[k] = computeInstanceToResource(instance)
		}

		if err := d.Set(AttrVirtualMachines, instanceIDs); err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set(AttrInstances, instanceDetails); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func computeInstanceToResource(instance *v3.Instance) interface{} {
	c := make(map[string]interface{})
	c[AttrInstanceID] = instance.ID
	c[AttrInstanceIPv6Address] = instance.Ipv6Address
	c[AttrInstanceName] = instance.Name
	c[AttrInstancePublicIPAddress] = utils.AddressToStringPtr(&instance.PublicIP)
	return c
}
