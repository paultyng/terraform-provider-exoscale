package utils

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	exov2 "github.com/exoscale/egoscale/v2"
	exov3 "github.com/exoscale/egoscale/v3"

	"github.com/exoscale/terraform-provider-exoscale/pkg/config"
)

// ZonedStateContextFunc is an alternative resource importer function to be
// used for importing zone-local resources, where the resource ID is expected
// to be suffixed with "@ZONE" (e.g. "c01af84d-6ac6-4784-98bb-127c98be8258@ch-gva-2").
// Upon successful execution, the returned resource state contains the ID of the
// resource and the "zone" attribute set to the value parsed from the import ID.
func ZonedStateContextFunc(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	parts := strings.SplitN(d.Id(), "@", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf(`invalid ID %q, expected format "<ID>@<ZONE>"`, d.Id())
	}

	d.SetId(parts[0])

	if err := d.Set("zone", parts[1]); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}

type IDStringer interface {
	Id() string
}

func IDString(d IDStringer, name string) string {
	id := d.Id()
	if id == "" {
		id = "<new resource>"
	}

	return fmt.Sprintf("%s (ID = %s)", name, id)
}

// In returns true if v is found in list.
func In(list []string, v string) bool {
	for i := range list {
		if list[i] == v {
			return true
		}
	}

	return false
}

// DefaultString returns the value of the string pointer v if not nil, otherwise the default value specified.
func DefaultString(v *string, def string) string {
	if v != nil {
		return *v
	}

	return def
}

// DefaultInt64 returns the value of the int64 pointer v if not nil, otherwise the default value specified.
func DefaultInt64(v *int64, def int64) int64 {
	if v != nil {
		return *v
	}

	return def
}

// DefaultBool returns the value of the bool pointer v if not nil, otherwise the default value specified.
func DefaultBool(v *bool, def bool) bool {
	if v != nil {
		return *v
	}

	return def
}

// AddressToStringPtr returns a string representation of addr if not nil, otherwise nil.
func AddressToStringPtr(addr *net.IP) *string {
	if addr != nil {
		addrStr := addr.String()
		return &addrStr
	}

	return nil
}

// NonEmptyStringPtr returns a non-nil pointer to s if the string is not empty, otherwise nil.
func NonEmptyStringPtr(s string) *string {
	if s != "" {
		return &s
	}

	return nil
}

func SchemaSetToStringArray(set *schema.Set) []string {
	array := make([]string, set.Len())
	for i, group := range set.List() {
		array[i] = group.(string)
	}

	return array
}

func Unique(s []string) []string {
	inResult := map[string]struct{}{}
	var result []string
	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

// SuppressCaseFunc https://www.terraform.io/plugin/sdkv2/schemas/schema-behaviors#diffsuppressfunc
// Do no show case differences between state and resource
func SuppressCaseDiff(k, old, new string, d *schema.ResourceData) bool {
	return strings.EqualFold(old, new)
}

// EncodeUserData does compression and base64 encoding, used in resource_exoscale_compute[_instance[_pool]]
// returns (user_data, user_data_already_base64, error)
func EncodeUserData(userData string) (string, bool, error) {
	// template_cloudinit_config alows to gzip but not base64, prevent such case
	if len(userData) > 2 && userData[0] == '\x1f' && userData[1] == '\x8b' {
		return "", false, errors.New("user_data appears to be gzipped: it should be left raw, or also be base64 encoded")
	}

	// If user supplied data is already base64 encoded, do nothing.
	_, err := base64.StdEncoding.DecodeString(userData)
	if err == nil {
		return userData, true, nil
	}

	userDataBase64 := base64.StdEncoding.EncodeToString([]byte(userData))

	if len(userDataBase64) >= config.ComputeMaxUserDataLength {
		return "", false, fmt.Errorf("user-data maximum allowed length is %d bytes", config.ComputeMaxUserDataLength)
	}

	return userDataBase64, false, nil
}

// DecodeUserData does base64 decoding & decompression, used in resource_exoscale_compute[_instance[_pool]]
func DecodeUserData(data string) (string, error) {
	b64Decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	gz, err := gzip.NewReader(bytes.NewReader(b64Decoded))
	if err != nil {
		if errors.Is(err, gzip.ErrHeader) {
			// User data are not compressed, returning as-is.
			return string(b64Decoded), nil
		}

		return "", err
	}
	defer gz.Close()

	userData, err := io.ReadAll(gz)
	if err != nil {
		return "", err
	}

	return string(userData), nil
}

// ParseIAMAccessKeyResource parses IAM key format
func ParseIAMAccessKeyResource(v string) (*exov2.IAMAccessKeyResource, error) {
	var iamAccessKeyResource exov2.IAMAccessKeyResource

	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format")
	}
	iamAccessKeyResource.ResourceName = parts[1]

	parts = strings.SplitN(parts[0], "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid format")
	}
	iamAccessKeyResource.Domain = parts[0]
	iamAccessKeyResource.ResourceType = parts[1]

	if iamAccessKeyResource.Domain == "" ||
		iamAccessKeyResource.ResourceType == "" ||
		iamAccessKeyResource.ResourceName == "" {
		return nil, fmt.Errorf("invalid format")
	}

	return &iamAccessKeyResource, nil
}

