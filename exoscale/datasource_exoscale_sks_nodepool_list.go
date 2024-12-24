package exoscale

import (
	"context"
	"crypto/md5"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
	"github.com/exoscale/terraform-provider-exoscale/pkg/general"
	"github.com/exoscale/terraform-provider-exoscale/pkg/list"
)

const (
	dsSKSNodepoolsListIdentifier          = "exoscale_sks_nodepool_list"
	dsSKSNodepoolsListAttributeIdentifier = "nodepools"
)

func dataSourceSKSNodepoolListGetElementScheme() general.SchemaMap {
	return dataSourceSKSNodepool().Schema
}

func dataSourceSKSNodepoolList() *schema.Resource {
	return list.FilterableListDataSource(dsSKSNodepoolsListIdentifier, dsSKSNodepoolsListAttributeIdentifier, resSKSNodepoolAttrZone, getNodepoolList, nodepoolToDataMap, generateSKSNodepoolListID, dataSourceSKSNodepoolListGetElementScheme)
}

func generateSKSNodepoolListID(nodepools []*v3.SKSNodepool) string {
	ids := make([]string, 0, len(nodepools))

	for _, cluster := range nodepools {
		ids = append(ids, cluster.ID.String())
	}

	sort.Strings(ids)

	return fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(ids, ""))))
}

func getNodepoolList(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*v3.SKSNodepool, error) {
	zone := d.Get(resSKSClusterAttrZone).(string)

	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutRead))
	defer cancel()

	client, err := config.GetClientV3WithZone(ctx, meta, zone)
	if err != nil {
		return nil, err
	}

	clusters, err := client.ListSKSClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster list from zone %q: %s", zone, err)
	}

	var nodepools []*v3.SKSNodepool

	for _, cluster := range clusters.SKSClusters {
		for _, np := range cluster.Nodepools {
			nodepools = append(nodepools, &np)
		}
	}

	return nodepools, nil
}
