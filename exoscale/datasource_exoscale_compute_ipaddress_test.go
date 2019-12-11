package exoscale

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccDatasourceComputeIPAddress(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
%s

data "exoscale_compute_ipaddress" "ip_address" {
  zone = "ch-gva-2"
}`, testAccIPAddressConfigCreate),
				ExpectError: regexp.MustCompile(`You must set at least one attribute "id", "ip_address" or "description"`),
			},
			{
				Config: fmt.Sprintf(`
%s

data "exoscale_compute_ipaddress" "ip_address" {
  zone = "ch-gva-2"
  id   = "${exoscale_ipaddress.eip.id}"
}`, testAccIPAddressConfigCreate),
				Check: resource.ComposeTestCheckFunc(
					testAccDatasourceComputeIPAddressAttributes(testAttrs{
						"description": ValidateString(testIPDescription1),
					}),
				),
			},
			{
				Config: fmt.Sprintf(`
%s

data "exoscale_compute_ipaddress" "ip_address" {
  zone        = "ch-gva-2"
  description = "${exoscale_ipaddress.eip.description}"
}`, testAccIPAddressConfigCreate),
				Check: resource.ComposeTestCheckFunc(
					testAccDatasourceComputeIPAddressAttributes(testAttrs{
						"description": ValidateString(testIPDescription1),
					}),
				),
			},
			{
				Config: fmt.Sprintf(`
%s

data "exoscale_compute_ipaddress" "ip_address" {
  zone       = "ch-gva-2"
  ip_address = "${exoscale_ipaddress.eip.ip_address}"
}`, testAccIPAddressConfigCreate),
				Check: resource.ComposeTestCheckFunc(
					testAccDatasourceComputeIPAddressAttributes(testAttrs{
						"description": ValidateString(testIPDescription1),
					}),
				),
			},
		},
	})
}

func testAccDatasourceComputeIPAddressAttributes(expected testAttrs) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "exoscale_compute_ipaddress" {
				continue
			}

			return checkResourceAttributes(expected, rs.Primary.Attributes)
		}

		return errors.New("compute_ipaddress datasource not found in the state")
	}
}
