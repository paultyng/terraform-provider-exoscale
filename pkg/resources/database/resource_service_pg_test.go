package database_test

import (
	"bytes"
	"context"
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

type TemplateModelPg struct {
	ResourceName string

	Name string
	Plan string
	Zone string

	MaintenanceDow        string
	MaintenanceTime       string
	TerminationProtection bool

	AdminPassword     string
	AdminUsername     string
	BackupSchedule    string
	IpFilter          []string
	PgSettings        string
	PgbouncerSettings string
	PglookoutSettings string
	Version           string
}

type TemplateModelPgUser struct {
	ResourceName string

	Username string
	Service  string
	Zone     string
}

type TemplateModelPgDb struct {
	ResourceName string

	DatabaseName string
	Service      string
	Zone         string
}

func testResourcePg(t *testing.T) {
	serviceTpl, err := template.ParseFiles("testdata/resource_pg.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	userTpl, err := template.ParseFiles("testdata/resource_user_pg.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	dbTpl, err := template.ParseFiles("testdata/resource_database_pg.tmpl")
	if err != nil {
		t.Fatal(err)
	}

	serviceFullResourceName := "exoscale_dbaas.test"
	serviceDataBase := TemplateModelPg{
		ResourceName:          "test",
		Name:                  acctest.RandomWithPrefix(testutils.Prefix),
		Plan:                  "hobbyist-2",
		Zone:                  testutils.TestZoneName,
		TerminationProtection: false,
		Version:               "13",
	}

	userFullResourceName := "exoscale_dbaas_pg_user.test_user"
	userDataBase := TemplateModelPgUser{
		ResourceName: "test_user",
		Username:     "foo",
		Zone:         serviceDataBase.Zone,
		Service:      fmt.Sprintf("%s.name", serviceFullResourceName),
	}

	dbFullResourceName := "exoscale_dbaas_pg_database.test_database"
	dbDataBase := TemplateModelPgDb{
		ResourceName: "test_database",
		DatabaseName: "foo_db",
		Zone:         serviceDataBase.Zone,
		Service:      fmt.Sprintf("%s.name", serviceFullResourceName),
	}

	serviceDataCreate := serviceDataBase
	serviceDataCreate.MaintenanceDow = "monday"
	serviceDataCreate.MaintenanceTime = "01:23:00"
	serviceDataCreate.BackupSchedule = "01:23"
	serviceDataCreate.IpFilter = []string{"1.2.3.4/32"}
	serviceDataCreate.PgSettings = strconv.Quote(`{"timezone":"Europe/Zurich"}`)
	serviceDataCreate.PgbouncerSettings = strconv.Quote(`{"min_pool_size":10}`)

	userDataCreate := userDataBase
	dbDataCreate := dbDataBase

	buf := &bytes.Buffer{}
	err = serviceTpl.Execute(buf, &serviceDataCreate)
	if err != nil {
		t.Fatal(err)
	}
	err = userTpl.Execute(buf, &userDataCreate)
	if err != nil {
		t.Fatal(err)
	}
	err = dbTpl.Execute(buf, &dbDataCreate)
	if err != nil {
		t.Fatal(err)
	}
	configCreate := buf.String()

	serviceDataUpdate := serviceDataBase
	serviceDataUpdate.MaintenanceDow = "tuesday"
	serviceDataUpdate.MaintenanceTime = "02:34:00"
	serviceDataUpdate.BackupSchedule = "23:45"
	serviceDataUpdate.IpFilter = nil
	serviceDataUpdate.PgSettings = strconv.Quote(`{"max_worker_processes":10,"timezone":"Europe/Zurich"}`)
	serviceDataUpdate.PgbouncerSettings = strconv.Quote(`{"autodb_pool_size":5,"min_pool_size":10}`)
	serviceDataUpdate.PglookoutSettings = strconv.Quote(`{"max_failover_replication_time_lag":30}`)

	userDataUpdate := userDataBase
	userDataUpdate.Username = "bar"

	dbDataUpdate := dbDataBase
	dbDataUpdate.DatabaseName = "bar_db"

	buf = &bytes.Buffer{}
	err = serviceTpl.Execute(buf, &serviceDataUpdate)
	if err != nil {
		t.Fatal(err)
	}
	err = userTpl.Execute(buf, &userDataUpdate)
	if err != nil {
		t.Fatal(err)
	}
	err = dbTpl.Execute(buf, &dbDataUpdate)
	if err != nil {
		t.Fatal(err)
	}
	configUpdate := buf.String()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testutils.AccPreCheck(t) },
		CheckDestroy:             CheckServiceDestroy("pg", serviceDataBase.Name),
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
						err := CheckExistsPg(serviceDataBase.Name, &serviceDataCreate)
						if err != nil {
							return err
						}

						return nil
					},
					// User
					resource.TestCheckResourceAttrSet(userFullResourceName, "password"),
					resource.TestCheckResourceAttrSet(userFullResourceName, "type"),
					func(s *terraform.State) error {
						err := CheckExistsPgUser(serviceDataBase.Name, userDataBase.Username, &userDataCreate)
						if err != nil {
							return err
						}

						return nil
					},

					// Database
					func(s *terraform.State) error {
						err := CheckExistsPgDatabase(serviceDataBase.Name, dbDataCreate.DatabaseName, &dbDataCreate)
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
						err := CheckExistsPg(serviceDataBase.Name, &serviceDataUpdate)
						if err != nil {
							return err
						}

						return nil
					},

					// User
					func(s *terraform.State) error {
						// Check the old user was deleted
						err := CheckExistsPgUser(serviceDataBase.Name, userDataBase.Username, &userDataUpdate)
						if err == nil {
							return fmt.Errorf("expected to not find user %s", userDataBase.Username)
						}

						// Check the new user exists
						err = CheckExistsPgUser(serviceDataBase.Name, userDataUpdate.Username, &userDataUpdate)
						if err != nil {
							return err
						}

						return nil
					},

					// Database
					func(s *terraform.State) error {
						// Check the old database was deleted
						err := CheckExistsPgDatabase(serviceDataBase.Name, dbDataBase.DatabaseName, &dbDataUpdate)
						if err == nil {
							return fmt.Errorf("expected to not find database %s", dbDataBase.DatabaseName)
						}

						// Check the new user exists
						err = CheckExistsPgDatabase(serviceDataBase.Name, dbDataUpdate.DatabaseName, &dbDataUpdate)
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
				ImportState: true,
				// NOTE: ImportStateVerify doesn't work when there are optional attributes.
				//ImportStateVerify: true
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
			{
				ResourceName: dbFullResourceName,
				ImportStateIdFunc: func() resource.ImportStateIdFunc {
					return func(*terraform.State) (string, error) {
						return fmt.Sprintf("%s/%s@%s", serviceDataBase.Name, dbDataUpdate.DatabaseName, dbDataBase.Zone), nil
					}
				}(),
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func CheckExistsPg(name string, data *TemplateModelPg) error {
	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(name))
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

	if v := fmt.Sprintf("%02d:%02d", *service.BackupSchedule.BackupHour, *service.BackupSchedule.BackupMinute); data.BackupSchedule != v {
		return fmt.Errorf("backup_schedule: expected %q, got %q", data.BackupSchedule, v)
	}

	if *service.TerminationProtection != false {
		return fmt.Errorf("termination_protection: expected false, got true")
	}

	if !cmp.Equal(data.IpFilter, *service.IpFilter, cmpopts.EquateEmpty()) {
		return fmt.Errorf("pg.ip_filter: expected %q, got %q", data.IpFilter, *service.IpFilter)
	}

	if v := string(service.Maintenance.Dow); data.MaintenanceDow != v {
		return fmt.Errorf("pg.maintenance_dow: expected %q, got %q", data.MaintenanceDow, v)
	}

	if data.MaintenanceTime != service.Maintenance.Time {
		return fmt.Errorf("pg.maintenance_time: expected %q, got %q", data.MaintenanceTime, service.Maintenance.Time)
	}

	serviceMajVersion := strings.Split(*service.Version, ".")[0]

	if data.Version != serviceMajVersion {
		return fmt.Errorf("pg.version: expected %q, got %q", data.Version, serviceMajVersion)
	}

	//  NOTE: Due to default values setup by Aiven, we won't validate settings.

	return nil
}

func CheckExistsPgUser(service, username string, data *TemplateModelPgUser) error {

	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(service))
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
			serviceUsernames = append(serviceUsernames, u.Username)
			if u.Username == username {
				return nil
			}
		}
	}

	return fmt.Errorf("could not find user %s for service %s, found %v", username, service, serviceUsernames)
}

func CheckExistsPgDatabase(service, databaseName string, data *TemplateModelPgDb) error {

	client, err := testutils.APIClient()
	if err != nil {
		return err
	}

	ctx := exoapi.WithEndpoint(context.Background(), exoapi.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

	res, err := client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(service))
	if err != nil {
		return err
	}
	if res.StatusCode() != http.StatusOK {
		return fmt.Errorf("API request error: unexpected status %s", res.Status())
	}
	svc := res.JSON200

	serviceDbs := make([]string, 0)
	if svc.Databases != nil {
		for _, db := range *svc.Databases {
			serviceDbs = append(serviceDbs, string(db))
			if string(db) == databaseName {
				return nil
			}
		}
	}

	return fmt.Errorf("could not find database %s for service %s, found %v", databaseName, service, serviceDbs)
}
