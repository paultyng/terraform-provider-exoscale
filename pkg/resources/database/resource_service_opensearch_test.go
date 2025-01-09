package database_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	exoapi "github.com/exoscale/egoscale/v2/api"
	"github.com/exoscale/egoscale/v2/oapi"

	"github.com/exoscale/terraform-provider-exoscale/pkg/testutils"
)

type TemplateModelOpensearch struct {
	ResourceName string

	Name string
	Plan string
	Zone string

	MaintenanceDow        string
	MaintenanceTime       string
	TerminationProtection bool

	IpFilter                 []string
	ForkFromService          string
	RecoveryBackupName       string
	IndexPatterns            []TemplateModelOpensearchIndexPattern
	IndexTemplate            *TemplateModelOpensearchIndexTemplate
	Dashboards               *TemplateModelOpensearchDashboards
	KeepIndexRefreshInterval bool
	MaxIndexCount            string
	OpensearchSettings       string
	Version                  string
}

type TemplateModelOpensearchIndexPattern struct {
	MaxIndexCount    int64
	Pattern          string
	SortingAlgorithm string
}

type TemplateModelOpensearchIndexTemplate struct {
	MappingNestedObjectsLimit int64
	NumberOfReplicas          int64
	NumberOfShards            int64
}

type TemplateModelOpensearchDashboards struct {
	Enabled         bool
	MaxOldSpaceSize int64
	RequestTimeout  int64
}

type TemplateModelOpensearchUser struct {
	ResourceName string

	Username string
	Service  string
	Zone     string
}

