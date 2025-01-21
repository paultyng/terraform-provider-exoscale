package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	exoapi "github.com/exoscale/egoscale/v2/api"
	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/general"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

const (
	defaultSKSNodepoolDiskSize       int64 = 50
	defaultSKSNodepoolInstancePrefix       = "pool"

	sksNodepoolAddonStorageLVM = "storage-lvm"

	resSKSNodepoolAttrAntiAffinityGroupIDs   = "anti_affinity_group_ids"
	resSKSNodepoolAttrClusterID              = "cluster_id"
	resSKSNodepoolAttrCreatedAt              = "created_at"
	resSKSNodepoolAttrDeployTargetID         = "deploy_target_id"
	resSKSNodepoolAttrDescription            = "description"
	resSKSNodepoolAttrDiskSize               = "disk_size"
	resSKSNodepoolAttrInstancePoolID         = "instance_pool_id"
	resSKSNodepoolAttrInstancePrefix         = "instance_prefix"
	resSKSNodepoolAttrInstanceType           = "instance_type"
	resSKSNodepoolAttrKubeletGC              = "kubelet_image_gc"
	resSKSNodepoolAttrKubeletGCMinAge        = "min_age"
	resSKSNodepoolAttrKubeletGCHighThreshold = "high_threshold"
	resSKSNodepoolAttrKubeletGCLowThreshold  = "low_threshold"
	resSKSNodepoolAttrLabels                 = "labels"
	resSKSNodepoolAttrID                     = "id"
	resSKSNodepoolAttrName                   = "name"
	resSKSNOdepoolAttrIPV6Enabled            = "ipv6"
	resSKSNodepoolAttrPrivateNetworkIDs      = "private_network_ids"
	resSKSNodepoolAttrSecurityGroupIDs       = "security_group_ids"
	resSKSNodepoolAttrSize                   = "size"
	resSKSNodepoolAttrState                  = "state"
	resSKSNodepoolAttrStorageLVM             = "storage_lvm"
	resSKSNodepoolAttrTaints                 = "taints"
	resSKSNodepoolAttrTemplateID             = "template_id"
	resSKSNodepoolAttrVersion                = "version"
	resSKSNodepoolAttrZone                   = "zone"
)

func resourceSKSNodepoolIDString(d general.ResourceIDStringer) string {
	return general.ResourceIDString(d, "exoscale_sks_nodepool")
}

