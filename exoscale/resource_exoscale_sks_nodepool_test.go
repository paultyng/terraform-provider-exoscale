package exoscale

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"

	egoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"
)

var (
	testAccResourceSKSNodepoolAntiAffinityGroupName             = acctest.RandomWithPrefix(testPrefix)
	testAccResourceSKSNodepoolDescription                       = acctest.RandomWithPrefix(testPrefix)
	testAccResourceSKSNodepoolDescriptionUpdated                = testAccResourceSKSNodepoolDescription + "-updated"
	testAccResourceSKSNodepoolDiskSize                          = defaultSKSNodepoolDiskSize * 2
	testAccResourceSKSNodepoolDiskSizeUpdated                   = defaultSKSNodepoolDiskSize*2 + 10
	testAccResourceSKSNodepoolInstancePrefix                    = "test"
	testAccResourceSKSNodepoolInstanceType                      = "standard.small"
	testAccResourceSKSNodepoolInstanceTypeUpdated               = "standard.medium"
	testAccResourceSKSNodepoolKubeletImageGCMinAge              = "1m"
	testAccResourceSKSNodepoolKubeletImageGCHighThreshold int64 = 13
	testAccResourceSKSNodepoolKubeletImageGCLowThreshold  int64 = 12
	testAccResourceSKSNodepoolLabelValue                        = acctest.RandomWithPrefix(testPrefix)
	testAccResourceSKSNodepoolLabelValueUpdated                 = testAccResourceSKSNodepoolLabelValue + "-updated"
	testAccResourceSKSNodepoolName                              = acctest.RandomWithPrefix(testPrefix)
	testAccResourceSKSNodepoolNameUpdated                       = testAccResourceSKSNodepoolName + "-updated"
	testAccResourceSKSNodepoolPrivateNetworkName                = acctest.RandomWithPrefix(testPrefix)
	testAccResourceSKSNodepoolSize                        int64 = 2
	testAccResourceSKSNodepoolSizeUpdated                 int64 = 1
	testAccResourceSKSNodepoolIPV6Default                 bool  = false
	testAccResourceSKSNodepoolIPV6Updated                 bool  = true
	testAccResourceSKSNodepoolStorageLVM                  bool  = true
	testAccResourceSKSNodepoolTaintEffect                       = "NoSchedule"
	testAccResourceSKSNodepoolTaintValue                        = "test"
	testAccResourceSKSNodepoolTaintValueUpdated                 = "test-updated"

	testAccResourceSKSNodepoolConfigCreate = fmt.Sprintf(`
locals {
  zone = "%s"
}

resource "exoscale_sks_cluster" "test" {
  zone = local.zone
  name = "%s"

  timeouts {
    delete = "10m"
  }
}

resource "exoscale_sks_nodepool" "test" {
  zone = local.zone
  cluster_id = exoscale_sks_cluster.test.id
  name = "%s"
  description = "%s"
  instance_type = "%s"
  disk_size = %d
  size = %d
  instance_prefix = "%s"
  labels = { test = "%s" }
  taints = { test = "%s:%s" }
  storage_lvm = %t

  timeouts {
    delete = "10m"
  }

  kubelet_image_gc {
    min_age = "%s"
	high_threshold = %d
	low_threshold = %d
  }
}
`,
		testAccResourceSKSClusterLocalZone,
		testAccResourceSKSClusterName,
		testAccResourceSKSNodepoolName,
		testAccResourceSKSNodepoolDescription,
		testAccResourceSKSNodepoolInstanceType,
		testAccResourceSKSNodepoolDiskSize,
		testAccResourceSKSNodepoolSize,
		testAccResourceSKSNodepoolInstancePrefix,
		testAccResourceSKSNodepoolLabelValue,
		testAccResourceSKSNodepoolTaintValue,
		testAccResourceSKSNodepoolTaintEffect,
		testAccResourceSKSNodepoolStorageLVM,
		testAccResourceSKSNodepoolKubeletImageGCMinAge,
		testAccResourceSKSNodepoolKubeletImageGCHighThreshold,
		testAccResourceSKSNodepoolKubeletImageGCLowThreshold,
	)

	testAccResourceSKSNodepoolConfigUpdate = fmt.Sprintf(`
locals {
  zone = "%s"
}

data "exoscale_security_group" "default" {
  name = "default"
}

resource "exoscale_anti_affinity_group" "test" {
  name = "%s"
}

resource "exoscale_private_network" "test" {
  zone     = local.zone
  name     = "%s"
  start_ip = "10.0.0.20"
  end_ip   = "10.0.0.253"
  netmask  = "255.255.255.0"
}

resource "exoscale_sks_cluster" "test" {
  zone = local.zone
  name = "%s"

  timeouts {
    delete = "10m"
  }
}

resource "exoscale_sks_nodepool" "test" {
  zone = local.zone
  cluster_id = exoscale_sks_cluster.test.id
  name = "%s"
  description = "%s"
  instance_type = "%s"
  disk_size = %d
  size = %d
  instance_prefix = "%s"
  anti_affinity_group_ids = [exoscale_anti_affinity_group.test.id]
  security_group_ids = [data.exoscale_security_group.default.id]
  private_network_ids = [exoscale_private_network.test.id]
  labels = { test = "%s" }
  taints = { test = "%s:%s" }
  storage_lvm = %t
  ipv6        = %t

  timeouts {
    delete = "10m"
  }

  kubelet_image_gc {
    min_age = "%s"
	high_threshold = %d
	low_threshold = %d
  }
}
	  `,
		testAccResourceSKSClusterLocalZone,
		testAccResourceSKSNodepoolAntiAffinityGroupName,
		testAccResourceSKSNodepoolPrivateNetworkName,
		testAccResourceSKSClusterName,
		testAccResourceSKSNodepoolNameUpdated,
		testAccResourceSKSNodepoolDescriptionUpdated,
		testAccResourceSKSNodepoolInstanceTypeUpdated,
		testAccResourceSKSNodepoolDiskSizeUpdated,
		testAccResourceSKSNodepoolSizeUpdated,
		defaultSKSNodepoolInstancePrefix,
		testAccResourceSKSNodepoolLabelValueUpdated,
		testAccResourceSKSNodepoolTaintValueUpdated,
		testAccResourceSKSNodepoolTaintEffect,
		testAccResourceSKSNodepoolStorageLVM,
		testAccResourceSKSNodepoolIPV6Updated,
		testAccResourceSKSNodepoolKubeletImageGCMinAge,
		testAccResourceSKSNodepoolKubeletImageGCHighThreshold,
		testAccResourceSKSNodepoolKubeletImageGCLowThreshold,
	)
)

