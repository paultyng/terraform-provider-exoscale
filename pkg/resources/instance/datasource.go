package instance

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
	"github.com/exoscale/terraform-provider-exoscale/pkg/utils"
)

// DataSourceSchema returns a schema for a single Compute instance data source.
func DataSourceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		AttrAntiAffinityGroupIDs: {
			Description: "The list of attached [exoscale_anti_affinity_group](../resources/anti_affinity_group.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrCreatedAt: {
			Description: "The compute instance creation date.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrDeployTargetID: {
			Description: "A deploy target ID.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrDiskSize: {
			Description: "The instance disk size (GiB).",
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
		AttrID: {
			Description: "The compute instance ID to match (conflicts with `name`).",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrIPv6: {
			Description: "Whether IPv6 is enabled on the instance.",
			Type:        schema.TypeBool,
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
			Computed:    true,
		},
		AttrManagerID: {
			Description: "The instance manager ID, if any.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrManagerType: {
			Description: "The instance manager type (instance pool, SKS node pool, etc.), if any.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrName: {
			Description: "The instance name to match (conflicts with `id`).",
			Type:        schema.TypeString,
			Optional:    true,
		},
		AttrPrivateNetworkIDs: {
			Description: "The list of attached [exoscale_private_network](../resources/private_network.md) (IDs).",
			Type:        schema.TypeSet,
			Computed:    true,
			Set:         schema.HashString,
			Elem:        &schema.Schema{Type: schema.TypeString},
		},
		AttrPublicIPAddress: {
			Description: "The instance (main network interface) IPv4 address.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrReverseDNS: {
			Description: "Domain name for reverse DNS record.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrSSHKey: {
			Description: "The [exoscale_ssh_key](../resources/ssh_key.md) (name) authorized on the instance.",
			Type:        schema.TypeString,
			Computed:    true,
			Deprecated:  "Use ssh_keys instead",
		},
		AttrSSHKeys: {
			Description: "The list of [exoscale_ssh_key](../resources/ssh_key.md) (name) authorized on the instance.",
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
		AttrState: {
			Description: "The instance state.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrTemplateID: {
			Description: "The instance [exoscale_template](./template.md) ID.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrType: {
			Description: "The instance type.",
			Type:        schema.TypeString,
			Computed:    true,
		},
		AttrUserData: {
			Description: "The instance [cloud-init](http://cloudinit.readthedocs.io/en/latest/) configuration.",
			Type:        schema.TypeString,
			Computed:    true,
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
		Description: `Fetch Exoscale [Compute Instances](https://community.exoscale.com/documentation/compute/) data.

Corresponding resource: [exoscale_compute_instance](../resources/compute_instance.md).`,
		Schema: func() map[string]*schema.Schema {
			schema := DataSourceSchema()

			// adding context-aware schema settings here so getDataSourceComputeInstanceSchema can be used elsewhere
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

	instanceList, err := client.ListInstances(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	instanceListResp, err := instanceList.FindListInstancesResponseInstances(
		func() string {
			if byID {
				return id.(string)
			} else {
				return name.(string)
			}
		}(),
	)
	if err != nil {
		return diag.Errorf("unable to retrieve instance: %s", err)
	}

	instance, err := client.GetInstance(ctx, instanceListResp.ID)
	if err != nil {
		return diag.Errorf("unable to retrieve instance: %s", err)
	}

	d.SetId(string(instance.ID))

	data, err := dsBuildData(instance, zone)
	if err != nil {
		return diag.FromErr(err)
	}

	instanceType, err := client.GetInstanceType(
		ctx,
		instance.InstanceType.ID,
	)
	if err != nil {
		return diag.Errorf("unable to retrieve instance type: %s", err)
	}

	data[AttrType] = fmt.Sprintf(
		"%s.%s",
		strings.ToLower(string(instanceType.Family)),
		strings.ToLower(string(instanceType.Size)),
	)

	rdns, err := client.GetReverseDNSInstance(ctx, instance.ID)
	if err != nil && !errors.Is(err, v3.ErrNotFound) {
		return diag.Errorf("unable to retrieve instance reverse-dns: %s", err)
	}
	data[AttrReverseDNS] = strings.TrimSuffix(string(rdns.DomainName), ".")

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
func dsBuildData(instance *v3.Instance, zone string) (map[string]interface{}, error) {
	data := map[string]interface{}{}

	data[AttrDiskSize] = instance.DiskSize
	data[AttrID] = instance.ID
	data[AttrName] = instance.Name
	data[AttrState] = instance.State
	data[AttrZone] = zone

	data[AttrIPv6] = func() bool { return instance.PublicIPAssignment == v3.PublicIPAssignmentDual }()

	data[AttrDeployTargetID] = func() (s string) {
		if instance.DeployTarget != nil {
			s = instance.DeployTarget.ID.String()
		} else {
			s = ""
		}
		return
	}()

	if instance.SSHKey != nil {
		data[AttrSSHKey] = instance.SSHKey.Name
	}

	if instance.Template != nil {
		data[AttrTemplateID] = instance.Template.ID.String()
	}

	if instance.SSHKeys != nil {
		data[AttrSSHKeys] = func() []string {
			list := make([]string, len(instance.SSHKeys))
			for i, k := range instance.SSHKeys {
				list[i] = k.Name
			}
			return list
		}()
	}

	if instance.ElasticIPS != nil {
		data[AttrElasticIPIDs] = utils.ElasticIPsToElasticIPIDs(instance.ElasticIPS)
	}
	if instance.AntiAffinityGroups != nil {
		data[AttrAntiAffinityGroupIDs] = utils.AntiAffiniGroupsToAntiAffinityGroupIDs(instance.AntiAffinityGroups)
	}
	if instance.Labels != nil {
		data[AttrLabels] = instance.Labels
	}
	if instance.PrivateNetworks != nil {
		data[AttrPrivateNetworkIDs] = func() []string {
			list := make([]string, len(instance.PrivateNetworks))
			for i, pn := range instance.PrivateNetworks {
				list[i] = pn.ID.String()
			}
			return list
		}()
	}
	if instance.SecurityGroups != nil {
		data[AttrSecurityGroupIDs] = utils.SecurityGroupsToSecurityGroupIDs(instance.SecurityGroups)
	}

	if instance.Manager != nil {
		data[AttrManagerID] = instance.Manager.ID
		data[AttrManagerType] = instance.Manager.Type
	}

	data[AttrCreatedAt] = instance.CreatedAT.String()

	if instance.Ipv6Address != "" {
		data[AttrIPv6Address] = instance.Ipv6Address
	}

	if instance.PublicIP.String() != "" {
		data[AttrPublicIPAddress] = instance.PublicIP.String()
	}

	if instance.UserData != "" {
		userData, err := utils.DecodeUserData(instance.UserData)
		if err != nil {
			return nil, fmt.Errorf("unable to decode user data: %w", err)
		}
		data[AttrUserData] = userData
	}

	return data, nil
}