func resourceSKSNodepool() *schema.Resource {
	s := map[string]*schema.Schema{
		resSKSNodepoolAttrAntiAffinityGroupIDs: {
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "A list of [exoscale_anti_affinity_group](./anti_affinity_group.md) (IDs) to be attached to the managed instances.",
		},
		resSKSNodepoolAttrClusterID: {
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "The parent [exoscale_sks_cluster](./sks_cluster.md) ID.",
		},
		resSKSNodepoolAttrCreatedAt: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The pool creation date.",
		},
		resSKSNodepoolAttrDeployTargetID: {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "A deploy target ID.",
		},
		resSKSNodepoolAttrDescription: {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "A free-form text describing the pool.",
		},
		resSKSNodepoolAttrDiskSize: {
			Type:        schema.TypeInt,
			Optional:    true,
			Default:     defaultSKSNodepoolDiskSize,
			Description: "The managed instances disk size (GiB; default: `50`).",
		},
		resSKSNodepoolAttrInstancePoolID: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The underlying [exoscale_instance_pool](./instance_pool.md) ID.",
		},
		resSKSNodepoolAttrInstancePrefix: {
			Type:        schema.TypeString,
			Optional:    true,
			Default:     defaultSKSNodepoolInstancePrefix,
			Description: "The string used to prefix the managed instances name (default `pool`).",
		},
		resSKSNodepoolAttrInstanceType: {
			Type:             schema.TypeString,
			Required:         true,
			ValidateDiagFunc: validateComputeInstanceType,
			// Ignore case differences
			DiffSuppressFunc: suppressCaseDiff,
			Description:      "The managed compute instances type (`<family>.<size>`, e.g. `standard.medium`; use the [Exoscale CLI](https://github.com/exoscale/cli/) - `exo compute instance-type list` - for the list of available types).",
		},
		resSKSNodepoolAttrKubeletGC: {
			Type:        schema.TypeSet,
			Optional:    true,
			Description: "Configuration for this nodepool's kubelet image garbage collector",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					resSKSNodepoolAttrKubeletGCMinAge: {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "The minimum age for an unused image before it is garbage collected (k8s duration format, eg. 1h)",
					},
					resSKSNodepoolAttrKubeletGCHighThreshold: {
						Type:        schema.TypeInt,
						Optional:    true,
						Description: "The percent of disk usage after which image garbage collection is always run",
					},
					resSKSNodepoolAttrKubeletGCLowThreshold: {
						Type:        schema.TypeInt,
						Optional:    true,
						Description: "The percent of disk usage before which image garbage collection is never run",
					},
				},
			},
		},
		resSKSNodepoolAttrLabels: {
			Type:        schema.TypeMap,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Optional:    true,
			Description: "A map of key/value labels.",
		},
		resSKSNodepoolAttrID: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The SKS node pool ID.",
		},
		resSKSNodepoolAttrName: {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The SKS node pool name.",
		},
		resSKSNOdepoolAttrIPV6Enabled: {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
			Description: "Enable IPV6 for the nodepool nodes",
		},
		resSKSNodepoolAttrPrivateNetworkIDs: {
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "A list of [exoscale_private_network](./private_network.md) (IDs) to be attached to the managed instances.",
		},
		resSKSNodepoolAttrSecurityGroupIDs: {
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "A list of [exoscale_security_group](./security_group.md) (IDs) to be attached to the managed instances.",
		},
		resSKSNodepoolAttrSize: {
			Type:     schema.TypeInt,
			Required: true,
		},
		resSKSNodepoolAttrState: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The current pool state.",
		},
		resSKSNodepoolAttrStorageLVM: {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
			Description: "Create nodes with non-standard partitioning for persistent storage (requires min 100G of disk space) (may only be set at creation time).",
		},
		resSKSNodepoolAttrTaints: {
			Type:        schema.TypeMap,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Optional:    true,
			Description: `A map of key/value Kubernetes [taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) ('taints = { <key> = "<value>:<effect>" }').`,
		},
		resSKSNodepoolAttrTemplateID: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The managed instances template ID.",
		},
		resSKSNodepoolAttrVersion: {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The managed instances version.",
		},
		resSKSNodepoolAttrZone: {
			Type:        schema.TypeString,
			Required:    true,
			ForceNew:    true,
			Description: "The Exoscale [Zone](https://www.exoscale.com/datacenters/) name.",
		},
	}

	return &schema.Resource{
		Schema: s,

		Description: "Manage Exoscale Scalable Kubernetes Service (SKS) Node Pools.",

		CreateContext: resourceSKSNodepoolCreate,
		ReadContext:   resourceSKSNodepoolRead,
		UpdateContext: resourceSKSNodepoolUpdate,
		DeleteContext: resourceSKSNodepoolDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
				zonedRes, err := zonedStateContextFunc(ctx, d, nil)
				if err != nil {
					return nil, err
				}
				d = zonedRes[0]

				parts := strings.SplitN(d.Id(), "/", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf(`invalid ID %q, expected format "<CLUSTER-ID>/<NODEPOOL-ID>@<ZONE>"`, d.Id())
				}

				d.SetId(parts[1])
				if err := d.Set(resSKSNodepoolAttrClusterID, parts[0]); err != nil {
					return nil, err
				}

				return []*schema.ResourceData{d}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(config.DefaultTimeout),
			Read:   schema.DefaultTimeout(config.DefaultTimeout),
			Update: schema.DefaultTimeout(config.DefaultTimeout),
			Delete: schema.DefaultTimeout(config.DefaultTimeout),
		},
	}
}

func resourceSKSNodepoolCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning create", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	zone := d.Get(resSKSNodepoolAttrZone).(string)

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

	sksCluster, err := client.GetSKSCluster(ctx, v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string)))
	if err != nil {
		return diag.FromErr(err)
	}

	sksNodepoolCreate := new(v3.CreateSKSNodepoolRequest)

	if set, ok := d.Get(resSKSNodepoolAttrAntiAffinityGroupIDs).(*schema.Set); ok {
		sksNodepoolCreate.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDeployTargetID); ok {
		s := v.(string)
		sksNodepoolCreate.DeployTarget = &v3.DeployTarget{ID: v3.UUID(s)}
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDescription); ok {
		s := v.(string)
		sksNodepoolCreate.Description = s
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDiskSize); ok {
		i := int64(v.(int))
		sksNodepoolCreate.DiskSize = i
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrInstancePrefix); ok {
		s := v.(string)
		sksNodepoolCreate.InstancePrefix = s
	}

	if v, ok := d.GetOk(resSKSNOdepoolAttrIPV6Enabled); ok {
		b := v.(bool)
		if b {
			sksNodepoolCreate.PublicIPAssignment = v3.CreateSKSNodepoolRequestPublicIPAssignmentDual
		} else {
			sksNodepoolCreate.PublicIPAssignment = v3.CreateSKSNodepoolRequestPublicIPAssignmentInet4
		}
	}

	instanceType, err := utils.FindInstanceTypeByNameV3(ctx, client, d.Get(resSKSNodepoolAttrInstanceType).(string))
	if err != nil {
		return diag.Errorf("error retrieving instance type: %s", err)
	}
	sksNodepoolCreate.InstanceType = &v3.InstanceType{
		ID: instanceType.ID,
	}

	if k, ok := d.GetOk(resSKSNodepoolAttrKubeletGC); ok {
		kubeletGc := k.(*schema.Set).List()[0].(map[string]interface{})
		sksNodepoolKubeletGc := new(v3.KubeletImageGC)

		if val, ok := kubeletGc[resSKSNodepoolAttrKubeletGCMinAge]; ok {
			sksNodepoolKubeletGcMinAge := val.(string)
			sksNodepoolKubeletGc.MinAge = sksNodepoolKubeletGcMinAge
		}

		if val, ok := kubeletGc[resSKSNodepoolAttrKubeletGCLowThreshold]; ok {
			sksNodepoolKubeletGcLowThreshold := val.(int)
			sksNodepoolKubeletGcLowThresholdInt64 := int64(sksNodepoolKubeletGcLowThreshold)
			sksNodepoolKubeletGc.LowThreshold = sksNodepoolKubeletGcLowThresholdInt64

		}

		if val, ok := kubeletGc[resSKSNodepoolAttrKubeletGCHighThreshold]; ok {
			sksNodepoolKubeletGcHighThreshold := val.(int)
			sksNodepoolKubeletGcHighThresholdInt64 := int64(sksNodepoolKubeletGcHighThreshold)
			sksNodepoolKubeletGc.HighThreshold = sksNodepoolKubeletGcHighThresholdInt64

		}

		sksNodepoolCreate.KubeletImageGC = sksNodepoolKubeletGc
	}

	if l, ok := d.GetOk(resSKSNodepoolAttrLabels); ok {
		labels := make(map[string]string)
		for k, v := range l.(map[string]interface{}) {
			labels[k] = v.(string)
		}
		sksNodepoolCreate.Labels = labels
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrName); ok {
		s := v.(string)
		sksNodepoolCreate.Name = s
	}

	if set, ok := d.Get(resSKSNodepoolAttrPrivateNetworkIDs).(*schema.Set); ok {
		sksNodepoolCreate.PrivateNetworks = utils.PrivateNetworkIDsToPrivateNetworks(set.List())
	}

	if set, ok := d.Get(resSKSNodepoolAttrSecurityGroupIDs).(*schema.Set); ok {
		sksNodepoolCreate.SecurityGroups = utils.SecurityGroupIDsToSecurityGroups(set.List())
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrSize); ok {
		i := int64(v.(int))
		sksNodepoolCreate.Size = i
	}

	var addOns []string
	if enableStorageLVM := d.Get(resSKSNodepoolAttrStorageLVM).(bool); enableStorageLVM {
		addOns = append(addOns, sksNodepoolAddonStorageLVM)
	}
	if len(addOns) > 0 {
		sksNodepoolCreate.Addons = addOns
	}

	if t, ok := d.GetOk(resSKSNodepoolAttrTaints); ok {
		taints := make(v3.SKSNodepoolTaints)
		for k, v := range t.(map[string]interface{}) {
			taint, err := parseSKSNodepoolTaintV3(v.(string))
			if err != nil {
				return diag.Errorf("invalid taint %q: %s", v.(string), err)
			}
			taints[k] = *taint
		}
		sksNodepoolCreate.Taints = taints
	}

	op, err := client.CreateSKSNodepool(ctx, sksCluster.ID, *sksNodepoolCreate)
	if err != nil {
		return diag.FromErr(err)
	}
	if op, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
		return diag.FromErr(err)
	}

	sksNodepoolID := op.Reference.ID

	d.SetId(sksNodepoolID.String())

	tflog.Debug(ctx, "create finished successfully", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	return resourceSKSNodepoolRead(ctx, d, meta)
}

func resourceSKSNodepoolRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning read", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	zone := d.Get(resSKSNodepoolAttrZone).(string)

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

	sks, err := client.GetSKSCluster(ctx, v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string)))
	if err != nil {
		if errors.Is(err, exoapi.ErrNotFound) {
			// Parent SKS cluster doesn't exist anymore, so does the SKS Nodepool.
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	var sksNodepool *v3.SKSNodepool
	for _, np := range sks.Nodepools {
		if np.ID == v3.UUID(d.Id()) {
			sksNodepool = &np
			break
		}
	}
	if sksNodepool == nil {
		// Resource doesn't exist anymore, signaling the core to remove it from the state.
		d.SetId("")
		return nil
	}

	tflog.Debug(ctx, "read finished successfully", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	return diag.FromErr(resourceSKSNodepoolApply(ctx, client, d, sksNodepool))
}

func resourceSKSNodepoolUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning update", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	zone := d.Get(resSKSNodepoolAttrZone).(string)

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

	sksCluster, err := client.GetSKSCluster(ctx, v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string)))
	if err != nil {
		return diag.FromErr(err)
	}

	var sksNodepoolUpdate *v3.UpdateSKSNodepoolRequest
	var sksNp *v3.SKSNodepool
	for _, np := range sksCluster.Nodepools {
		if np.ID == v3.UUID(d.Id()) {
			sksNp = &np
			break
		}
	}
	if sksNp == nil {
		return diag.Errorf("SKS Nodepool %q not found", d.Id())
	}

	sksNodepoolUpdate = &v3.UpdateSKSNodepoolRequest{
		AntiAffinityGroups: sksNp.AntiAffinityGroups,
		DeployTarget:       sksNp.DeployTarget,
		Description:        sksNp.Description,
		DiskSize:           sksNp.DiskSize,
		InstancePrefix:     sksNp.InstancePrefix,
		InstanceType:       sksNp.InstanceType,
		Labels:             sksNp.Labels,
		Name:               sksNp.Name,
		PrivateNetworks:    sksNp.PrivateNetworks,
		PublicIPAssignment: v3.UpdateSKSNodepoolRequestPublicIPAssignment(sksNp.PublicIPAssignment),
		SecurityGroups:     sksNp.SecurityGroups,
		Taints:             sksNp.Taints,
	}

	var updated bool

	if d.HasChange(resSKSNodepoolAttrAntiAffinityGroupIDs) {
		set := d.Get(resSKSNodepoolAttrAntiAffinityGroupIDs).(*schema.Set)
		sksNodepoolUpdate.AntiAffinityGroups = utils.AntiAffinityGroupIDsToAntiAffinityGroups(set.List())
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDeployTargetID) {
		v := d.Get(resSKSNodepoolAttrDeployTargetID).(string)
		sksNodepoolUpdate.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(v),
		}
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDescription) {
		v := d.Get(resSKSNodepoolAttrDescription).(string)
		sksNodepoolUpdate.Description = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDiskSize) {
		v := int64(d.Get(resSKSNodepoolAttrDiskSize).(int))
		sksNodepoolUpdate.DiskSize = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrInstancePrefix) {
		v := d.Get(resSKSNodepoolAttrInstancePrefix).(string)
		sksNodepoolUpdate.InstancePrefix = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrInstanceType) {
		instanceType, err := utils.FindInstanceTypeByNameV3(ctx, client, d.Get(resSKSNodepoolAttrInstanceType).(string))
		if err != nil {
			return diag.Errorf("error retrieving instance type: %s", err)
		}
		sksNodepoolUpdate.InstanceType = instanceType
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrLabels) {
		labels := make(map[string]string)
		for k, v := range d.Get(resSKSNodepoolAttrLabels).(map[string]interface{}) {
			labels[k] = v.(string)
		}
		sksNodepoolUpdate.Labels = labels
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrName) {
		v := d.Get(resSKSNodepoolAttrName).(string)
		sksNodepoolUpdate.Name = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrPrivateNetworkIDs) {
		set := d.Get(resSKSNodepoolAttrPrivateNetworkIDs).(*schema.Set)
		sksNodepoolUpdate.PrivateNetworks = utils.PrivateNetworkIDsToPrivateNetworks(set.List())
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrSecurityGroupIDs) {
		set := d.Get(resSKSNodepoolAttrSecurityGroupIDs).(*schema.Set)
		sksNodepoolUpdate.SecurityGroups = utils.SecurityGroupIDsToSecurityGroups(set.List())
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrTaints) {
		taints := make(v3.SKSNodepoolTaints)
		for k, v := range d.Get(resSKSNodepoolAttrTaints).(map[string]interface{}) {
			taint, err := parseSKSNodepoolTaintV3(v.(string))
			if err != nil {
				return diag.Errorf("invalid taint %q: %s", v.(string), err)
			}
			taints[k] = *taint
		}
		sksNodepoolUpdate.Taints = taints
		updated = true
	}

	if d.HasChange(resSKSNOdepoolAttrIPV6Enabled) {
		v := d.Get(resSKSNOdepoolAttrIPV6Enabled).(bool)
		if v {
			sksNodepoolUpdate.PublicIPAssignment = v3.UpdateSKSNodepoolRequestPublicIPAssignmentDual
		} else {
			sksNodepoolUpdate.PublicIPAssignment = v3.UpdateSKSNodepoolRequestPublicIPAssignmentInet4
		}
	}

	if updated {
		op, err := client.UpdateSKSNodepool(ctx, sksCluster.ID, sksNp.ID, *sksNodepoolUpdate)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, err := client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange(resSKSNodepoolAttrSize) {
		op, err := client.ScaleSKSNodepool(
			ctx,
			sksCluster.ID,
			sksNp.ID,
			v3.ScaleSKSNodepoolRequest{
				Size: int64(d.Get(resSKSNodepoolAttrSize).(int)),
			},
		)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, err = client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
			return diag.FromErr(err)
		}
	}

	tflog.Debug(ctx, "update finished successfully", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	return resourceSKSNodepoolRead(ctx, d, meta)
}

func resourceSKSNodepoolDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning delete", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	zone := d.Get(resSKSNodepoolAttrZone).(string)

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

	sksCluster, err := client.GetSKSCluster(ctx, v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string)))
	if err != nil {
		return diag.FromErr(err)
	}

	op, err := client.DeleteSKSNodepool(ctx, sksCluster.ID, v3.UUID(d.Id()))
	if err != nil {
		return diag.FromErr(err)
	}
	if _, err := client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
		return diag.FromErr(err)
	}

	tflog.Debug(ctx, "delete finished successfully", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	return nil
}

func resourceSKSNodepoolApply(
	ctx context.Context,
	client *v3.Client,
	d *schema.ResourceData,
	sksNodepool *v3.SKSNodepool,
) error {
	if sksNodepool.AntiAffinityGroups != nil {
		aags := utils.AntiAffiniGroupsToAntiAffinityGroupIDs(sksNodepool.AntiAffinityGroups)
		if err := d.Set(resSKSNodepoolAttrAntiAffinityGroupIDs, aags); err != nil {
			return err
		}
	}

	if sksNodepool.Addons != nil {
		if err := d.Set(resSKSNodepoolAttrStorageLVM, in(sksNodepool.Addons, sksNodepoolAddonStorageLVM)); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrCreatedAt, sksNodepool.CreatedAT.String()); err != nil {
		return err
	}

	if sksNodepool.DeployTarget != nil {
		if err := d.Set(resSKSNodepoolAttrDeployTargetID, sksNodepool.DeployTarget.ID.String()); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrDescription, sksNodepool.Description); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrDiskSize, sksNodepool.DiskSize); err != nil {
		return err
	}

	if sksNodepool.InstancePool != nil {
		if err := d.Set(resSKSNodepoolAttrInstancePoolID, sksNodepool.InstancePool.ID.String()); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrInstancePrefix, sksNodepool.InstancePrefix); err != nil {
		return err
	}

	instanceType, err := client.GetInstanceType(
		ctx,
		sksNodepool.InstanceType.ID,
	)
	if err != nil {
		return fmt.Errorf("error retrieving instance type: %w", err)
	}
	if err := d.Set(resSKSNodepoolAttrInstanceType, fmt.Sprintf(
		"%s.%s",
		strings.ToLower(string(instanceType.Family)),
		strings.ToLower(string(instanceType.Size)),
	)); err != nil {
		return err
	}

	if sksNodepool.KubeletImageGC != nil {
		kubeletGc := d.Get(resSKSNodepoolAttrKubeletGC).(*schema.Set)
		if err := d.Set(resSKSNodepoolAttrKubeletGC, schema.NewSet(kubeletGc.F, []interface{}{
			func() map[string]interface{} {
				i := map[string]interface{}{}
				if sksNodepool.KubeletImageGC.MinAge != "" {
					i[resSKSNodepoolAttrKubeletGCMinAge] = sksNodepool.KubeletImageGC.MinAge
				}
				if sksNodepool.KubeletImageGC.HighThreshold != 0 {
					i[resSKSNodepoolAttrKubeletGCHighThreshold] = int(sksNodepool.KubeletImageGC.HighThreshold)
				}
				if sksNodepool.KubeletImageGC.LowThreshold != 0 {
					i[resSKSNodepoolAttrKubeletGCLowThreshold] = int(sksNodepool.KubeletImageGC.LowThreshold)
				}
				return i
			}(),
		})); err != nil {
			return err
		}

	}

	if err := d.Set(resSKSNodepoolAttrLabels, sksNodepool.Labels); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrName, sksNodepool.Name); err != nil {
		return err
	}

	if sksNodepool.PrivateNetworks != nil {
		privnets := utils.PrivateNetworksToPrivateNetworkIDs(sksNodepool.PrivateNetworks)
		if err := d.Set(resSKSNodepoolAttrPrivateNetworkIDs, privnets); err != nil {
			return err
		}
	}

	if sksNodepool.SecurityGroups != nil {
		sgs := utils.SecurityGroupsToSecurityGroupIDs(sksNodepool.SecurityGroups)
		if err := d.Set(resSKSNodepoolAttrSecurityGroupIDs, sgs); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrSize, sksNodepool.Size); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrState, sksNodepool.State); err != nil {
		return err
	}

	if sksNodepool.Taints != nil {
		taints := make(map[string]string)
		for k, v := range sksNodepool.Taints {
			taints[k] = fmt.Sprintf("%s:%s", v.Value, v.Effect)
		}
		if err := d.Set(resSKSNodepoolAttrTaints, taints); err != nil {
			return err
		}
	}

	if sksNodepool.Template != nil {
		if err := d.Set(resSKSNodepoolAttrTemplateID, sksNodepool.Template.ID); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrVersion, sksNodepool.Version); err != nil {
		return err
	}

	if sksNodepool.PublicIPAssignment == v3.SKSNodepoolPublicIPAssignmentDual {
		if err := d.Set(resSKSNOdepoolAttrIPV6Enabled, true); err != nil {
			return err
		}
	} else {
		if err := d.Set(resSKSNOdepoolAttrIPV6Enabled, false); err != nil {
			return err
		}
	}

	return nil
}

// parseSKSNodepoolTaint parses a CLI-formatted Kubernetes Node taint
// description formatted as VALUE:EFFECT, and returns discrete values
// for the value/effect as v3.SKSNodepoolTaint, or an error if
// the input value parsing failed.
func parseSKSNodepoolTaintV3(v string) (*v3.SKSNodepoolTaint, error) {

	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("expected format VALUE:EFFECT")
	}
	taintValue, taintEffect := parts[0], parts[1]

	if taintValue == "" || taintEffect == "" {
		return nil, errors.New("expected format VALUE:EFFECT")
	}

	return &v3.SKSNodepoolTaint{
		Effect: v3.SKSNodepoolTaintEffect(taintEffect),
		Value:  taintValue,
	}, nil
}