func TestAccResourceSKSNodepool(t *testing.T) {
	var (
		r           = "exoscale_sks_nodepool.test"
		sksCluster  egoscale.SKSCluster
		sksNodepool egoscale.SKSNodepool
	)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviders,
		CheckDestroy:      testAccCheckResourceSKSNodepoolDestroy(r),
		Steps: []resource.TestStep{
			{
				// Create
				Config: testAccResourceSKSNodepoolConfigCreate,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceSKSClusterExists("exoscale_sks_cluster.test", &sksCluster),
					testAccCheckResourceSKSNodepoolExists(r, &sksNodepool),
					func(s *terraform.State) error {
						a := require.New(t)

						a.Equal(testAccResourceSKSNodepoolDescription, *sksNodepool.Description)
						a.Equal(testAccResourceSKSNodepoolDiskSize, *sksNodepool.DiskSize)
						a.Equal(testAccResourceSKSNodepoolLabelValue, (*sksNodepool.Labels)["test"])
						a.Equal(testAccResourceSKSNodepoolName, *sksNodepool.Name)
						a.Equal(testAccResourceSKSNodepoolInstancePrefix, *sksNodepool.InstancePrefix)
						a.Equal(testInstanceTypeIDSmall, *sksNodepool.InstanceTypeID)
						a.Equal(testAccResourceSKSNodepoolSize, *sksNodepool.Size)
						a.Equal(1, len(*sksNodepool.AddOns))
						a.Equal(&egoscale.SKSNodepoolTaint{
							Effect: testAccResourceSKSNodepoolTaintEffect,
							Value:  testAccResourceSKSNodepoolTaintValue,
						}, (*sksNodepool.Taints)["test"])
						a.Equal(&egoscale.SKSNodepoolKubeletImageGc{
							MinAge:        &testAccResourceSKSNodepoolKubeletImageGCMinAge,
							HighThreshold: &testAccResourceSKSNodepoolKubeletImageGCHighThreshold,
							LowThreshold:  &testAccResourceSKSNodepoolKubeletImageGCLowThreshold},
							sksNodepool.KubeletImageGc)
						return nil
					},
					checkResourceState(r, checkResourceStateValidateAttributes(testAttrs{
						resSKSNodepoolAttrCreatedAt:                                                    validation.ToDiagFunc(validation.NoZeroValues),
						resSKSNodepoolAttrDescription:                                                  validateString(testAccResourceSKSNodepoolDescription),
						resSKSNodepoolAttrDiskSize:                                                     validateString(fmt.Sprint(testAccResourceSKSNodepoolDiskSize)),
						resSKSNodepoolAttrInstancePoolID:                                               validation.ToDiagFunc(validation.IsUUID),
						resSKSNodepoolAttrInstancePrefix:                                               validateString(testAccResourceSKSNodepoolInstancePrefix),
						resSKSNodepoolAttrInstanceType:                                                 validateString(testAccResourceSKSNodepoolInstanceType),
						resSKSNodepoolAttrKubeletGC + ".#":                                             validateString("1"),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCMinAge:        validateString(testAccResourceSKSNodepoolKubeletImageGCMinAge),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCHighThreshold: validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCHighThreshold)),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCLowThreshold:  validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCLowThreshold)),
						resSKSNodepoolAttrLabels + ".test":                                             validateString(testAccResourceSKSNodepoolLabelValue),
						resSKSNodepoolAttrName:                                                         validateString(testAccResourceSKSNodepoolName),
						resSKSNodepoolAttrSize:                                                         validateString(fmt.Sprint(testAccResourceSKSNodepoolSize)),
						resSKSNodepoolAttrState:                                                        validation.ToDiagFunc(validation.NoZeroValues),
						resSKSNOdepoolAttrIPV6Enabled:                                                  validateBool(testAccResourceSKSNodepoolIPV6Default),
						resSKSNodepoolAttrTaints + ".test": validateString(fmt.Sprintf(
							"%s:%s",
							testAccResourceSKSNodepoolTaintValue,
							testAccResourceSKSNodepoolTaintEffect,
						)),
						resSKSNodepoolAttrTemplateID: validation.ToDiagFunc(validation.IsUUID),
						resSKSNodepoolAttrVersion:    validation.ToDiagFunc(validation.NoZeroValues),
					})),
				),
			},
			{
				// Update
				Config: testAccResourceSKSNodepoolConfigUpdate,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourceSKSNodepoolExists(r, &sksNodepool),
					func(s *terraform.State) error {
						a := require.New(t)
						a.Len(*sksNodepool.AntiAffinityGroupIDs, 1)
						a.Equal(testAccResourceSKSNodepoolDescriptionUpdated, *sksNodepool.Description)
						a.Equal(testAccResourceSKSNodepoolDiskSizeUpdated, *sksNodepool.DiskSize)
						a.Equal(testAccResourceSKSNodepoolLabelValueUpdated, (*sksNodepool.Labels)["test"])
						a.Equal(testAccResourceSKSNodepoolNameUpdated, *sksNodepool.Name)
						a.Equal(defaultSKSNodepoolInstancePrefix, *sksNodepool.InstancePrefix)
						a.Equal(testInstanceTypeIDMedium, *sksNodepool.InstanceTypeID)
						a.Len(*sksNodepool.PrivateNetworkIDs, 1)
						a.Len(*sksNodepool.SecurityGroupIDs, 1)
						a.Equal(testAccResourceSKSNodepoolSizeUpdated, *sksNodepool.Size)
						a.Equal(&egoscale.SKSNodepoolTaint{
							Effect: testAccResourceSKSNodepoolTaintEffect,
							Value:  testAccResourceSKSNodepoolTaintValueUpdated,
						}, (*sksNodepool.Taints)["test"])
						a.Equal(&egoscale.SKSNodepoolKubeletImageGc{
							MinAge:        &testAccResourceSKSNodepoolKubeletImageGCMinAge,
							HighThreshold: &testAccResourceSKSNodepoolKubeletImageGCHighThreshold,
							LowThreshold:  &testAccResourceSKSNodepoolKubeletImageGCLowThreshold},
							sksNodepool.KubeletImageGc)

						return nil
					},
					checkResourceState(r, checkResourceStateValidateAttributes(testAttrs{
						resSKSNodepoolAttrAntiAffinityGroupIDs + ".#":                                  validateString("1"),
						resSKSNodepoolAttrCreatedAt:                                                    validation.ToDiagFunc(validation.NoZeroValues),
						resSKSNodepoolAttrDescription:                                                  validateString(testAccResourceSKSNodepoolDescriptionUpdated),
						resSKSNodepoolAttrDiskSize:                                                     validateString(fmt.Sprint(testAccResourceSKSNodepoolDiskSizeUpdated)),
						resSKSNodepoolAttrInstancePoolID:                                               validation.ToDiagFunc(validation.IsUUID),
						resSKSNodepoolAttrInstancePrefix:                                               validateString(defaultSKSNodepoolInstancePrefix),
						resSKSNodepoolAttrInstanceType:                                                 validateString(testAccResourceSKSNodepoolInstanceTypeUpdated),
						resSKSNodepoolAttrKubeletGC + ".#":                                             validateString("1"),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCMinAge:        validateString(testAccResourceSKSNodepoolKubeletImageGCMinAge),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCHighThreshold: validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCHighThreshold)),
						resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCLowThreshold:  validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCLowThreshold)),
						resSKSNodepoolAttrLabels + ".test":                                             validateString(testAccResourceSKSNodepoolLabelValueUpdated),
						resSKSNodepoolAttrName:                                                         validateString(testAccResourceSKSNodepoolNameUpdated),
						resSKSNodepoolAttrPrivateNetworkIDs + ".#":                                     validateString("1"),
						resSKSNodepoolAttrSecurityGroupIDs + ".#":                                      validateString("1"),
						resSKSNodepoolAttrSize:                                                         validateString(fmt.Sprint(testAccResourceSKSNodepoolSizeUpdated)),
						resSKSNodepoolAttrState:                                                        validation.ToDiagFunc(validation.NoZeroValues),
						resSKSNOdepoolAttrIPV6Enabled:                                                  validateBool(testAccResourceSKSNodepoolIPV6Updated),
						resSKSNodepoolAttrTaints + ".test": validateString(fmt.Sprintf(
							"%s:%s",
							testAccResourceSKSNodepoolTaintValueUpdated,
							testAccResourceSKSNodepoolTaintEffect,
						)),
						resSKSNodepoolAttrTemplateID: validation.ToDiagFunc(validation.IsUUID),
						resSKSNodepoolAttrVersion:    validation.ToDiagFunc(validation.NoZeroValues),
					})),
				),
			},
			{
				// Import
				ResourceName: r,
				ImportStateIdFunc: func(
					sksCluster *egoscale.SKSCluster,
					sksNodepool *egoscale.SKSNodepool,
				) resource.ImportStateIdFunc {
					return func(*terraform.State) (string, error) {
						return fmt.Sprintf("%s/%s@%s", *sksCluster.ID, *sksNodepool.ID, testAccResourceSKSClusterLocalZone), nil
					}
				}(&sksCluster, &sksNodepool),
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					return checkResourceAttributes(
						testAttrs{
							resSKSNodepoolAttrAntiAffinityGroupIDs + ".#":                                  validateString("1"),
							resSKSNodepoolAttrClusterID:                                                    validation.ToDiagFunc(validation.IsUUID),
							resSKSNodepoolAttrCreatedAt:                                                    validation.ToDiagFunc(validation.NoZeroValues),
							resSKSNodepoolAttrDescription:                                                  validateString(testAccResourceSKSNodepoolDescriptionUpdated),
							resSKSNodepoolAttrDiskSize:                                                     validateString(fmt.Sprint(testAccResourceSKSNodepoolDiskSizeUpdated)),
							resSKSNodepoolAttrInstancePoolID:                                               validation.ToDiagFunc(validation.IsUUID),
							resSKSNodepoolAttrInstancePrefix:                                               validateString(defaultSKSNodepoolInstancePrefix),
							resSKSNodepoolAttrInstanceType:                                                 validateString(testAccResourceSKSNodepoolInstanceTypeUpdated),
							resSKSNodepoolAttrKubeletGC + ".#":                                             validateString("1"),
							resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCMinAge:        validateString(testAccResourceSKSNodepoolKubeletImageGCMinAge),
							resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCHighThreshold: validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCHighThreshold)),
							resSKSNodepoolAttrKubeletGC + ".0." + resSKSNodepoolAttrKubeletGCLowThreshold:  validateString(fmt.Sprint(testAccResourceSKSNodepoolKubeletImageGCLowThreshold)),
							resSKSNodepoolAttrLabels + ".test":                                             validateString(testAccResourceSKSNodepoolLabelValueUpdated),
							resSKSNodepoolAttrName:                                                         validateString(testAccResourceSKSNodepoolNameUpdated),
							resSKSNodepoolAttrPrivateNetworkIDs + ".#":                                     validateString("1"),
							resSKSNodepoolAttrSecurityGroupIDs + ".#":                                      validateString("1"),
							resSKSNodepoolAttrSize:                                                         validateString(fmt.Sprint(testAccResourceSKSNodepoolSizeUpdated)),
							resSKSNodepoolAttrState:                                                        validation.ToDiagFunc(validation.NoZeroValues),
							resSKSNodepoolAttrTaints + ".test": validateString(fmt.Sprintf(
								"%s:%s",
								testAccResourceSKSNodepoolTaintValueUpdated,
								testAccResourceSKSNodepoolTaintEffect,
							)),
							resSKSNodepoolAttrTemplateID: validation.ToDiagFunc(validation.IsUUID),
							resSKSNodepoolAttrVersion:    validation.ToDiagFunc(validation.NoZeroValues),
						},
						func(s []*terraform.InstanceState) map[string]string {
							for _, state := range s {
								if state.ID == *sksNodepool.ID {
									return state.Attributes
								}
							}
							return nil
						}(s),
					)
				},
			},
		},
	})
}

