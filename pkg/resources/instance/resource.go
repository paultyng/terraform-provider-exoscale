package instance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	v3 "github.com/exoscale/egoscale/v3"

	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

func Resource() *schema.Resource {
	s := map[string]*schema.Schema{
		AttrAntiAffinityGroupIDs: {
			Description: "A list of [exoscale_anti_affinity_group](./anti_affinity_group.md) (IDs) to attach to the instance (may only be set at creation time).",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			ForceNew:    true,
		},
		AttrCreatedAt: {
			Description: "The instance creation date.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrDestroyProtected: {
			Description: "Mark the instance as protected, the Exoscale API will refuse to delete the instance until the protection is removed (boolean; default: `false`).",
			Type:        schema.TypeBool,
			Optional:    true,
		},
		AttrDeployTargetID: {
			Description: "A deploy target ID.",
			Type:        schema.TypeString,
			Optional:    true,
			ForceNew:    true,
		},
		AttrDiskSize: {
			Description:  "The instance disk size (GiB; at least `10`). Can not be decreased after creation. **WARNING**: updating this attribute stops/restarts the instance.",
			Type:         schema.TypeInt,
			Computed:     true,
			Optional:     true,
			ValidateFunc: validation.IntAtLeast(10),
		},
		AttrElasticIPIDs: {
			Description: "A list of [exoscale_elastic_ip](./elastic_ip.md) (IDs) to attach to the instance.",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrIPv6: {
			Description: "Enable IPv6 on the instance (boolean; default: `false`).",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		AttrMACAddress: {
			Description: "MAC address",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrIPv6Address: {
			Description: "The instance (main network interface) IPv6 address (if enabled).",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrLabels: {
			Description: "A map of key/value labels.",
			Type:        schema.TypeMap,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Optional:    true,
		},
		AttrName: {
			Description: "The compute instance name.",
			Type:        schema.TypeString,
			Required:    true,
		},
		AttrPrivateNetworkIDs: {
			Description: "A list of private networks (IDs) attached to the instance. Please use the `network_interface.*.network_id` argument instead.",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Deprecated:  "Use the network_interface block instead.",
		},
		AttrNetworkInterface: {
			Description: "Private network interfaces (may be specified multiple times). Structure is documented below.",
			Type:        schema.TypeSet,
			Optional:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"ip_address": {
						Description:      "The IPv4 address to request as static DHCP lease if the network interface is attached to a *managed* private network.",
						Type:             schema.TypeString,
						Optional:         true,
						Computed:         true,
						ValidateDiagFunc: validation.ToDiagFunc(validation.IsIPv4Address),
					},
					"network_id": {
						Description: "The [exoscale_private_network](./private_network.md) (ID) to attach to the instance.",
						Type:        schema.TypeString,
						Required:    true,
					},
					"mac_address": {
						Description: "MAC address",
						Type:        schema.TypeString,
						Computed:    true,
					},
				},
			},
		},
		AttrBlockStorageVolumeIDs: {
			Description: "A list of [exoscale_block_storage_volume](./block_storage_volume.md) (ID) to attach to the instance.",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrPublicIPAddress: {
			Description: "The instance (main network interface) IPv4 address.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrPrivate: {
			Description: "Whether the instance is private (no public IP addresses; default: false)",
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
		},
		AttrReverseDNS: {
			Description: "Domain name for reverse DNS record.",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrSSHKey: {
			Description:   "The [exoscale_ssh_key](./ssh_key.md) (name) to authorize in the instance (may only be set at creation time).",
			Type:          schema.TypeString,
			Optional:      true,
			Deprecated:    "Use ssh_keys instead",
			ConflictsWith: []string{AttrSSHKeys},
		},
		AttrSSHKeys: {
			Description: "The list of [exoscale_ssh_key](./ssh_key.md) (name) to authorize in the instance (may only be set at creation time).",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrSecurityGroupIDs: {
			Description: "A list of [exoscale_security_group](./security_group.md) (IDs) to attach to the instance.",
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrState: {
			Description: "The instance state (`running` or `stopped`; default: `running`).",
			Type:        schema.TypeString,
			Optional:    true,
			Computed:    true,
		},
		AttrTemplateID: {
			Description: "The [exoscale_template](../data-sources/template.md) (ID) to use when creating the instance.",
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
		},
		AttrType: {
			Description:      "The instance type (`<family>.<size>`, e.g. `standard.medium`; use the [Exoscale CLI](https://github.com/exoscale/cli/) - `exo compute instance-type list` - for the list of available types). **WARNING**: updating this attribute stops/restarts the instance.",
			Type:             schema.TypeString,
			Required:         true,
			ValidateDiagFunc: utils.ValidateComputeInstanceType,
			// Ignore case differences
			DiffSuppressFunc: utils.SuppressCaseDiff,
		},
		AttrUserData: {
			Description:      "[cloud-init](https://cloudinit.readthedocs.io/) configuration.",
			Type:             schema.TypeString,
			ValidateDiagFunc: utils.ValidateComputeUserData,
			Optional:         true,
		},
		AttrZone: {
			Description: "The Exoscale [Zone](https://www.exoscale.com/datacenters/) name.",
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
		},
	}

	return &schema.Resource{
		Schema: s,

		Description: "Manage Exoscale [Compute Instances](https://community.exoscale.com/documentation/compute/).\n" +
			"\n" +
			"Corresponding data sources: [exoscale_compute_instance](../data-sources/compute_instance.md), [exoscale_compute_instance_list](../data-sources/compute_instance_list.md).\n" +
			"\n" +
			"After the creation, you can retrieve the password of an instance with [Exoscale CLI](https://github.com/exoscale/cli): `exo compute instance reveal-password NAME`.",

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
	clientV3, err := utils.SwitchClientZone(
		ctx,
		defaultClientV3,
		v3.ZoneName(zone),
	)
	if err != nil {
		return diag.FromErr(err)
	}

	instanceRequest := &v3.CreateInstanceRequest{
		Name:     d.Get(AttrName).(string),
		Template: &v3.Template{ID: v3.UUID(d.Get(AttrTemplateID).(string))},
	}

	if set, ok := d.Get(AttrAntiAffinityGroupIDs).(*schema.Set); ok {
		instanceRequest.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
	}

	if v, ok := d.GetOk(AttrDeployTargetID); ok {
		s := v.(string)
		instanceRequest.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(s),
		}
	}

	if v, ok := d.GetOk(AttrDiskSize); ok {
		i := int64(v.(int))
		instanceRequest.DiskSize = i
	}

	if privateInstance, ok := d.GetOk(AttrPrivate); ok {
		privateInstanceBool := privateInstance.(bool)
		if privateInstanceBool {
			t := "none"
			instanceRequest.PublicIPAssignment = v3.PublicIPAssignment(t)
		}
	} else if enableIPv6, ok := d.GetOk(AttrIPv6); ok {
		ipv6EnabledBool := enableIPv6.(bool)
		if ipv6EnabledBool {
			t := "dual"
			instanceRequest.PublicIPAssignment = v3.PublicIPAssignment(t)
		}
	} else {
		t := "inet4"
		instanceRequest.PublicIPAssignment = v3.PublicIPAssignment(t)
	}

	if l, ok := d.GetOk(AttrLabels); ok {
		labels := make(map[string]string)
		for k, v := range l.(map[string]interface{}) {
			labels[k] = v.(string)
		}
		instanceRequest.Labels = labels
	}

	if v, ok := d.GetOk(AttrSSHKeys); ok {
		keySet := v.(*schema.Set)
		if keySet.Len() > 0 {
			keys := make([]v3.SSHKey, keySet.Len())
			for i, k := range keySet.List() {
				keys[i] = v3.SSHKey{Name: k.(string)}
			}
			instanceRequest.SSHKeys = keys

		}
	} else if v, ok := d.GetOk(AttrSSHKey); ok {
		s := v.(string)
		instanceRequest.SSHKey = &v3.SSHKey{Name: s}
	}

	if set, ok := d.Get(AttrSecurityGroupIDs).(*schema.Set); ok {
		instanceRequest.SecurityGroups = utils.SecurityGroupIDsToSecurityGroups(set.List())
	}

	instanceType, err := utils.FindInstanceTypeByNameV3(ctx, clientV3, d.Get(AttrType).(string))
	if err != nil {
		return diag.Errorf("unable to retrieve instance type: %s", err)
	}
	instanceRequest.InstanceType = instanceType

	if v := d.Get(AttrUserData).(string); v != "" {
		userData, _, err := utils.EncodeUserData(v)
		if err != nil {
			return diag.FromErr(err)
		}
		instanceRequest.UserData = userData
	}

	op, err := clientV3.CreateInstance(ctx, *instanceRequest)
	if err != nil {
		return diag.FromErr(err)
	}
	op, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess)
	if err != nil {
		return diag.FromErr(err)
	}

	instanceId := op.Reference.ID

	if isDestroyProtected, ok := d.GetOk(AttrDestroyProtected); ok && isDestroyProtected.(bool) {
		_, err := clientV3.AddInstanceProtection(ctx, instanceId)
		if err != nil {
			return diag.Errorf("unable to make instance %s destroy protected: %s", instanceId, err)
		}
	}

	if set, ok := d.Get(AttrElasticIPIDs).(*schema.Set); ok {
		if set.Len() > 0 {
			for _, id := range set.List() {
				op, err := clientV3.AttachInstanceToElasticIP(
					ctx,
					v3.UUID(id.(string)),
					v3.AttachInstanceToElasticIPRequest{Instance: &v3.InstanceTarget{ID: instanceId}},
				)
				if err != nil {
					return diag.Errorf("unable to attach Elastic IP %s: %s", id.(string), err)
				}
				if _, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.Errorf("unable to attach Elastic IP %s: %s", id.(string), err)
				}
			}
		}
	}

	if nifSet, ok := d.Get(AttrNetworkInterface).(*schema.Set); ok {
		for _, nif := range nifSet.List() {
			nif, err := NewNetworkInterface(nif)
			if err != nil {
				return diag.FromErr(err)
			}

			op, err := clientV3.AttachInstanceToPrivateNetwork(
				ctx,
				v3.UUID(nif.NetworkID),
				v3.AttachInstanceToPrivateNetworkRequest{
					Instance: &v3.AttachInstanceToPrivateNetworkRequestInstance{
						ID: instanceId,
					},
					IP: net.ParseIP(*nif.IPAddress),
				},
			)
			if err != nil {
				return diag.Errorf("unable to attach Private Network %s: %s", nif.NetworkID, err)
			}
			if _, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to attach Private Network %s: %s", nif.NetworkID, err)
			}
		}
	}

	// Attach block storage volumes if set
	if bsSet, ok := d.Get(AttrBlockStorageVolumeIDs).(*schema.Set); ok {
		for _, bs := range bsSet.List() {
			bid, err := v3.ParseUUID(bs.(string))
			if err != nil {
				return diag.Errorf("unable to parse block storage ID: %s", err)
			}

			request := v3.AttachBlockStorageVolumeToInstanceRequest{
				Instance: &v3.InstanceTarget{
					ID: instanceId,
				},
			}

			op, err := clientV3.AttachBlockStorageVolumeToInstance(
				ctx,
				bid,
				request,
			)
			if err != nil {
				return diag.Errorf("unable to parse attached instance ID: %s", err)
			}

			_, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess)
			if err != nil {
				return diag.Errorf("failed to create block storage: %s", err)
			}
		}
	}

	if v, ok := d.GetOk(AttrReverseDNS); ok {
		rdns := v.(string)
		op, err := clientV3.UpdateReverseDNSInstance(
			ctx,
			instanceId,
			v3.UpdateReverseDNSInstanceRequest{
				DomainName: rdns,
			},
		)
		if err != nil {
			return diag.Errorf("unable to create Reverse DNS record: %s", err)
		}
		if _, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.Errorf("unable to create Reverse DNS record: %s", err)
		}

	}

	if v := d.Get(AttrState).(string); v == "stopped" {
		op, err := clientV3.StopInstance(ctx, instanceId)
		if err != nil {
			return diag.Errorf("unable to stop instance: %s", err)
		}
		if _, err = clientV3.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.Errorf("unable to stop instance: %s", err)
		}
	}

	d.SetId(string(instanceId))

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

	clientV3, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	instance, err := clientV3.GetInstance(ctx, v3.UUID(d.Id()))
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

	return rApply(ctx, clientV3, d, instance)
}

func rUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics { //nolint:gocyclo
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

	instance, err := client.GetInstance(ctx, v3.UUID(d.Id()))
	if err != nil {
		return diag.FromErr(err)
	}

	var updated bool
	instanceUpdateRequest := v3.UpdateInstanceRequest{}

	if d.HasChange(AttrLabels) {
		labels := make(map[string]string)
		for k, v := range d.Get(AttrLabels).(map[string]interface{}) {
			labels[k] = v.(string)
		}
		instanceUpdateRequest.Labels = labels
		updated = true
	}

	if d.HasChange(AttrName) {
		v := d.Get(AttrName).(string)
		instanceUpdateRequest.Name = v
		updated = true
	}

	if d.HasChange(AttrUserData) {
		v, _, err := utils.EncodeUserData(d.Get(AttrUserData).(string))
		if err != nil {
			return diag.FromErr(err)
		}
		instanceUpdateRequest.UserData = v
		updated = true
	}

	if updated {
		op, err := client.UpdateInstance(ctx, instance.ID, instanceUpdateRequest)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange(AttrReverseDNS) {
		rdns := d.Get(AttrReverseDNS).(string)
		if rdns == "" {
			op, err := client.DeleteReverseDNSInstance(
				ctx,
				instance.ID,
			)
			if err != nil {
				return diag.FromErr(err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.FromErr(err)
			}
		} else {
			op, err := client.UpdateReverseDNSInstance(
				ctx,
				instance.ID,
				v3.UpdateReverseDNSInstanceRequest{
					DomainName: rdns,
				},
			)
			if err != nil {
				return diag.FromErr(err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	// Attach/detach Block Storage Volumes
	if d.HasChange(AttrBlockStorageVolumeIDs) {
		o, n := d.GetChange(AttrBlockStorageVolumeIDs)
		old := o.(*schema.Set)
		cur := n.(*schema.Set)

		if added := cur.Difference(old); added.Len() > 0 {
			for _, id := range added.List() {
				bid, err := v3.ParseUUID(id.(string))
				if err != nil {
					return diag.Errorf("unable to parse block storage ID: %s", err)
				}

				request := v3.AttachBlockStorageVolumeToInstanceRequest{
					Instance: &v3.InstanceTarget{
						ID: instance.ID,
					},
				}

				op, err := client.AttachBlockStorageVolumeToInstance(
					ctx,
					bid,
					request,
				)
				if err != nil {
					return diag.Errorf("unable to parse attached instance ID: %s", err)
				}

				_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
				if err != nil {
					return diag.Errorf("failed to attach block storage: %s", err)
				}
			}
		}

		if removed := old.Difference(cur); removed.Len() > 0 {
			for _, id := range removed.List() {
				bid, err := v3.ParseUUID(id.(string))
				if err != nil {
					return diag.Errorf("unable to parse block storage ID: %s", err)
				}

				op, err := client.DetachBlockStorageVolume(
					ctx,
					bid,
				)
				if err != nil {
					// Ideally we would have a custom error defined in OpenAPI spec & egoscale.
					// For now we just check the error text.
					if strings.HasSuffix(err.Error(), "Volume not attached") {
						tflog.Info(ctx, "volume not attached")
						continue
					}

					return diag.Errorf("failed to detach block storage: %s", err)
				}

				_, err = client.Wait(ctx, op, v3.OperationStateSuccess)
				if err != nil {
					return diag.Errorf("failed to detach block storage: %s", err)
				}
			}
		}
	}

	if d.HasChange(AttrElasticIPIDs) {
		o, n := d.GetChange(AttrElasticIPIDs)
		old := o.(*schema.Set)
		cur := n.(*schema.Set)

		if added := cur.Difference(old); added.Len() > 0 {
			for _, id := range added.List() {
				op, err := client.AttachInstanceToElasticIP(
					ctx,
					v3.UUID(id.(string)),
					v3.AttachInstanceToElasticIPRequest{Instance: &v3.InstanceTarget{ID: instance.ID}},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		if removed := old.Difference(cur); removed.Len() > 0 {
			for _, id := range removed.List() {
				op, err := client.DetachInstanceFromElasticIP(
					ctx,
					v3.UUID(id.(string)),
					v3.DetachInstanceFromElasticIPRequest{Instance: &v3.InstanceTarget{ID: instance.ID}},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	if d.HasChange(AttrNetworkInterface) {
		o, n := d.GetChange(AttrNetworkInterface)
		old := o.(*schema.Set)
		cur := n.(*schema.Set)

		if removed := old.Difference(cur); removed.Len() > 0 {
			for _, nif := range removed.List() {
				nif, err := NewNetworkInterface(nif)
				if err != nil {
					return diag.FromErr(err)
				}

				op, err := client.DetachInstanceFromPrivateNetwork(
					ctx,
					v3.UUID(nif.NetworkID),
					v3.DetachInstanceFromPrivateNetworkRequest{Instance: &v3.Instance{ID: instance.ID}},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		if added := cur.Difference(old); added.Len() > 0 {
			for _, nif := range added.List() {
				nif, err := NewNetworkInterface(nif)
				if err != nil {
					return diag.FromErr(err)
				}

				op, err := client.AttachInstanceToPrivateNetwork(
					ctx,
					v3.UUID(nif.NetworkID),
					v3.AttachInstanceToPrivateNetworkRequest{
						Instance: &v3.AttachInstanceToPrivateNetworkRequestInstance{ID: instance.ID},
						IP:       net.ParseIP(*nif.IPAddress),
					},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	if d.HasChange(AttrSecurityGroupIDs) {
		o, n := d.GetChange(AttrSecurityGroupIDs)
		old := o.(*schema.Set)
		cur := n.(*schema.Set)

		if added := cur.Difference(old); added.Len() > 0 {
			for _, id := range added.List() {
				op, err := client.AttachInstanceToSecurityGroup(
					ctx,
					v3.UUID(id.(string)),
					v3.AttachInstanceToSecurityGroupRequest{Instance: &v3.Instance{ID: instance.ID}},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		if removed := old.Difference(cur); removed.Len() > 0 {
			for _, id := range removed.List() {
				op, err := client.DetachInstanceFromSecurityGroup(
					ctx,
					v3.UUID(id.(string)),
					v3.DetachInstanceFromSecurityGroupRequest{Instance: &v3.Instance{ID: instance.ID}},
				)
				if err != nil {
					return diag.FromErr(err)
				}
				if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	if d.HasChanges(
		AttrState,
		AttrDiskSize,
		AttrType,
	) {
		// Check if size is below current size to prevent uneeded stop as API will prevent the scale operation
		if d.HasChange(AttrDiskSize) &&
			instance.DiskSize > int64(d.Get(AttrDiskSize).(int)) {
			return diag.Errorf("unable to scale down the disk size, use size > %v", instance.DiskSize)
		}

		// Compute instance scaling/disk resizing API operations requires the instance to be stopped.
		if d.Get(AttrState) == "stopped" ||
			d.HasChange(AttrDiskSize) ||
			d.HasChange(AttrType) {
			op, err := client.StopInstance(ctx, instance.ID)
			if err != nil {
				return diag.Errorf("unable to stop instance: %s", err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to stop instance: %s", err)
			}

		}

		if d.HasChange(AttrDiskSize) {
			op, err := client.ResizeInstanceDisk(
				ctx,
				instance.ID,
				v3.ResizeInstanceDiskRequest{DiskSize: int64(d.Get(AttrDiskSize).(int))},
			)
			if err != nil {
				return diag.Errorf("unable to resize disk: %s", err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to resize disk: %s", err)
			}

		}

		if d.HasChange(AttrType) {
			instanceType, err := utils.FindInstanceTypeByNameV3(ctx, client, d.Get(AttrType).(string))
			if err != nil {
				return diag.Errorf("unable to retrieve instance type: %s", err)
			}
			op, err := client.ScaleInstance(ctx, instance.ID, v3.ScaleInstanceRequest{InstanceType: instanceType})
			if err != nil {
				return diag.Errorf("unable to scale instance: %s", err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to scale instance: %s", err)
			}
		}

		if d.Get(AttrState) == "running" {
			op, err := client.StartInstance(ctx, instance.ID, v3.StartInstanceRequest{})
			if err != nil {
				return diag.Errorf("unable to start instance: %s", err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to start instance: %s", err)
			}

		}
	}

	// as we do not have a `get-instance-protection` API call,
	// the tf state of the `destroy_protected` field cannot be reconciled
	// and we cannot rely on d.HasChange to detect a change.
	// Therefore we simply apply what the practitioner configured
	// If the field is absent, the protection will be removed
	isDestroyProtected := d.Get(AttrDestroyProtected)
	if isDestroyProtected != nil {
		if isDestroyProtected.(bool) {
			op, err := client.AddInstanceProtection(ctx, instance.ID)
			if err != nil {
				return diag.Errorf("unable to make instance %s destroy protected: %s", instance.ID, err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to make instance %s destroy protected: %s", instance.ID, err)
			}
		} else {
			op, err := client.RemoveInstanceProtection(ctx, instance.ID)
			if err != nil {
				return diag.Errorf("unable to remove destroy protection from instance %s: %s", instance.ID, err)
			}
			if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
				return diag.Errorf("unable to remove destroy protection from instance %s: %s", instance.ID, err)
			}
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

	op, err := client.DeleteReverseDNSInstance(ctx, v3.UUID(d.Id()))
	if err != nil && !errors.Is(err, v3.ErrNotFound) {
		return diag.FromErr(err)
	}

	if op != nil {
		if _, err := client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.FromErr(err)
		}
	}

	op, err = client.DeleteInstance(
		ctx,
		v3.UUID(d.Id()),
	)
	if err != nil {
		return diag.FromErr(err)
	}
	if _, err := client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "delete finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return nil
}

func rApply( //nolint:gocyclo
	ctx context.Context,
	clientV3 *v3.Client,
	d *schema.ResourceData,
	instance *v3.Instance,
) diag.Diagnostics {
	if len(instance.AntiAffinityGroups) > 0 {
		antiAffinityGroupIDs := make([]string, len(instance.AntiAffinityGroups))
		for i, aag := range instance.AntiAffinityGroups {
			antiAffinityGroupIDs[i] = aag.ID.String()
		}

		if err := d.Set(AttrAntiAffinityGroupIDs, antiAffinityGroupIDs); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrCreatedAt, instance.CreatedAT.String()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(
		AttrDeployTargetID,
		func() string {
			if instance.DeployTarget != nil {
				return instance.DeployTarget.ID.String()
			} else {
				return ""
			}
		}(),
	); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrDiskSize, instance.DiskSize); err != nil {
		return diag.FromErr(err)
	}

	if len(instance.ElasticIPS) > 0 {
		elasticIPIDs := make([]string, len(instance.ElasticIPS))
		for i, eip := range instance.ElasticIPS {
			elasticIPIDs[i] = eip.ID.String()
		}

		if err := d.Set(AttrElasticIPIDs, elasticIPIDs); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrIPv6, utils.DefaultBool(v3.Ptr(instance.Ipv6Address != ""), false)); err != nil {
		return diag.FromErr(err)
	}

	if instance.Ipv6Address != "" {
		if err := d.Set(AttrIPv6Address, instance.Ipv6Address); err != nil {
			return diag.FromErr(err)
		}
	}

	if instance.MACAddress != "" {
		if err := d.Set(AttrMACAddress, instance.MACAddress); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrLabels, instance.Labels); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set(AttrName, instance.Name); err != nil {
		return diag.FromErr(err)
	}

	if len(instance.PrivateNetworks) > 0 {
		privateNetworkIDs := make([]string, len(instance.PrivateNetworks))
		networkInterfaces := make([]map[string]interface{}, len(instance.PrivateNetworks))

		for i, privnet := range instance.PrivateNetworks {
			privateNetwork, err := clientV3.GetPrivateNetwork(ctx, privnet.ID)
			if err != nil {
				return diag.FromErr(err)
			}

			var instanceAddress *string
			for _, lease := range privateNetwork.Leases {
				if lease.InstanceID.String() == instance.ID.String() {
					address := lease.IP.String()
					instanceAddress = &address
					break
				}
			}

			nif, err := NetworkInterface{privnet.ID.String(), instanceAddress, privnet.MACAddress}.ToInterface()
			if err != nil {
				return diag.FromErr(err)
			}

			networkInterfaces[i] = nif
			privateNetworkIDs[i] = privnet.ID.String()
		}
		if err := d.Set(AttrPrivateNetworkIDs, privateNetworkIDs); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set(AttrNetworkInterface, networkInterfaces); err != nil {
			return diag.FromErr(err)
		}
	}

	if instance.PublicIP != nil {
		if err := d.Set(AttrPublicIPAddress, instance.PublicIP.String()); err != nil {
			return diag.FromErr(err)
		}
	}

	// The API has a small quirk where it will populate both fiels regardless if
	// we use ssh_keys or the deprecated ssh_keys, so we add a small check to avoid
	// populating it in the plan if unset
	if instance.SSHKey != nil && d.Get(AttrSSHKey) != "" {
		if err := d.Set(AttrSSHKey, instance.SSHKey.Name); err != nil {
			return diag.FromErr(err)
		}
	} else if instance.SSHKeys != nil {
		keyNames := make([]string, len(instance.SSHKeys))
		for i, k := range instance.SSHKeys {
			keyNames[i] = k.Name
		}
		if err := d.Set(AttrSSHKeys, keyNames); err != nil {
			return diag.FromErr(err)
		}
	}

	if len(instance.SecurityGroups) > 0 {
		securityGroupIDs := make([]string, len(instance.SecurityGroups))
		for i, sg := range instance.SecurityGroups {
			securityGroupIDs[i] = sg.ID.String()
		}

		if err := d.Set(AttrSecurityGroupIDs, securityGroupIDs); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set(AttrState, instance.State); err != nil {
		return diag.FromErr(err)
	}

	if instance.Template != nil {
		if err := d.Set(AttrTemplateID, instance.Template.ID.String()); err != nil {
			return diag.FromErr(err)
		}
	}

	rdns, err := clientV3.GetReverseDNSInstance(ctx, instance.ID)
	if err != nil && !errors.Is(err, v3.ErrNotFound) {
		return diag.Errorf("unable to retrieve instance reverse-dns: %s", err)
	}
	rdnsAttr := ""
	if rdns != nil {
		rdnsAttr = strings.TrimSuffix(string(rdns.DomainName), ".")
	}
	if err := d.Set(AttrReverseDNS, rdnsAttr); err != nil {
		return diag.FromErr(err)
	}

	instanceTypes, err := clientV3.ListInstanceTypes(ctx)
	if err != nil {
		return diag.Errorf("unable to find instance type: %s", err)
	}

	instanceType, err := instanceTypes.FindInstanceType(instance.InstanceType.ID.String())
	if err != nil {
		return diag.Errorf("unable to find instance type: %s", err)
	}

	if err := d.Set(AttrType, fmt.Sprintf(
		"%s.%s",
		strings.ToLower(string(instanceType.Family)),
		strings.ToLower(string(instanceType.Size)),
	)); err != nil {
		return diag.FromErr(err)
	}

	if instance.UserData != "" {
		userData, err := utils.DecodeUserData(instance.UserData)
		if err != nil {
			return diag.Errorf("unable to decode user data: %s", err)
		}
		if err := d.Set(AttrUserData, userData); err != nil {
			return diag.FromErr(err)
		}
	}

	if instance.PublicIP != nil {
		// Connection info for the `ssh` remote-exec provisioner
		d.SetConnInfo(map[string]string{
			"type": "ssh",
			"host": instance.PublicIP.String(),
		})
	}

	return nil
}
