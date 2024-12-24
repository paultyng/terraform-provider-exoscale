package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/general"
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
	resSKSNodepoolAttrPrivateNetworkIDs      = "private_network_ids"
	resSKSNodepoolAttrPublicIPAssignment     = "public_ip_assignment"
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
		resSKSNodepoolAttrPrivateNetworkIDs: {
			Type:        schema.TypeSet,
			Optional:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "A list of [exoscale_private_network](./private_network.md) (IDs) to be attached to the managed instances.",
		},
		resSKSNodepoolAttrPublicIPAssignment: {
			Type:     schema.TypeString,
			Optional: true,
			Description: `Configures public IP assignment of the Instances with:
    * IPv4 ('inet4') addressing only (default);
    * both IPv4 and IPv6 ('dual') addressing.`,
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

func getAAG(set *schema.Set) (v []v3.AntiAffinityGroup) {
	if l := set.Len(); l > 0 {
		list := make([]v3.AntiAffinityGroup, l)
		for i, v := range set.List() {
			list[i].ID = v3.UUID(v.(string))
		}
		v = list
	}
	return
}

func getPNs(set *schema.Set) (v []v3.PrivateNetwork) {
	if l := set.Len(); l > 0 {
		list := make([]v3.PrivateNetwork, l)
		for i, v := range set.List() {
			list[i].ID = v3.UUID(v.(string))
		}
		v = list
	}
	return
}

func getSGs(set *schema.Set) (v []v3.SecurityGroup) {
	if l := set.Len(); l > 0 {
		list := make([]v3.SecurityGroup, l)
		for i, v := range set.List() {
			list[i].ID = v3.UUID(v.(string))
		}
		v = list
	}
	return
}

func getInstanceType(ctx context.Context, client *v3.Client, instanceTypeName string) (*v3.InstanceType, error) {
	parts := strings.Split(instanceTypeName, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid instance type: %q", instanceTypeName)
	}

	family := parts[0]
	size := parts[1]

	instanceTypes, err := client.ListInstanceTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving instance types: %s", err)
	}

	for _, it := range instanceTypes.InstanceTypes {
		if it.Family == v3.InstanceTypeFamily(family) && it.Size == v3.InstanceTypeSize(size) {
			return client.GetInstanceType(ctx, it.ID)
		}
	}

	return nil, fmt.Errorf("instance type %q not found", instanceTypeName)
}

func resourceSKSNodepoolCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	tflog.Debug(ctx, "beginning create", map[string]interface{}{
		"id": resourceSKSNodepoolIDString(d),
	})

	zone := d.Get(resSKSNodepoolAttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutCreate))
	defer cancel()

	client, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	clusterID := v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string))

	sksNodepoolReq := v3.CreateSKSNodepoolRequest{}

	if set, ok := d.Get(resSKSNodepoolAttrAntiAffinityGroupIDs).(*schema.Set); ok {
		sksNodepoolReq.AntiAffinityGroups = getAAG(set)
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDeployTargetID); ok {
		s := v.(string)
		sksNodepoolReq.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(s),
		}
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDescription); ok {
		s := v.(string)
		sksNodepoolReq.Description = s
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrDiskSize); ok {
		i := int64(v.(int))
		sksNodepoolReq.DiskSize = i
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrInstancePrefix); ok {
		s := v.(string)
		sksNodepoolReq.InstancePrefix = s
	}

	instanceTypeName := d.Get(resSKSNodepoolAttrInstanceType).(string)
	instanceType, err := getInstanceType(ctx, client, instanceTypeName)
	if err != nil {
		return diag.Errorf("failed to find instance type: %s", err)
	}

	sksNodepoolReq.InstanceType = instanceType

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

		sksNodepoolReq.KubeletImageGC = sksNodepoolKubeletGc
	}

	if l, ok := d.GetOk(resSKSNodepoolAttrLabels); ok {
		labels := make(map[string]string)
		for k, v := range l.(map[string]interface{}) {
			labels[k] = v.(string)
		}
		sksNodepoolReq.Labels = labels
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrName); ok {
		s := v.(string)
		sksNodepoolReq.Name = s
	}

	if set, ok := d.Get(resSKSNodepoolAttrPublicIPAssignment).(string); ok {
		sksNodepoolReq.PublicIPAssignment = v3.CreateSKSNodepoolRequestPublicIPAssignment(set)
	}

	if set, ok := d.Get(resSKSNodepoolAttrPrivateNetworkIDs).(*schema.Set); ok {
		sksNodepoolReq.PrivateNetworks = getPNs(set)
	}

	if set, ok := d.Get(resSKSNodepoolAttrSecurityGroupIDs).(*schema.Set); ok {
		sksNodepoolReq.SecurityGroups = getSGs(set)
	}

	if v, ok := d.GetOk(resSKSNodepoolAttrSize); ok {
		i := int64(v.(int))
		sksNodepoolReq.Size = i
	}

	var addOns []string
	if enableStorageLVM := d.Get(resSKSNodepoolAttrStorageLVM).(bool); enableStorageLVM {
		addOns = append(addOns, sksNodepoolAddonStorageLVM)
	}
	if len(addOns) > 0 {
		sksNodepoolReq.Addons = addOns
	}

	if t, ok := d.GetOk(resSKSNodepoolAttrTaints); ok {
		taints := make(v3.SKSNodepoolTaints)
		for k, v := range t.(map[string]interface{}) {
			taint, err := parseSKSNodepoolTaint(v.(string))
			if err != nil {
				return diag.Errorf("invalid taint %q: %s", v.(string), err)
			}
			taints[k] = *taint
		}
		sksNodepoolReq.Taints = taints
	}

	op, err := client.CreateSKSNodepool(ctx, clusterID, sksNodepoolReq)
	if err != nil {
		return diag.FromErr(err)
	}

	opSuccess, err := client.Wait(ctx, op, v3.OperationStateSuccess)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(opSuccess.Reference.ID.String())

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

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutRead))
	defer cancel()

	client, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	clusterID := v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string))
	sks, err := client.GetSKSCluster(ctx, clusterID)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
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

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutUpdate))
	defer cancel()

	client, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	nodepoolID := v3.UUID(d.Id())

	clusterID := v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string))
	sksCluster, err := client.GetSKSCluster(ctx, clusterID)
	if err != nil {
		return diag.FromErr(err)
	}

	found := false
	for _, np := range sksCluster.Nodepools {
		if np.ID == nodepoolID {
			found = true
			break
		}
	}
	if !found {
		return diag.Errorf("SKS Nodepool %q not found in cluster %q", nodepoolID, clusterID)
	}

	sksNodepoolReq := v3.UpdateSKSNodepoolRequest{}

	var updated bool

	if d.HasChange(resSKSNodepoolAttrAntiAffinityGroupIDs) {
		set := d.Get(resSKSNodepoolAttrAntiAffinityGroupIDs).(*schema.Set)
		sksNodepoolReq.AntiAffinityGroups = getAAG(set)
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDeployTargetID) {
		v := d.Get(resSKSNodepoolAttrDeployTargetID).(string)
		sksNodepoolReq.DeployTarget = &v3.DeployTarget{
			ID: v3.UUID(v),
		}
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDescription) {
		v := d.Get(resSKSNodepoolAttrDescription).(string)
		sksNodepoolReq.Description = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrDiskSize) {
		v := int64(d.Get(resSKSNodepoolAttrDiskSize).(int))
		sksNodepoolReq.DiskSize = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrInstancePrefix) {
		v := d.Get(resSKSNodepoolAttrInstancePrefix).(string)
		sksNodepoolReq.InstancePrefix = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrInstanceType) {
		instanceTypeName := d.Get(resSKSNodepoolAttrInstanceType).(string)
		instanceType, err := getInstanceType(ctx, client, instanceTypeName)
		if err != nil {
			return diag.Errorf("failed to find instance type: %s", err)
		}

		sksNodepoolReq.InstanceType = instanceType
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrLabels) {
		labels := make(v3.Labels)
		for k, v := range d.Get(resSKSNodepoolAttrLabels).(map[string]interface{}) {
			labels[k] = v.(string)
		}
		sksNodepoolReq.Labels = labels
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrName) {
		v := d.Get(resSKSNodepoolAttrName).(string)
		sksNodepoolReq.Name = v
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrPublicIPAssignment) {
		set := d.Get(resSKSNodepoolAttrPublicIPAssignment).(string)
		sksNodepoolReq.PublicIPAssignment = v3.UpdateSKSNodepoolRequestPublicIPAssignment(set)
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrPrivateNetworkIDs) {
		set := d.Get(resSKSNodepoolAttrPrivateNetworkIDs).(*schema.Set)
		sksNodepoolReq.PrivateNetworks = getPNs(set)
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrSecurityGroupIDs) {
		set := d.Get(resSKSNodepoolAttrSecurityGroupIDs).(*schema.Set)
		sksNodepoolReq.SecurityGroups = getSGs(set)
		updated = true
	}

	if d.HasChange(resSKSNodepoolAttrTaints) {
		taints := make(v3.SKSNodepoolTaints)
		for k, v := range d.Get(resSKSNodepoolAttrTaints).(map[string]interface{}) {
			taint, err := parseSKSNodepoolTaint(v.(string))
			if err != nil {
				return diag.Errorf("invalid taint %q: %s", v.(string), err)
			}
			taints[k] = *taint
		}
		sksNodepoolReq.Taints = taints
		updated = true
	}

	if updated {
		err := await(ctx, client)(client.UpdateSKSNodepool(ctx, clusterID, nodepoolID, sksNodepoolReq))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange(resSKSNodepoolAttrSize) {
		if err := await(ctx, client)(client.ScaleSKSNodepool(ctx, clusterID, nodepoolID, v3.ScaleSKSNodepoolRequest{
			Size: int64(d.Get(resSKSNodepoolAttrSize).(int)),
		})); err != nil {
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

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutDelete))
	defer cancel()

	client, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	clusterID := v3.UUID(d.Get(resSKSNodepoolAttrClusterID).(string))
	_, err = client.GetSKSCluster(ctx, clusterID)
	if err != nil {
		return diag.FromErr(err)
	}

	sksNodepoolID := v3.UUID(d.Id())
	if err := await(ctx, client)(client.DeleteSKSNodepool(ctx, clusterID, sksNodepoolID)); err != nil {
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
	if len(sksNodepool.AntiAffinityGroups) > 0 {
		antiAffinityGroupIDs := make([]string, 0, len(sksNodepool.AntiAffinityGroups))
		for _, aag := range sksNodepool.AntiAffinityGroups {
			antiAffinityGroupIDs = append(antiAffinityGroupIDs, aag.ID.String())
		}
		if err := d.Set(resSKSNodepoolAttrAntiAffinityGroupIDs, antiAffinityGroupIDs); err != nil {
			return err
		}
	}

	if len(sksNodepool.Addons) > 0 {
		if err := d.Set(resSKSNodepoolAttrStorageLVM, in(sksNodepool.Addons, sksNodepoolAddonStorageLVM)); err != nil {
			return err
		}
	}

	if err := d.Set(resSKSNodepoolAttrCreatedAt, sksNodepool.CreatedAT.String()); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrDeployTargetID, sksNodepool.DeployTarget.ID); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrDescription, sksNodepool.Description); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrDiskSize, sksNodepool.DiskSize); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrInstancePoolID, sksNodepool.InstancePool.ID); err != nil {
		return err
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

	if err := d.Set(resSKSNodepoolAttrLabels, sksNodepool.Labels); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrName, sksNodepool.Name); err != nil {
		return err
	}

	if len(sksNodepool.PrivateNetworks) > 0 {
		privateNetworkIDs := make([]string, 0, len(sksNodepool.PrivateNetworks))
		for _, pn := range sksNodepool.PrivateNetworks {
			privateNetworkIDs = append(privateNetworkIDs, pn.ID.String())
		}
		if err := d.Set(resSKSNodepoolAttrPrivateNetworkIDs, privateNetworkIDs); err != nil {
			return err
		}
	}

	if len(sksNodepool.SecurityGroups) > 0 {
		securityGroupIDs := make([]string, 0, len(sksNodepool.SecurityGroups))
		for _, sg := range sksNodepool.SecurityGroups {
			securityGroupIDs = append(securityGroupIDs, sg.ID.String())
		}
		if err := d.Set(resSKSNodepoolAttrSecurityGroupIDs, securityGroupIDs); err != nil {
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

	if err := d.Set(resSKSNodepoolAttrTemplateID, sksNodepool.Template.ID); err != nil {
		return err
	}

	if err := d.Set(resSKSNodepoolAttrVersion, sksNodepool.Version); err != nil {
		return err
	}

	return nil
}

// parseSKSNodepoolTaint parses a CLI-formatted Kubernetes Node taint
// description formatted as VALUE:EFFECT, and returns discrete values
// for the value/effect as egoscale.SKSNodepoolTaint, or an error if
// the input value parsing failed.
func parseSKSNodepoolTaint(v string) (*v3.SKSNodepoolTaint, error) {
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
