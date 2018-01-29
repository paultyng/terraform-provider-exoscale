package exoscale

import (
	"fmt"
	"net"

	"github.com/exoscale/egoscale"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
)

func networkResource() *schema.Resource {
	return &schema.Resource{
		Create: createNetwork,
		Exists: existsNetwork,
		Read:   readNetwork,
		Update: updateNetwork,
		Delete: deleteNetwork,

		Importer: &schema.ResourceImporter{
			State: importNetwork,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"display_text": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"network_offering": {
				Type:     schema.TypeString,
				Required: true,
			},
			"zone": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"cidr": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.CIDRNetwork(0, 32),
			},
			"netmask": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"gateway": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"dns1": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"dns2": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"network_domain": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func createNetwork(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)

	name := d.Get("name").(string)
	displayText := d.Get("display_text").(string)
	if displayText == "" {
		displayText = name
	}

	zoneName := d.Get("zone").(string)
	zone, err := getZoneByName(client, zoneName)
	if err != nil {
		return err
	}

	networkName := d.Get("network_offering").(string)
	networkOffering, err := getNetworkOfferingByName(client, networkName)
	if err != nil {
		return err
	}

	if networkOffering.SpecifyIPRanges {
		return fmt.Errorf("SpecifyIPRanges is not yet supported.")
	}

	netmask := net.IPv4zero
	gateway := net.IPv4zero

	if cidr, ok := d.GetOk("cidr"); ok {
		c := cidr.(string)
		ip, ipnet, err := net.ParseCIDR(c)
		if err != nil {
			return err
		}

		if ip.To4() == nil {
			return fmt.Errorf("Provided cidr %s is not an IPv4 address", c)
		}

		// subnet address
		subnetIP := ip.Mask(ipnet.Mask)
		// netmask
		netmask = net.IPv4(
			ipnet.Mask[0],
			ipnet.Mask[1],
			ipnet.Mask[2],
			ipnet.Mask[3])

		// last address
		gateway = net.IPv4(
			subnetIP[0]+^ipnet.Mask[0],
			subnetIP[1]+^ipnet.Mask[1],
			subnetIP[2]+^ipnet.Mask[2],
			subnetIP[3]+^ipnet.Mask[3])
	}

	resp, err := client.Request(&egoscale.CreateNetwork{
		Name:              name,
		DisplayText:       displayText,
		NetworkOfferingID: networkOffering.ID,
		ZoneID:            zone.ID,
		Netmask:           netmask,
		Gateway:           gateway,
	})

	if err != nil {
		return err
	}

	network := resp.(*egoscale.CreateNetworkResponse).Network

	d.SetId(network.ID)

	return readNetwork(d, meta)
}

func readNetwork(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	resp, err := client.Request(&egoscale.ListNetworks{
		ID: d.Id(),
	})

	if err != nil {
		return handleNotFound(d, err)
	}

	networks := resp.(*egoscale.ListNetworksResponse)
	if networks.Count == 0 {
		return fmt.Errorf("No network found for ID: %s", d.Id())
	}

	network := networks.Network[0]
	return applyNetwork(d, network)
}

func existsNetwork(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := GetComputeClient(meta)
	resp, err := client.Request(&egoscale.ListNetworks{
		ID: d.Id(),
	})

	if err != nil {
		e := handleNotFound(d, err)
		return d.Id() != "", e
	}

	networks := resp.(*egoscale.ListNetworksResponse)
	if networks.Count == 0 {
		d.SetId("")
		return false, nil
	}

	return true, nil
}

func updateNetwork(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	async := meta.(BaseConfig).async

	resp, err := client.AsyncRequest(&egoscale.UpdateNetwork{
		ID:          d.Id(),
		Name:        d.Get("name").(string),
		DisplayText: d.Get("display_text").(string),
	}, async)

	if err != nil {
		return err
	}

	network := resp.(*egoscale.UpdateNetworkResponse).Network
	return applyNetwork(d, network)
}

func deleteNetwork(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	async := meta.(BaseConfig).async

	err := client.BooleanAsyncRequest(&egoscale.DeleteNetwork{
		ID: d.Id(),
	}, async)

	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}

func importNetwork(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := readNetwork(d, meta); err != nil {
		return nil, err
	}

	resources := make([]*schema.ResourceData, 1)
	resources[0] = d
	return resources, nil
}

func applyNetwork(d *schema.ResourceData, network egoscale.Network) error {
	d.SetId(network.ID)
	d.Set("name", network.Name)
	d.Set("display_text", network.DisplayText)
	d.Set("network_domain", network.NetworkDomain)
	d.Set("network_offering", network.NetworkOfferingName)
	d.Set("zone", network.ZoneName)
	d.Set("cidr", network.Cidr)
	d.Set("gateway", network.Gateway.String())
	d.Set("netmask", network.Netmask.String())
	d.Set("dns1", network.DNS1)
	d.Set("dns2", network.DNS2)

	return nil
}