func testAccCheckResourceSKSNodepoolExists(r string, sksNodepool *egoscale.SKSNodepool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return errors.New("resource not found in the state")
		}

		if rs.Primary.ID == "" {
			return errors.New("resource ID not set")
		}

		clusterID, ok := rs.Primary.Attributes[resSKSNodepoolAttrClusterID]
		if !ok {
			return fmt.Errorf("resource attribute %q not set", resSKSNodepoolAttrClusterID)
		}

		client := getClient(testAccProvider.Meta())
		ctx := exoapi.WithEndpoint(
			context.Background(),
			exoapi.NewReqEndpoint(testEnvironment, testAccResourceSKSClusterLocalZone),
		)

		cluster, err := client.GetSKSCluster(ctx, testAccResourceSKSClusterLocalZone, clusterID)
		if err != nil {
			return err
		}

		for _, np := range cluster.Nodepools {
			if *np.ID == rs.Primary.ID {
				*sksNodepool = *np
				// (sc-74233) Additional tests to prevent deletion while instancepool is updating or scaling
				// Wait for the nodepool and the underlying instancepool to be in running state
				for i := 0; i < 60; i++ {
					npool, err := client.GetSksNodepoolWithResponse(ctx, clusterID, *np.ID)
					if err != nil {
						return err
					}
					npstate := *npool.JSON200.State
					ipool, err := client.GetInstancePoolWithResponse(ctx, *np.InstancePoolID)
					if err != nil {
						return err
					}
					ipstate := *ipool.JSON200.State
					if npstate == "running" && ipstate == "running" {
						return nil
					}
					time.Sleep(10 * time.Second)
				}
				return fmt.Errorf("resource SKS Nodepool (%s) and InstancePool (%s) not running", *np.ID, *np.InstancePoolID)
			}
		}

		return fmt.Errorf("resource SKS Nodepool %q not found", rs.Primary.ID)
	}
}

func testAccCheckResourceSKSNodepoolDestroy(r string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[r]
		if !ok {
			return errors.New("resource not found in the state")
		}

		if rs.Primary.ID == "" {
			return errors.New("resource ID not set")
		}

		clusterID, ok := rs.Primary.Attributes[resSKSNodepoolAttrClusterID]
		if !ok {
			return fmt.Errorf("resource attribute %q not set", resSKSNodepoolAttrClusterID)
		}

		client := getClient(testAccProvider.Meta())
		ctx := exoapi.WithEndpoint(
			context.Background(),
			exoapi.NewReqEndpoint(testEnvironment, testAccResourceSKSClusterLocalZone),
		)

		sksCluster, err := client.GetSKSCluster(ctx, testAccResourceSKSClusterLocalZone, clusterID)
		if err != nil {
			if errors.Is(err, exoapi.ErrNotFound) {
				return nil
			}

			return err
		}

		for _, np := range sksCluster.Nodepools {
			if *np.ID == rs.Primary.ID {
				return errors.New("SKS Nodepool still exists")
			}
		}

		return nil
	}
}