func testResourceOpensearch(t *testing.T) {
	serviceTpl, err := template.ParseFiles("testdata/resource_opensearch.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	userTpl, err := template.ParseFiles("testdata/resource_user_opensearch.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	serviceFullResourceName := "exoscale_dbaas.test"
	serviceDataBase := TemplateModelOpensearch{
		ResourceName:          "test",
		Name:                  acctest.RandomWithPrefix(testutils.Prefix),
		Plan:                  "hobbyist-2",
		Zone:                  testutils.TestZoneName,
		TerminationProtection: false,
		Version:               "1",
	}

	userFullResourceName := "exoscale_dbaas_opensearch_user.test_user"
	userDataBase := TemplateModelOpensearchUser{
		ResourceName: "test_user",
		Username:     "foo",
		Zone:         serviceDataBase.Zone,
		Service:      fmt.Sprintf("%s.name", serviceFullResourceName),
	}

	serviceDataCreate := serviceDataBase
	serviceDataCreate.MaintenanceDow = "monday"
	serviceDataCreate.MaintenanceTime = "01:23:00"
	serviceDataCreate.IndexPatterns = []TemplateModelOpensearchIndexPattern{
		{2, "log.?", "alphabetical"},
		{12, "internet.*", "creation_date"},
	}
	serviceDataCreate.IndexTemplate = &TemplateModelOpensearchIndexTemplate{5, 4, 3}
	serviceDataCreate.Dashboards = &TemplateModelOpensearchDashboards{true, 129, 30001}
	serviceDataCreate.KeepIndexRefreshInterval = true
	serviceDataCreate.IpFilter = []string{"0.0.0.0/0"}
	serviceDataCreate.MaxIndexCount = "4"

	userDataCreate := userDataBase

	buf := &bytes.Buffer{}
	err = serviceTpl.Execute(buf, &serviceDataCreate)
	if err != nil {
		t.Fatal(err)
	}
	err = userTpl.Execute(buf, &userDataCreate)
	if err != nil {
		t.Fatal(err)
	}
	configCreate := buf.String()

	serviceDataUpdate := serviceDataBase
	serviceDataUpdate.MaintenanceDow = "tuesday"
	serviceDataUpdate.MaintenanceTime = "02:34:00"
	serviceDataUpdate.IndexPatterns = []TemplateModelOpensearchIndexPattern{
		{4, "log.?", "alphabetical"},
		{12, "internet.*", "creation_date"},
	}
	serviceDataUpdate.IndexTemplate = &TemplateModelOpensearchIndexTemplate{5, 4, 3}
	serviceDataUpdate.Dashboards = &TemplateModelOpensearchDashboards{true, 132, 30006}
	serviceDataUpdate.KeepIndexRefreshInterval = true
	serviceDataUpdate.MaxIndexCount = "0"
	serviceDataUpdate.IpFilter = []string{"1.1.1.1/32"}
	serviceDataUpdate.IpFilter = nil

	userDataUpdate := userDataBase
	userDataUpdate.Username = "bar"

	buf = &bytes.Buffer{}
	err = serviceTpl.Execute(buf, &serviceDataUpdate)
	if err != nil {
		t.Fatal(err)
	}
	err = userTpl.Execute(buf, &userDataUpdate)
	if err != nil {
		t.Fatal(err)
	}
	configUpdate := buf.String()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testutils.AccPreCheck(t) },
		CheckDestroy:             CheckServiceDestroy("opensearch", serviceDataBase.Name),
		ProtoV6ProviderFactories: testutils.TestAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create
				Config: configCreate,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Service
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "created_at"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "disk_size"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "node_cpus"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "node_memory"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "nodes"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "ca_certificate"),
					resource.TestCheckResourceAttrSet(serviceFullResourceName, "updated_at"),
					func(s *terraform.State) error {
						err := CheckExistsOpensearch(serviceDataBase.Name, &serviceDataCreate)
						if err != nil {
							return err
						}

						return nil
					},
					// User
					resource.TestCheckResourceAttrSet(userFullResourceName, "password"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "type"),
					func(s *terraform.State) error {
						err := CheckExistsOpensearchUser(serviceDataBase.Name, userDataBase.Username, &userDataCreate)
						if err != nil {
							return err
						}

						return nil
					},
				),
			},
			{
				// Update
				Config: configUpdate,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Service
					func(s *terraform.State) error {
						err := CheckExistsOpensearch(serviceDataBase.Name, &serviceDataUpdate)
						if err != nil {
							return err
						}

						return nil
					},

					// User
					func(s *terraform.State) error {
						// Check the old user was deleted
						err := CheckExistsOpensearchUser(serviceDataBase.Name, userDataBase.Username, &userDataUpdate)
						if err == nil {
							return fmt.Errorf("expected to not find user %s", userDataBase.Username)
						}

						// Check the new user exists
						err = CheckExistsOpensearchUser(serviceDataBase.Name, userDataUpdate.Username, &userDataUpdate)
						if err != nil {
							return err
						}

						return nil
					},
				),
			},
			{
				// Import
				ResourceName: serviceFullResourceName,
				ImportStateIdFunc: func() resource.ImportStateIdFunc {
					return func(*terraform.State) (string, error) {
						return fmt.Sprintf("%s@%s", serviceDataBase.Name, serviceDataBase.Zone), nil
					}
				}(),
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: strings.Fields("updated_at state"),
			},
			{
				ResourceName: userFullResourceName,
				ImportStateIdFunc: func() resource.ImportStateIdFunc {
					return func(*terraform.State) (string, error) {
						return fmt.Sprintf("%s/%s@%s", serviceDataBase.Name, userDataUpdate.Username, userDataBase.Zone), nil
					}
				}(),
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func CheckExistsOpensearch(name string, data *TemplateModelOpensearch) error {
	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServiceOpensearchWithResponse(ctx, oapi.DbaasServiceName(name))
	if err != nil {
		return err
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("API request error: unexpected status %s", res.Status())
	}
	service := res.JSON200

	if data.Plan != service.Plan {
		return fmt.Errorf("plan: expected %q, got %q", data.Plan, service.Plan)
	}

	if *service.TerminationProtection != false {
		return fmt.Errorf("termination_protection: expected false, got true")
	}

	if !cmp.Equal(data.IpFilter, *service.IpFilter, cmpopts.EquateEmpty()) {
		return fmt.Errorf("opensearch.ip_filter: expected %q, got %q", data.IpFilter, *service.IpFilter)
	}

	if v := string(service.Maintenance.Dow); data.MaintenanceDow != v {
		return fmt.Errorf("opensearch.maintenance_dow: expected %q, got %q", data.MaintenanceDow, v)
	}

	if data.MaintenanceTime != service.Maintenance.Time {
		return fmt.Errorf("opensearch.maintenance_time: expected %q, got %q", data.MaintenanceTime, service.Maintenance.Time)
	}

	if data.KeepIndexRefreshInterval != *service.KeepIndexRefreshInterval {
		return fmt.Errorf("keep_index_refresh_interval: expected %v, got %v", data.KeepIndexRefreshInterval, *service.KeepIndexRefreshInterval)
	}

	if data.MaxIndexCount != strconv.FormatInt(*service.MaxIndexCount, 10) {
		return fmt.Errorf("max_index_count: expected %v, got %v", data.MaxIndexCount, *service.MaxIndexCount)
	}

	if len(data.IndexPatterns) != len(*service.IndexPatterns) {
		return fmt.Errorf("index_patterns: expected length of %v, got %v", len(data.IndexPatterns), len(*service.IndexPatterns))
	}

	if data.IndexTemplate != nil && service.IndexTemplate != nil {
		if data.IndexTemplate.MappingNestedObjectsLimit != *service.IndexTemplate.MappingNestedObjectsLimit {
			return fmt.Errorf("index_template.mapping_nasted_objects_limit: expected %v, got %v", data.IndexTemplate.MappingNestedObjectsLimit, *service.IndexTemplate.MappingNestedObjectsLimit)
		}
		if data.IndexTemplate.NumberOfReplicas != *service.IndexTemplate.NumberOfReplicas {
			return fmt.Errorf("index_template.number_of_replicas: expected %v, got %v", data.IndexTemplate.NumberOfReplicas, *service.IndexTemplate.NumberOfReplicas)
		}
		if data.IndexTemplate.NumberOfShards != *service.IndexTemplate.NumberOfShards {
			return fmt.Errorf("index_template.number_of_shards: expected %v, got %v", data.IndexTemplate.NumberOfShards, *service.IndexTemplate.NumberOfShards)
		}
	}

	if data.Dashboards != nil && service.OpensearchDashboards != nil {
		if data.Dashboards.Enabled != *service.OpensearchDashboards.Enabled {
			return fmt.Errorf("dashboards.enabled: expected %v, got %v", data.Dashboards.Enabled, *service.OpensearchDashboards.Enabled)
		}
		if data.Dashboards.MaxOldSpaceSize != *service.OpensearchDashboards.MaxOldSpaceSize {
			return fmt.Errorf("dashboards.max_old_space_size: expected %v, got %v", data.Dashboards.MaxOldSpaceSize, *service.OpensearchDashboards.MaxOldSpaceSize)
		}
		if data.Dashboards.RequestTimeout != *service.OpensearchDashboards.OpensearchRequestTimeout {
			return fmt.Errorf("dashboards.request_timeout: expected %v, got %v", data.Dashboards.RequestTimeout, *service.OpensearchDashboards.OpensearchRequestTimeout)
		}
	}

	if data.OpensearchSettings != "" {
		obj := map[string]interface{}{}
		s, err := strconv.Unquote(data.OpensearchSettings)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), &obj)
		if err != nil {
			return err
		}
		if !cmp.Equal(
			obj,
			*service.OpensearchSettings,
		) {
			return fmt.Errorf("opensearch.opensearch_settings: expected %q, got %q", obj, *service.OpensearchSettings)
		}
	}

	if data.Version != *service.Version {
		return fmt.Errorf("opensearch.version: expected %q, got %q", data.Version, *service.Version)
	}

	return nil
}

func CheckExistsOpensearchUser(service, username string, data *TemplateModelOpensearchUser) error {

	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServiceOpensearchWithResponse(ctx, oapi.DbaasServiceName(service))
	if err != nil {
		return err
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("API request error: unexpected status %s", res.Status())
	}
	svc := res.JSON200

	serviceUsernames := make([]string, 0)
	if svc.Users != nil {
		for _, u := range *svc.Users {
			if u.Username != nil {
				serviceUsernames = append(serviceUsernames, *u.Username)
				if *u.Username == username {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("could not find user %s for service %s, found %v", username, service, serviceUsernames)
}