// ValidateZone validates zone string.
func ValidateZone() schema.SchemaValidateDiagFunc {
	return validation.ToDiagFunc(validation.StringInSlice(config.Zones, false))
}

// ValidateComputeInstanceType validates that the given field contains a valid Exoscale Compute instance type.
func ValidateComputeInstanceType(v interface{}, _ cty.Path) diag.Diagnostics {
	value, ok := v.(string)
	if !ok {
		return diag.Errorf("expected field %q type to be string", v)
	}

	if !strings.Contains(value, ".") {
		return diag.Errorf(`invalid value %q, expected format "FAMILY.SIZE"`, value)
	}

	return nil
}

// ValidateComputeUserData validates that the given field contains a valid data.
func ValidateComputeUserData(v interface{}, _ cty.Path) diag.Diagnostics {
	value, ok := v.(string)
	if !ok {
		return diag.Errorf("expected field %q type to be string", v)
	}

	_, _, err := EncodeUserData(value)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// ValidateLowercaseString validates that the given fields contains only lowercase characters
func ValidateLowercaseString(val interface{}, key string) (warns []string, errs []error) {
	v := val.(string)
	if strings.ContainsAny(v, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		errs = append(errs, fmt.Errorf("%q must be lowercase, got: %q", key, v))
	}
	return
}

// SwitchClientZone clones the existing exoscale Client in the new zone.
func SwitchClientZone(ctx context.Context, client *exov3.Client, zone exov3.ZoneName) (*exov3.Client, error) {
	if zone == "" {
		return client, nil
	}
	endpoint, err := client.GetZoneAPIEndpoint(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("switch client zone v3: %w", err)
	}

	return client.WithEndpoint(endpoint), nil
}

// FindInstanceTypeByName copies the behaviour of egoscale v2's client.FindInstanceType
// but using the v3 api:
// FindInstanceType attempts to find an Instance type by family+size or ID.
// To search by family+size, the expected format for v is "[FAMILY.]SIZE" (e.g. "large", "gpu.medium"),
// with family defaulting to "standard" if not specified.
func FindInstanceTypeByNameV3(ctx context.Context, client *exov3.Client, id string) (*exov3.InstanceType, error) {

	var typeFamily, typeSize string

	parts := strings.SplitN(id, ".", 2)
	if l := len(parts); l > 0 {
		if l == 1 {
			typeFamily, typeSize = "standard", strings.ToLower(parts[0])
		} else {
			typeFamily, typeSize = strings.ToLower(parts[0]), strings.ToLower(parts[1])
		}
	}

	res, err := client.ListInstanceTypes(ctx)
	if err != nil {
		return nil, err
	}

	for _, r := range res.InstanceTypes {
		if string(r.Family) == typeFamily && string(r.Size) == typeSize {
			return client.GetInstanceType(ctx, r.ID)
		}
	}

	return nil, exov3.ErrNotFound
}

// In the context of converting list of IDs to resources for egoscale V3 update and
// creation requests, we're using a  lot of inline funcitons, increasing reading complexity
// when one is called more than once, we should aim to regroup them here.
func AntiAffiniGroupsToAntiAffinityGroupIDs(aags []exov3.AntiAffinityGroup) (ls []string) {
	ls = make([]string, len(aags))
	for i, aag := range aags {
		ls[i] = aag.ID.String()
	}
	return
}

func AntiAffinityGroupIDsToAntiAffinityGroups(ids []interface{}) (ls []exov3.AntiAffinityGroup) {
	ls = make([]exov3.AntiAffinityGroup, len(ids))
	for i, id := range ids {
		ls[i] = exov3.AntiAffinityGroup{
			ID: exov3.UUID(id.(string)),
		}
	}
	return
}

func PrivateNetworksToPrivateNetworkIDs(privnets []exov3.PrivateNetwork) (ls []string) {
	ls = make([]string, len(privnets))
	for i, aag := range privnets {
		ls[i] = aag.ID.String()
	}
	return
}

func PrivateNetworkIDsToPrivateNetworks(ids []interface{}) (ls []exov3.PrivateNetwork) {
	ls = make([]exov3.PrivateNetwork, len(ids))
	for i, id := range ids {
		ls[i] = exov3.PrivateNetwork{
			ID: exov3.UUID(id.(string)),
		}
	}
	return
}

func SecurityGroupsToSecurityGroupIDs(sgs []exov3.SecurityGroup) (ls []string) {
	ls = make([]string, len(sgs))
	for i, aag := range sgs {
		ls[i] = aag.ID.String()
	}
	return
}

func SecurityGroupIDsToSecurityGroups(ids []interface{}) (ls []exov3.SecurityGroup) {
	ls = make([]exov3.SecurityGroup, len(ids))
	for i, id := range ids {
		ls[i] = exov3.SecurityGroup{
			ID: exov3.UUID(id.(string)),
		}
	}
	return
}

func ElasticIPsToElasticIPIDs(eips []exov3.ElasticIP) (ls []string) {
	ls = make([]string, len(eips))
	for i, aag := range eips {
		ls[i] = aag.ID.String()
	}
	return
}

func ElasticIPIDsToElasticIPs(ids []interface{}) (ls []exov3.ElasticIP) {
	ls = make([]exov3.ElasticIP, len(ids))
	for i, id := range ids {
		ls[i] = exov3.ElasticIP{
			ID: exov3.UUID(id.(string)),
		}
	}
	return
}
