package database_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/exoscale/egoscale/v2/api"
	"github.com/exoscale/egoscale/v2/oapi"

	"github.com/exoscale/terraform-provider-exoscale/pkg/testutils"
)

func TestDatabase(t *testing.T) {
	t.Run("ResourcePg", testResourcePg)
	t.Run("ResourceMysql", testResourceMysql)
	t.Run("ResourceRedis", testResourceRedis)
	t.Run("ResourceKafka", testResourceKafka)
	t.Run("ResourceOpensearch", testResourceOpensearch)
	t.Run("ResourceGrafana", testResourceGrafana)
	t.Run("DataSourceURI", testDataSourceURI)
}

func CheckServiceDestroy(dbType, name string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		client, err := testutils.APIClient()
		if err != nil {
			return err
		}

		ctx := api.WithEndpoint(context.Background(), api.NewReqEndpoint(testutils.TestEnvironment(), testutils.TestZoneName))

		switch dbType {
		case "grafana":
			_, err = client.GetDbaasServiceGrafanaWithResponse(ctx, oapi.DbaasServiceName(name))
		case "kafka":
			_, err = client.GetDbaasServiceKafkaWithResponse(ctx, oapi.DbaasServiceName(name))
		case "mysql":
			_, err = client.GetDbaasServiceMysqlWithResponse(ctx, oapi.DbaasServiceName(name))
		case "pg":
			_, err = client.GetDbaasServicePgWithResponse(ctx, oapi.DbaasServiceName(name))
		case "redis":
			_, err = client.GetDbaasServiceRedisWithResponse(ctx, oapi.DbaasServiceName(name))
		case "opensearch":
			_, err = client.GetDbaasServiceOpensearchWithResponse(ctx, oapi.DbaasServiceName(name))
		default:
			return fmt.Errorf("unsupported database service type %q", dbType)
		}

		if err != nil {
			if errors.Is(err, api.ErrNotFound) {
				return nil
			}
			return err
		}

		return fmt.Errorf("database service %q not deleted", name)
	}
}
