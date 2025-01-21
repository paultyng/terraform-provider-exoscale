package instance_pool

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

// DataSourceSchema returns a schema for a single instance pool data source.
func DataSourceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		AttrAntiAffinityGroupIDs: {
			Description: "The list of attached [exoscale_anti_affinity_group](../resources/anti_affinity_group.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrAffinityGroupIDs: {
			Description: "The list of attached [exoscale_anti_affinity_group](../resources/anti_affinity_group.md) (IDs). Use anti_affinity_group_ids instead.",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Deprecated:  "Use anti_affinity_group_ids instead.",
		},
		AttrDeployTargetID: {
			Description: "The deploy target ID.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrDescription: {
			Description: "The instance pool description.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrDiskSize: {
			Description: "The managed instances disk size.",
			Type:        schema.TypeInt,
			Computed:    true,
		},
		AttrElasticIPIDs: {
			Description: "The list of attached [exoscale_elastic_ip](../resources/elastic_ip.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrInstancePrefix: {
			Description: "The string used to prefix the managed instances name.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrInstanceType: {
			Description: "The managed instances type.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrIPv6: {
			Description: "Whether IPv6 is enabled on managed instances.",
			Type:        schema.TypeBool,
			Computed:    true,
		},
		AttrKeyPair: {
			Description: "The [exoscale_ssh_key](../resources/ssh_key.md) (name) authorized on the managed instances.",
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
			Description: "The pool name to match (conflicts with `id`).",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrID: {
			Description: "The instance pool ID to match (conflicts with `name`).",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrNetworkIDs: {
			Description: "The list of attached [exoscale_private_network](../resources/private_network.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrSecurityGroupIDs: {
			Description: "The list of attached [exoscale_security_group](../resources/security_group.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrSize: {
			Description: "The number managed instances.",
			Type:        schema.TypeInt,
			Computed:    true,
		},
		AttrMinAvailable: {
			Description: "Minimum number of running Instances.",
			Type:        schema.TypeInt,
			Computed:    true,
		},
		AttrState: {
			Description: "The pool state.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrTemplateID: {
			Description: "The managed instances [exoscale_template](./template.md) ID.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrUserData: {
			Description: "[cloud-init](http://cloudinit.readthedocs.io/en/latest/) configuration.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrInstances: {
			Description: "The list of managed instances. Structure is documented below.",
			Type:        schema.TypeSet,
			Computed:    true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					AttrInstanceID: {
						Description: "The compute instance ID.",
						Type:        schema.TypeString,
						Optional:    true,
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
		},
	}
}

func DataSource() *schema.Resource {
	return &schema.Resource{
		Description: `Fetch Exoscale [Instance Pools](https://community.exoscale.com/documentation/compute/instance-pools/) data.

Corresponding resource: [exoscale_instance_pool](../resources/instance_pool.md).`,
		Schema: func() map[string]*schema.Schema {
			schema := DataSourceSchema()

			// adding context-aware schema settings here so getDataSourceInstancePoolSchema can be used in list method
			schema[AttrID].ConflictsWith = []string{AttrName}
			schema[AttrName].ConflictsWith = []string{AttrID}
			return schema
		}(),
		ReadContext: dsRead,
	}
}

func dsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	id, byID := d.GetOk(AttrID)
	name, byName := d.GetOk(AttrName)
	if !byID && !byName {
		return diag.Errorf(
			"either %s or %s must be specified",
			AttrName,
			AttrID,
		)
	}

	var pool *v3.InstancePool
	if byID {
		pool, err = client.GetInstancePool(
			ctx,
			v3.UUID(id.(string)),
		)
	} else {
		rep, err := client.ListInstancePools(ctx)
		if err == nil {
			return diag.FromErr(err)
		}

		p, err := rep.FindInstancePool(name.(string))
		if err == nil {
			return diag.FromErr(err)
		}
		pool = &p
	}

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(pool.ID.String())

	data, err := dsBuildData(pool, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	instanceType, err := client.GetInstanceType(
		ctx,
		pool.InstanceType.ID,
	)
	if err != nil {
		return diag.Errorf("error retrieving instance type: %s", err)
	}

	data[AttrInstanceType] = fmt.Sprintf(
		"%s.%s",
		strings.ToLower(string(instanceType.Family)),
		strings.ToLower(string(instanceType.Size)),
	)

	if pool.Instances != nil {
		instancesData := make([]interface{}, len(pool.Instances))
		for k, i := range pool.Instances {
			instance, err := client.GetInstance(ctx, i.ID)
			if err != nil {
				return diag.FromErr(err)
			}

			var ipv6, publicIp string
			if instance.Ipv6Address != "" {
				ipv6 = instance.Ipv6Address
			}
			if instance.PublicIP.String() != "" {
				publicIp = instance.PublicIP.String()
			}

			instancesData[k] = map[string]interface{}{
				AttrInstanceID:              id,
				AttrInstanceIPv6Address:     ipv6,
				AttrInstanceName:            instance.Name,
				AttrInstancePublicIPAddress: publicIp,
			}
		}

		data[AttrInstances] = instancesData
	}

	for key, value := range data {
		err := d.Set(key, value)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	tflog.Debug(ctx, "read finished successfully", map[string]interface{}{
		"id": utils.IDString(d, Name),
	})

	return nil
}

// dsBuildData builds terraform data object from egoscale API struct.
func dsBuildData(pool *v3.InstancePool, zone string) (map[string]interface{}, error) {
	data := map[string]interface{}{}

	if pool.DeployTarget != nil {
		data[AttrDeployTargetID] = pool.DeployTarget.ID
	}
	data[AttrDescription] = utils.DefaultString(&pool.Description, "")
	data[AttrDiskSize] = pool.DiskSize
	data[AttrID] = pool.ID
	data[AttrInstancePrefix] = utils.DefaultString(&pool.InstancePrefix, "")
	data[AttrIPv6] = utils.DefaultBool(pool.Ipv6Enabled, false)
	if pool.SSHKey != nil {
		data[AttrKeyPair] = pool.SSHKey.Name
	}
	data[AttrName] = pool.Name
	data[AttrSize] = pool.Size
	data[AttrMinAvailable] = pool.MinAvailable
	data[AttrState] = pool.State
	data[AttrTemplateID] = pool.Template.ID
	data[AttrZone] = zone

	if pool.AntiAffinityGroups != nil {

		antiAffinityGroupIds := make([]string, len(pool.AntiAffinityGroups))

		for i, g := range pool.AntiAffinityGroups {
			antiAffinityGroupIds[i] = g.ID.String()
		}

		data[AttrAntiAffinityGroupIDs] = antiAffinityGroupIds
		data[AttrAffinityGroupIDs] = antiAffinityGroupIds // deprecated
	}

	if pool.Labels != nil {
		data[AttrLabels] = pool.Labels
	}

	if pool.ElasticIPS != nil {
		data[AttrElasticIPIDs] = utils.ElasticIPsToElasticIPIDs(pool.ElasticIPS)
	}

	if pool.PrivateNetworks != nil {
		data[AttrNetworkIDs] = utils.PrivateNetworksToPrivateNetworkIDs(pool.PrivateNetworks)
	}

	if pool.SecurityGroups != nil {
		data[AttrSecurityGroupIDs] = utils.SecurityGroupsToSecurityGroupIDs(pool.SecurityGroups)
	}

	if pool.UserData != "" {
		userData, err := utils.DecodeUserData(pool.UserData)
		if err != nil {
			return nil, fmt.Errorf("error decoding user data: %w", err)
		}
		data[AttrUserData] = userData
	}

	return data, nil
}
