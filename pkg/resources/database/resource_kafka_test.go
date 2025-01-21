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

type TemplateModelKafka struct {
	ResourceName string

	Name string
	Plan string
	Zone string

	MaintenanceDow        string
	MaintenanceTime       string
	TerminationProtection bool

	EnableCertAuth         bool
	EnableKafkaConnect     bool
	EnableKafkaREST        bool
	EnableSASLAuth         bool
	EnableSchemaRegistry   bool
	IpFilter               []string
	KafkaSettings          string
	ConnectSettings        string
	RestSettings           string
	SchemaRegistrySettings string
	Version                string
}

type TemplateModelKafkaUser struct {
	ResourceName string

	Username string
	Zone     string
	Service  string

	Type     string
	Password string

	AccessKey        string
	AccessCert       string
	AccessCertExpiry string
}

func testResourceKafka(t *testing.T) {
	serviceTpl, err := template.ParseFiles("testdata/resource_kafka.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	userTpl, err := template.ParseFiles("testdata/resource_user_kafka.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	serviceFullResourceName := "exoscale_database.test"
	serviceDataBase := TemplateModelKafka{
		ResourceName:          "test",
		Name:                  acctest.RandomWithPrefix(testutils.Prefix),
		Plan:                  "business-4",
		Zone:                  testutils.TestZoneName,
		TerminationProtection: false,
		Version:               "3.8",
	}

	userFullResourceName := "exoscale_dbaas_kafka_user.test_user"
	userDataBase := TemplateModelKafkaUser{
		ResourceName: "test_user",
		Username:     "foo",
		Zone:         serviceDataBase.Zone,
		Service:      fmt.Sprintf("%s.name", serviceFullResourceName),
	}

	serviceDataCreate := serviceDataBase
	serviceDataCreate.MaintenanceDow = "monday"
	serviceDataCreate.MaintenanceTime = "01:23:00"
	serviceDataCreate.EnableCertAuth = true
	serviceDataCreate.IpFilter = []string{"1.2.3.4/32"}
	serviceDataCreate.KafkaSettings = strconv.Quote(`{"num_partitions":10}`)

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
	serviceDataUpdate.EnableCertAuth = false
	serviceDataUpdate.EnableSASLAuth = true
	serviceDataUpdate.EnableKafkaREST = true
	serviceDataUpdate.EnableKafkaConnect = true
	serviceDataUpdate.IpFilter = nil
	serviceDataUpdate.KafkaSettings = strconv.Quote(`{"compression_type":"gzip","num_partitions":10}`)
	serviceDataUpdate.RestSettings = strconv.Quote(`{"consumer_request_max_bytes":100000}`)
	serviceDataUpdate.ConnectSettings = strconv.Quote(`{"session_timeout_ms":6000}`)

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
		CheckDestroy:             CheckServiceDestroy("kafka", serviceDataBase.Name),
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
						err := CheckExistsKafka(serviceDataBase.Name, &serviceDataCreate)
						if err != nil {
							return err
						}

						return nil
					},

					// User
					resource.TestCheckResourceAttrSet(userFullResourceName, "password"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "type"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "access_key"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "access_cert"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "access_cert_expiry"),
					func(s *terraform.State) error {
						err := CheckExistsKafkaUser(serviceDataBase.Name, userDataBase.Username, &userDataCreate)
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
						err := CheckExistsKafka(serviceDataBase.Name, &serviceDataUpdate)
						if err != nil {
							return err
						}

						return nil
					},

					// User
					func(s *terraform.State) error {
						// Check the old user was deleted
						err := CheckExistsKafkaUser(serviceDataBase.Name, userDataBase.Username, &userDataUpdate)
						if err == nil {
							return fmt.Errorf("expected to not find user %s", userDataBase.Username)
						}

						// Check the new user exists
						err = CheckExistsKafkaUser(serviceDataBase.Name, userDataUpdate.Username, &userDataUpdate)
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

func CheckExistsKafka(name string, data *TemplateModelKafka) error {
	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServiceKafkaWithResponse(ctx, oapi.DbaasServiceName(name))
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
		return fmt.Errorf("kafka.ip_filter: expected %q, got %q", data.IpFilter, *service.IpFilter)
	}

	if v := string(service.Maintenance.Dow); data.MaintenanceDow != v {
		return fmt.Errorf("kafka.maintenance_dow: expected %q, got %q", data.MaintenanceDow, v)
	}

	if data.MaintenanceTime != service.Maintenance.Time {
		return fmt.Errorf("kafka.maintenance_time: expected %q, got %q", data.MaintenanceTime, service.Maintenance.Time)
	}

	if data.EnableKafkaConnect != *service.KafkaConnectEnabled {
		return fmt.Errorf("kafka.enable_kafka_connect: expected %v, got %v", data.EnableKafkaConnect, *service.KafkaConnectEnabled)
	}

	if data.EnableKafkaREST != *service.KafkaRestEnabled {
		return fmt.Errorf("kafka.enable_kafka_rest: expected %v, got %v", data.EnableKafkaREST, *service.KafkaRestEnabled)
	}

	if data.EnableSchemaRegistry != *service.SchemaRegistryEnabled {
		return fmt.Errorf("kafka.enable_schema_registry: expected %v, got %v", data.EnableSchemaRegistry, *service.SchemaRegistryEnabled)
	}

	if data.EnableCertAuth != *service.AuthenticationMethods.Certificate {
		return fmt.Errorf("kafka.enable_cert_auth: expected %v, got %v", data.EnableCertAuth, *service.AuthenticationMethods.Certificate)
	}

	if data.EnableSASLAuth != *service.AuthenticationMethods.Sasl {
		return fmt.Errorf("kafka.enable_sasl_auth: expected %v, got %v", data.EnableSASLAuth, *service.AuthenticationMethods.Sasl)
	}

	if data.KafkaSettings != "" {
		obj := map[string]interface{}{}
		s, err := strconv.Unquote(data.KafkaSettings)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), &obj)
		if err != nil {
			return err
		}
		if !cmp.Equal(
			obj,
			*service.KafkaSettings,
		) {
			return fmt.Errorf("kafka.kafka_settings: expected %q, got %q", obj, *service.KafkaSettings)
		}
	}

	if data.ConnectSettings != "" {
		obj := map[string]interface{}{}
		s, err := strconv.Unquote(data.ConnectSettings)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), &obj)
		if err != nil {
			return err
		}
		if !cmp.Equal(
			obj,
			*service.KafkaConnectSettings,
		) {
			return fmt.Errorf("kafka.kafka_connect_settings: expected %q, got %q", obj, *service.KafkaConnectSettings)
		}
	}

	if data.RestSettings != "" {
		obj := map[string]interface{}{}
		s, err := strconv.Unquote(data.RestSettings)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), &obj)
		if err != nil {
			return err
		}
		if !cmp.Equal(
			obj,
			*service.KafkaRestSettings,
		) {
			return fmt.Errorf("kafka.kafka_rest_settings: expected %q, got %q", obj, *service.KafkaRestSettings)
		}
	}

	if data.SchemaRegistrySettings != "" {
		obj := map[string]interface{}{}
		s, err := strconv.Unquote(data.SchemaRegistrySettings)
		if err != nil {
			return err
		}
		err = json.Unmarshal([]byte(s), &obj)
		if err != nil {
			return err
		}
		if !cmp.Equal(
			obj,
			*service.SchemaRegistrySettings,
		) {
			return fmt.Errorf("kafka.schema_registry_settings: expected %q, got %q", obj, *service.SchemaRegistrySettings)
		}
	}

	if data.Version != *service.Version {
		return fmt.Errorf("kafka.version: expected %q, got %q", data.Version, *service.Version)
	}

	return nil
}

func CheckExistsKafkaUser(service, username string, data *TemplateModelKafkaUser) error {

	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServiceKafkaWithResponse(ctx, oapi.DbaasServiceName(service))
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
