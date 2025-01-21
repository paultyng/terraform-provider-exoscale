# Changelog

## Unreleased

FEATURES:

- InstancePool: min-available support #406
- dbaas: support for users #401
- exoscale_instance: use egoscale v3 & support for multiple SSH Keys #408
- sks_nodepool: use egoscale v3 + add support for dual pane #409

BUG FIXES:

- Ignore block storage detach error when already detached #393
- Remove hardcoded timeout from db redis test #403

## 0.62.3

BUG FIXES:

- Fix typo that prevented running in preprod #400
- Don't validate addons attribute in SKS cluster resource test #399

## 0.62.2

BUG FIXES:

- Bump default timeout to 1h #398

## 0.62.1

BUG FIXES:

- Fix for allowing admin_username for dbaas #396

## 0.62.0

FEATURES:

- sos: introduce bucket policy resource #391

## 0.61.1

BUG FIXES:

- Add default mysql settings values in relevant acc test #394

IMPROVEMENTS:

- Fetching database credentials via dedicated API endpoints #395

## 0.61.0

FEATURES:

- database_uri: add URI parameters as attributes #387

IMPROVEMENTS:

- go.mk: upgrade to v2.0.3 #383
- Re-enable DNS service tests #384

BUG FIXES:

- Fix tests for database redis resource #388

## 0.60.0

FEATURES:

- exoscale_compute_instance: mac address attribute #373

BUG FIXES:

- fix: allow ICMP Code and Type -1 to 254 #372
- Bump kafka version used by tests #378

IMPROVEMENTS:

- Bump dependency google.golang.org/protobuf from 1.31.0 to 1.33.0 #370
- egoscale/v3: use separate module v3.1.0 #374
- Add example for sks nodepool kubelet_image_gc.min_age #377

## 0.59.2 (June 13, 2024)

BUG FIXES:

- Fix panic when ListSKSClusterVersions returns error #357
- Don't throw error if volume detach call fails due to volume not attached #367

IMPROVEMENTS:

- `exoscale_instance_pool`: add `anti-affinity-group` & deprecate `affinity-group` #355
- template: document ignore_changes #368

## 0.59.1 (June 3, 2024)

IMPROVEMENTS

- SKS: document dependency of CSI on CCM #359
- go.mk: lint with staticcheck #364

BUG FIXES

- Set zone when attaching blockstorage volume #362
- Fixes for acceptance tests issues #363

## 0.59.0 (May 13, 2024)

FEATURES:

- block-storage: update names and labels of volumes and snapshots #354

BUG FIXES:

- IAM: fix bug in datasouces by always init null values #353

## 0.58.0 (April 29, 2024)

FEATURES:

- Add Kubelet Image GC support for SKS nodepools
- Block storage volume resource & data source #341
- Block storage volume snapshot resource & data source #344
- sks_cluster: enable CSI addon on existing clusters #350

BUG FIXES:

- Fix dbaas bugs causing acceptance tests to fail #346
- docs: fix example in index.md #345
- Set labels on unmanaged eip creation #347

## 0.57.0 (April 3, 2024)

FEATURES:

- sks: CSI addon (#335)

IMPROVEMENTS:

- go.mk: remove submodule and initialize through make #338

## 0.56.0 (February 28, 2024)

FEATURES:

- compute_instance: add destroy_protected attr #337

IMPROVEMENTS:

- Add note about multiple ports rules in security group migration guide #333

## 0.55.0 (January 26, 2024)

IMPROVEMENTS:

- Bump golang.org/x/crypto from 0.14.0 to 0.17.0 (#323)
- sks_nodepool: add an example for taints #324
- SKS tests: renable cluster update test as upstream bug is fixed (#309)
- Make `iam_role.name` attribute require replace as per API behavior (#330)
- Handle DNS record normalization #332

BUG FIXES:

- instance: error when new disk_size < current one (#328)
- Bump dependencies (#329)
- database: change of plan should not recreate resource (#327)
- database: UseStateForUnknown for state and maintenance (#331)
- iam: fix bug in `iam_role` and `iam_org_policy` rules update (#330)

## 0.54.1 (December 6, 2023)

FEATURES:

- Add IAMv3 examples #316

BUG FIXES:

- iam_access_key: fix unexpected changes in operations field (#319)

DEPRECATIONS:

- IAM Policy rule resources field is deprecated in all resources #315

## 0.54.0 (November 23, 2023)

BREAKING CHANGES:

- Remove deprecated resources and datasources (APIv1) #285

IMPROVEMENTS:

- documentation: update SOS backend configuration demo #318

## 0.53.2 (November 15, 2023)

IMPROVEMENTS:

- sks_cluster: document possibility to use no CNI at all #313
- optimize exoscale_compute_instance_list #317

## 0.53.1 (October 26, 2023)

IMPROVEMENTS:

- Bump google.golang.org/grpc from 1.56.0 to 1.56.3 #312
- Bump golang.org/x/net from 0.11.0 to 0.17.0 #308

BUG FIXES:

- Fixed CI Badge in README.md (#306)
- Fixed database settings schema error when regex pattern cannot be compiled #310
- Replaced deprecated setting in database pg test #310

## 0.53.0 (October 6, 2023)

FEATURES:

- New resources and datasources: `exoscale_iam_api_key`, `exoscale_iam_role` and `exoscale_iam_org_policy`.

## 0.52.1 (September 15, 2023)

BUG FIXES:

- Add missing `vie2` zone in validator used by framework resources (#300)
- Update kafka version used in tests (#301)
- Temporarily disable SKS cluster update test (#302)

## 0.52.0 (September 8, 2023)

FEATURES:

- `exoscale_database` resource & `exoscale_database_uri` datasource: migrate to framework (#276)
- `exoscale_database` resource: add Grafana (#276)
- **New datasource** `exoscale_nlb_service_list` (#282)

BUG FIXES:

- `resource_exoscale_nlb_service`: unset uri and tls_sni when healthcheck mode is tcp (#295)
- `resource_exoscale_nlb_service`: force service re-creation when InstancePool ID is updated (#295)
- datasource `compute_instance`: labels and anti-affinity-groups are computed

IMPROVEMENTS:

- automate releases with a GitHub Action workflow

## 0.51.0 (August 9, 2023)

FEATURES:

- Datasource, resource `exoscale_private_network`: add labels (#281).

BREAKING CHANGES:

- user data are not gzipped anymore. If you want to gzip user data, you may want to use the cloudinit_config datasource.

IMPROVEMENTS:

- resource `sks_nodepool`: wait for nodepool and instancepool in running state in tests
- resource `sks_nodepool`: add **storage_lvm** addon

BUG FIX:

- datasource `exoscale_instance_pool_list`: fix panic when instance pool with labels is found

## 0.50.0 (June 23, 2023)

IMPROVEMENTS:

- security group: fix panic if security group has no ID in security group rule (#274)
- README: add a note about the terraform plugin framework (#273)
- docs: wrap ! symbol explanation in a box  (#271)

## 0.49.0 (June 7, 2023)

BREAKING CHANGES:

- Resource `exoscale_database` doesn't expose anymore the `uri` read-only property. This allows one to use this resource without being
forced to store this sensitive information in the Terraform state.

FEATURES:

- Datasource `exoscale_database_uri`: optionally expose an `exoscale_database` URI (which contains sensitive information).
- Datasource `exoscale_zones`.
- `resource_sks_cluster`: allow upgrades of SKS cluster.

IMPROVEMENTS:

- `resource_compute_instance`: read `network_interface.ip_address` from API when not set.
- go.mk: standardize CI with other Go repos

## 0.48.0 (May 12, 2023)

FEATURES:

- Resource `security_group_rule`: add the `public_security_group` attribute (#263)
- Resource `compute_instance`: support creating private instances (#262)

## 0.47.0 (April 24, 2023)

FEATURES:

- `Resource compute_instance`: add command to get password (#256)
- Add data sources for SKS clusters and nodepools (#245)

IMPROVEMENTS

- `Examples`: Use datasource exoscale_template (#250)
- `documentation`: generate docs from provider schemas (#248)
- `Move resources to a separate packages`: instance, instance_pool, anti_affinity_group (#246)
- Typo fixed in resNLBServiceAttrHealthcheckPort (#258)
- `README`: document how the documentation has to be regenerated by contributors (#259)

BUG FIX:

- Fix domain_record resource and datasource tests (#249)

## 0.46.0 (February 23, 2023)

FEATURES:

- `compute_instance_list`: allow filtering by string properties and labels (#241)

BUG FIX:

- `resource_database`: Make opensearch ip-filter a set to fix sorting issue (#244)

## 0.45.0 (February 13, 2023)

BUG FIX:

- `datasource_elastic_ip`: Fix label filtering (#242)

DEPRECATIONS:

- Removed nested imports feature from resources: `exoscale_domain` and `exoscale_security_group`

## 0.44.0 (January 31, 2023)

BUG FIX:

- `datasource_elastic_ip`: Labels are now correctly returned (#233)

FEATURES:

- `datasource_elastic_ip`: Added support for filtering by labels (#233)
- Resource `exoscale_elastic_ip`: healthcheck config could be updated (#237)

## 0.43.0 (January 6, 2023)

FEATURES:

- **Reverse DNS**: Support in `exoscale_compute_instance` and `exoscale_elastic_ip` resources (#234)
- **New datasource** `exoscale_template` (replaces compute_template) (#235)

BUG FIXES:
- `compute_instance`: fix anti_affinity_group_ids (ForceNew) (#231)

IMPROVEMENTS:
- Extends the acceptance tests timeout from 60 minutes to 90 minutes

## 0.42.0 (November 24, 2022)

FEATURES:

- `elastic_ip`: Added support for labels (#227)

## 0.41.1 (November 17, 2022)

BUG FIXES:

- `datasource_compute_instance_list`: add missing ID (#226)
- `resource_database`: always set `backup-schedule` on update to mitigate Aiven bug (#229)
- Fixed acceptance tests

IMPROVEMENTS:
- Documentation update

## 0.41.0 (September 20, 2022)

FEATURES:

- `elastic_ip`: Added support for EIPv6 (#211)

## 0.40.2 (September 14, 2022)

BUG FIXES:

- `resource_compute_instance`: fix instance restart after change (#220)

IMPROVEMENTS:

- Use HTTP client with retry logic (#216)
- Use recommended tflog library for logging (#214)

CHANGES:

- `resource_compute_instance`: force replacement when `deploy_target_id` is updated

## 0.40.1 (September 2, 2022)

BUG FIXES:

- resource_database_mysql/pg: fix backup schedule update bug (#212).
- domain/domain_record: use environment config (#208).

CHANGES:

- Instance pool acc test disabled temporarily (#213).

## 0.40.0 (July 27, 2022)

FEATURES:

- `sks_cluster`: new `aggregation_ca`, `control_plane_ca`, and `kubelet_ca` exported attributes (#201).

IMPROVEMENTS:

- docs: global overhaul and removal of deprecated examples.

## 0.39.1 (July 20, 2022)

BUG FIXES:

- `resource_domain_record` fix default value for ttl/prio

## 0.39.0 (July 19, 2022)

IMPROVEMENTS:

- docs: exoscale_ssh_keypair -> exoscale_ssh_key migration guide (#197)
- docs: added note about SOS usage (#191)

CHANGES:

- dns resources now use API v2 (#186)

BUG FIXES:

- `exoscale_iam_access_key` fix failures when resources are specified (#194)
- `resource_database_kafka` update kafka version used in tests (#193)

## 0.38.0 (June 23, 2022)

FEATURES:

- **New Resource:** `exoscale_iam_access_key` (#182)

BUG FIXES:

- API signature bug fixed upgrading `egoscale` to v0.88.1 (#184)

IMPROVEMENTS:

- Acceptance tests not relying anymore on harcoded template IDs (#185)

## 0.37.1 (June 14, 2022)

BUG FIXES:
- `database` fix infinite version attribute update (#181)

## 0.37.0 (June 1, 2022)

FEATURES:

- **New Data Source:** `exoscale_compute_instance_list`
- **New Data Source:** `exoscale_instance_pool`, `exoscale_instance_pool_list`

## 0.36.0 (May 6, 2022)

FEATURES:

- add opensearch support for `exoscale_database`

## 0.35.0 (April 20, 2022)

FEATURES:

- `exoscale_instance_pool`: new `instances` exported attribute (exports not only instances IDs but also IP addresses and names)

DEPRECATIONS:

- `exoscale_instance_pool`: `virtual_machines` exported attribute is deprecated in favor of the `instances` exported attribute

BUG FIXES:

- `exoscale_compute`, `exoscale_compute_instance` and `exoscale_instance_pool`: `user_data` argument length is now checked at plan time rather than on apply (#167)
- `exoscale_instance_pool`: wait the right ammount of instances are provisioned when creating or updating this resource (#168)

## 0.34.0 (March 29, 2022)

DEPRECATIONS:

- `exoscale_compute_instance`: the `private_network_ids` argument has been deprecated and is now read-only. Use `network_interface` blocks instead

BUG FIXES:

- `exoscale_compute_instance` data source crash when the instance belongs to an instance pool or an SKS node pool (#162)

## 0.33.1 (March 15, 2022)

BUG FIXES:

- `exoscale_compute_instance`: ignore case differences for `instance-type` (#161)
- `exoscale_instance_pool`: ignore case differences for `instance-type` (#161)
- `exoscale_sks_nodepool`: ignore case differences for `instance-type` (#161)
- `exoscale_security_group_rule`: ignore case differences for `protocol` (#161)
- `exoscale_security_group_rule`: validate `cidr` or `user_security_group` or `user_security_group_id` is supplied (#160)

## 0.33.0 (March 11, 2022)

FEATURES:

- **New Resource:** `exoscale_sks_kubeconfig`

BUG FIXES:

- `database`: fix cidr blocks filtering for `ip_filter` attributes.

## 0.32.0 (February 28, 2022)

BUG FIXES:

- `compute_instance`: fix bug caused by the new API returning lowercase names, when referencing security_groups by mixed-case names. (#149)
- `security_group_rules`: fix bug caused by the new API returning lowercase names, when user_security_group_list contains mixed-case names. (#149)
- `security_group_rules`: fix bug with protocols without ports. (#145)
- `security_group`: fix resource import along with associated `security_group_rule` resources. (#149)
- tests: fix DBaaS plan (hobbyist-1 is no longer available).
- doc: fix some broken links.

DEPRECATIONS:

- `security_group_rules`: now deprecated in favor of `security_group_rule` (added a migration guide in the documentation).

## 0.31.2 (December 21, 2021)

BUG FIXES:

- `security_group`: fix bug caused by the new API now returning lowercase names
- `security_group_rules`: fix bug caused by the new API not accepting `start_port = 0` anymore.

## 0.31.1 (December 15, 2021)

BUG FIXES:

- `exoscale_database`: fix bug causing `json: cannot unmarshal string into Go struct field .connection-info.slave of type map[string]interface {}` error


## 0.31.0 (December 15, 2021)

FEATURES:

- **New Data Source:** `exoscale_anti_affinity_group`
- **New Data Source:** `exoscale_compute_instance`
- **New Data Source:** `exoscale_elastic_ip`
- **New Data Source:** `exoscale_private_network`
- **New Resource:** `exoscale_anti_affinity_group`
- **New Resource:** `exoscale_compute_instance`
- **New Resource:** `exoscale_elastic_ip`
- **New Resource:** `exoscale_private_network`
- **New Resource:** `exoscale_ssh_key`

IMPROVEMENTS:

- `exoscale_security_group`: add support for external sources
- `sks_nodepool`: add support for K8s taints
- `sks_cluster`: add support for OIDC configuration


## 0.30.1 (November 15, 2021)

BUG FIXES:

- Fix Exoscale API errors related to resources sending empty strings


## 0.30.0 (October 25, 2021)

CHANGES:

- The `exoscale_database` resource has been overhauled, and now requires type-specific parameters to be specified in a dedicated block. See documentation for more information.


## 0.29.0 (September 9, 2021)

IMPROVEMENTS:

- `exoscale_instance_pool`: add support for labels
- `exoscale_nlb`: add support for labels
- `exoscale_sks_cluster`/`exoscale_sks_nodepool`: add support for labels


## 0.28.0 (August 18, 2021)

CHANGES:

- `exoscale_sks_nodepool`: the `instance_type` parameter now expects a `FAMILY.SIZE` format (e.g. `standard.small`, `memory.huge`...). Previous size-only values (e.g. `small`, `medium` etc.) must now be prefixed with `standard.`.


## 0.27.0 (August 17, 2021)

DEPRECATIONS:

- `exoscale_instance_pool`: the `service_offering` parameter is deprecated and replaced by `instance_type`


## 0.26.0 (August 12, 2021)

FEATURES:

- **New Resource:** `exoscale_database` (BETA)

IMPROVEMENTS:

- `exoscale_sks_nodepool`: add support for Private Networks (#114)
- `exoscale_sks_cluster`: add support for auto-upgrades
- Upgrade to Terraform SDK v2


## 0.25.0 (July 2, 2021)

IMPROVEMENTS:

- `exoscale_sks_nodepool`: add support for Deploy Target/Instance Prefix
- `exoscale_compute`/`exoscale_instance_pool`: improve cloud-init userdata handling

BUG FIXES:

- `exoscale_security_group_rule*`: support -1 value for `icmp_(code|type)`
- Fix non-existence detection logic for NLB service/SKS Nodepool


## 0.24.0 (May 11, 2021)

DEPRECATIONS:

- `exoscale_sks_cluster`: the `addons` parameter is deprecated and replaced by `exoscale_ccm`/`metrics_server`

CHANGES:

- `exoscale_sks_cluster`: use latest available version advertised by the API by default

IMPROVEMENTS:

- `exoscale_instance_pool`: add support for Deploy Targets
- `exoscale_instance_pool`: add support for instance prefix
- `exoscale_instance_pool`: add support for `ipv6` attribute resetting


## 0.23.0 (March 19, 2021)

IMPROVEMENTS:

- `exoscale_sks_cluster`/`exoscale_sks_nodepool`: add support for field resetting
- `exoscale_ipaddress`: add support for reverse DNS ([#97](https://github.com/exoscale/terraform-provider-exoscale/pull/97))
- `exoscale_instance_pool`: add support for Elastic IPs ([#95](https://github.com/exoscale/terraform-provider-exoscale/pull/95))
- `sks_nodepool`: add support for Security Groups/Anti-Affinity Groups updating ([#92](https://github.com/exoscale/terraform-provider-exoscale/pull/92))

BUG FIXES:

- Fix "Error: dns error: Record not found" ([#94](https://github.com/exoscale/terraform-provider-exoscale/pull/94))


## 0.22.0 (February 15, 2021)

FEATURES:

- **New Resources:** `exoscale_sks_cluster`/`exoscale_sks_nodepool`


## 0.21.1 (January 18, 2021)

IMPROVEMENTS:

- Updating a `exoscale_compute` resource's `security_groups`/`security_group_ids` attributes no longer reboots the related Compute instance


## 0.21.0 (December 21, 2020)

FEATURES:

- **New Data Source:** `exoscale_nlb` ([#85](https://github.com/exoscale/terraform-provider-exoscale/pull/85))

IMPROVEMENTS:

- The `instance_pool` resource now supports Anti-Affinity Groups

BUG FIXES:

- Fix client API request tracing
- Fix non-existing resource error method


## 0.20.0 (September 22, 2020)

IMPROVEMENTS:

- The `exoscale_nlb_service` resource now supports HTTPS health checking ([#71](https://github.com/exoscale/terraform-provider-exoscale/pull/71))
- `exoscale_security_group_rule*`: providing a port is no longer necessary for protocols AH, ESP, GRE and IPIP ([#78](https://github.com/exoscale/terraform-provider-exoscale/pull/78))

BUG FIXES:

- `exoscale_instance_pool`: improved non-existent Instance Pool handling ([#74](https://github.com/exoscale/terraform-provider-exoscale/pull/74))
- `exoscale_nlb`: improved non-existent NLB handling ([#75](https://github.com/exoscale/terraform-provider-exoscale/pull/75))
- `exoscale_network`: improved non-existent Private Network handling ([#77](https://github.com/exoscale/terraform-provider-exoscale/pull/77))


## 0.19.0 (September 2, 2020)

IMPROVEMENTS:

- The `exoscale_ipaddress` resource now supports HTTPS health checking ([#66](https://github.com/exoscale/terraform-provider-exoscale/issues/66))
- The `exoscale_instance_pool` resource now supports IPv6 ([#68](https://github.com/exoscale/terraform-provider-exoscale/issues/68))
- The `exoscale_instance_pool` resource now supports in-place `disk_size` update ([#70](https://github.com/exoscale/terraform-provider-exoscale/issues/70))

BUG FIXES:

- Fix the `exoscale_security_group_rule` resource documentation about conflicting parameters ([#67](https://github.com/exoscale/terraform-provider-exoscale/issues/67))

CHANGES:

- The `exoscale_compute_template` data source now returns the most recent result found instead of an error if multiple templates match a same name ([#63](https://github.com/exoscale/terraform-provider-exoscale/issues/63))


## 0.18.2 (July 22, 2020)

BUG FIXES:

- Fixed Go module path following repository migration from github.com/terraform-providers


## 0.18.1 (July 22, 2020)

BUG FIXES:

- Fixed GoReleaser build configuration


## 0.18.0 (July 22, 2020)

FEATURES:

- **New Data Source:** `exoscale_affinity` ([#58](https://github.com/exoscale/terraform-provider-exoscale/issues/58))
- **New Data Source:** `exoscale_security_group` ([#59](https://github.com/exoscale/terraform-provider-exoscale/issues/59))
- **New Data Source:** `exoscale_network` ([#60](https://github.com/exoscale/terraform-provider-exoscale/issues/60))
- The `exoscale_compute` resource now supports a new `reverse_dns` attribute ([#56](https://github.com/exoscale/terraform-provider-exoscale/issues/56))


## 0.17.1 (June 22, 2020)

BUG FIXES:

- Updated egoscale library following API changes


## 0.17.0 (June 17, 2020)

- **New Resources:** `exoscale_nlb`/`exoscale_nlb_service` ([#52](https://github.com/exoscale/terraform-provider-exoscale/issues/52))

BUG FIXES:

- Fix the `exoscale_instance_pool` resource `virtual_machines` attribute ([#53](https://github.com/exoscale/terraform-provider-exoscale/issues/53))

IMPROVEMENTS:

- Various documentation updates and corrections


## 0.16.2 (April 10, 2020)

BUG FIXES:

- Fix the `exoscale_ssh_keypair` resource ([#50](https://github.com/exoscale/terraform-provider-exoscale/issues/50)), which `private_key` attribute was not set after requesting an SSH key pair creation by the API.


## 0.16.1 (February 11, 2020)

BUG FIXES:

- Fix the `exoscale_network` resource import method ([#46](https://github.com/exoscale/terraform-provider-exoscale/issues/46))


## 0.16.0 (January 22, 2020)

FEATURES:

- **New Data Source:** `exoscale_compute` ([#42](https://github.com/exoscale/terraform-provider-exoscale/issues/42))
- **New Data Source:** `exoscale_compute_ipaddress` ([#31](https://github.com/exoscale/terraform-provider-exoscale/issues/31))
- **New Data Source:** `exoscale_domain` ([#34](https://github.com/exoscale/terraform-provider-exoscale/issues/34))
- **New Data Source:** `exoscale_domain_record` ([#33](https://github.com/exoscale/terraform-provider-exoscale/issues/33))

CHANGES:

- The `exoscale_compute` resource `key_pair` argument is now optional ([#38](https://github.com/exoscale/terraform-provider-exoscale/issues/38))

IMPROVEMENTS:

- Acceptance tests refactoring ([#35](https://github.com/exoscale/terraform-provider-exoscale/issues/35))
- Fix configuration examples syntax ([#39](https://github.com/exoscale/terraform-provider-exoscale/issues/39))

DEPRECATIONS:

- The `exoscale_compute` resource `name` attribute is now deprecated, replaced by the new `hostname` attribute ([#44](https://github.com/exoscale/terraform-provider-exoscale/issues/44))


## 0.15.0 (December 12, 2019)

FEATURES:

- **New Resource:** `exoscale_instance_pool` ([#11](https://github.com/exoscale/terraform-provider-exoscale/issues/11))

CHANGES:

- The `exoscale_network` resource `network_offering` attribute is now deprecated ([#26](https://github.com/exoscale/terraform-provider-exoscale/issues/26))


## 0.14.0 (December 02, 2019)

FEATURES:

- The `exoscale_ipaddress` resource now supports a `description` attribute ([#18](https://github.com/exoscale/terraform-provider-exoscale/issues/18))

BUG FIXES:

- Fix the `exoscale_compute` resource import method when importing a Compute instance with secondary IP addresses attached ([#23](https://github.com/exoscale/terraform-provider-exoscale/issues/23))
- Fix the `exoscale_ipaddress` resource import method by IP address ([#24](https://github.com/exoscale/terraform-provider-exoscale/issues/24))


## 0.13.2 (November 07, 2019)

BUG FIXES:

- Fix the `exoscale_compute` resource import method ([#20](https://github.com/exoscale/terraform-provider-exoscale/issues/20))


## 0.13.1 (November 05, 2019)

BUG FIXES:

- Fix the `exoscale_domain_record` resource import method ([#12](https://github.com/exoscale/terraform-provider-exoscale/issues/12))

IMPROVEMENTS:

- Add provider version to HTTP client User-Agent ([#16](https://github.com/exoscale/terraform-provider-exoscale/issues/16))
- Prevent state changes when a `compute` resource is temporarily being migrated during a plan refresh ([#17](https://github.com/exoscale/terraform-provider-exoscale/issues/17))

CHANGES:

- The `exoscale_compute` *template* attribute deprecated in version 0.13.0 has been reinstated ([#15](https://github.com/exoscale/terraform-provider-exoscale/issues/15)). Both `template` and `template_id` are exclusive, and referencing custom templates require the use of the *template_id* attribute with the `exoscale_compute_template` data source.


## 0.13.0 (October 15, 2019)

DEPRECATIONS:

- The `exoscale_compute` *template* attribute is now deprecated, replaced by `template_id`. See resource documentation for details ([#9](https://github.com/exoscale/terraform-provider-exoscale/issues/9))
- The `exoscale_compute` *username* attribute is now deprecated, users wanting to use the *remote-exec* provisioner should now rely on the *exoscale_compute_template* data source `username` attribute. See resource documentation for details ([#9](https://github.com/exoscale/terraform-provider-exoscale/issues/9))

IMPROVEMENTS:

- Various documentation improvements ([#4](https://github.com/exoscale/terraform-provider-exoscale/issues/4), [#7](https://github.com/exoscale/terraform-provider-exoscale/issues/7))

CHANGES:

- Switch to the Terraform Plugin SDK ([#5](https://github.com/exoscale/terraform-provider-exoscale/issues/5))
- Switch the HTTP client to [go-cleanhttp](https://github.com/hashicorp/go-cleanhttp) ([#10](https://github.com/exoscale/terraform-provider-exoscale/issues/10))


## 0.12.1 (August 26, 2019)

IMPROVEMENTS:

- Improve exoscale_network resource API call resiliency ([#2](https://github.com/exoscale/terraform-provider-exoscale/issues/2))

CHANGES:

- mod: update egoscale to 0.18.1
- mod: update Terraform SDK to 0.12.6


## 0.12.0 (August 12, 2019)

CHANGES:

- Internal refactoring requested by HashiCorp during provider review (#228)
- mod: update Terraform SDK to 0.12.1


## 0.11.0 (May 23, 2019)

FEATURES:

- **New Data Source:** `exoscale_compute_template` (#231)

IMPROVEMENTS:

- Add support for *managed* Elastic IP to the `exoscale_ipaddress` resource

CHANGES:

- `start_ip`/`end_ip`/`netmask` attributes are now required for *managed* Private Networks
- `affinity_groups`/`affinity_group_ids` attributes change now force a `exoscale_compute` resource to be re-created


## 0.10.0 (March 6, 2019)

- dep: playing with terraform v0.12.0-beta1 (#200)


## 0.10.0-beta1 (March 4, 2019)

- examples: fix syntax
- terraform 0.12-beta1


## 0.9.46 (March 4, 2019)

- affinity: fix virtual machine ids (#220)


## 0.9.45 (February 28, 2019)

- dep: egoscale v0.14.3 (#219)
- rules: fix egress update (#218)
- examples: k8s using kubeadm (#67)
- Ignore drift for object_lock_configuration (#216)
- website: build locally using middleman (#214)


## 0.9.44 (February 21, 2019)

- Add CAA to domainRecordResource (#215)


## 0.9.43 (February 12, 2019)

- security group: fix import (#212)


## 0.9.42 (February 8, 2019)

- rules: allow creating a batch of ingress/egress rules (#199)
- mod: pretend this project is already part of terraform-providers (#209)


## 0.9.41 (January 10, 2019)

- compute: keep base64 encoded user_data as is (#206)
- project: upgrade terraform v0.11.11 (#204)
- examples: add managed privnet (#203)


## 0.9.40 (December 12, 2018)

- test: fix the acceptance tests
- exoscale: adapt to library changes
- vendor: bump libraries


## 0.9.39 (November 19, 2018)

- dns record: fail (#202)


## 0.9.38 (November 13, 2018)

- secondary ip: fix id (#201)
- no domain (#198)


## 0.9.37.1 (November 2, 2018)

- network: remove cidr (#197)


## 0.9.37 (November 2, 2018)

This release features the managed privnet (DHCP) capabilities, only in the `ch-gva-2` zone for the time being.

- travis: copy AWS provider travis setup (#193)
- managed privnet: the code (#186)
- sg rule: import using only the ruleid (#190)
- security group rule: add IPIP (#191)
- Fix README's reference to the CloudStack configuration file (#189)
- security group: no more tags (#180)


## 0.9.36 (August 31, 2018)

- compute: fix #181
- examples: remove tags on security groups (#178)


## 0.9.35 (August 29, 2018)

- update deps (#177)
- Dep updates (#175)


## 0.9.34 (August 16, 2018)

- egoscale v0.11 (#173)
- tests: Arftul is no more (#172)


## 0.9.33 (August 3, 2018)

- provider: http traces (#170)
- Update ego (#171)
- dep: update egoscale (and others) (#168)


## 0.9.32 (July 19, 2018)

- dep: bump go-ini version
- validation: adding tests (and fixing bugs) (#162)
- security group: test updating the tags (#165)
- test: updating compute instance (#166)
- compute: don't udpate size if they virtually are the same (#164)
- security group rule: add acceptance test (#159)
- domain record: add acceptance test (#161)
- Add port 10250 as it is prerequesite (#160)


## 0.9.31 (July 17, 2018)

- compute: less validation to enable GPU SO (#157)
- secondary ip: add acceptance test (#156)
- provider: more envs (#155)
- nic: add acceptance test (#154)
- Network acc (#153)
- deps: use less-types branch (#121)


## 0.9.30 (July 6, 2018)

- dep: update go-ini to 1.37
- domain: try to not erase things (#150)
- secondary ipaddress: fix import (#149)
- dep: upgrade egoscale to 0.9.31
- global: bump default timeout to 5m (#152)
- travis: run acceptance test on travis (#148)


## 0.9.29 (June 28, 2018)

- dep: bump egoscale to 0.9.30 (#146)
- examples: creating a bucket using aws provider (#104)


## 0.9.28 (June 26, 2018)

- import DNS record (#144)
- provider: when cloudstack.ini is used, build dns_endpoint (#128)
- dep: ensure -update


## 0.9.27 (June 21, 2018)

- Possibility to disable gzipping user-data (#142)


## 0.9.26 (June 20, 2018)

- `security_group_rule`: handle gone security group (#141)


## 0.9.25 (June 19, 2018)

- put hashicorp's gitignore
- secondary ip: create a compound id instead of the cs id (#126)
- fixup! global: use hashi's scripts
- readme: use hashi's readme (#140)
- license: change to MPL2 (#139)
- global: add changelog for hashi'
- global: use hashi's scripts
- global: rename files according hashi's conventions
- examples: add RKE example
- compute: use Details to activate ipv6
- Update ipaddress.html.markdown


## 0.9.24 (June 4, 2018)

- provider: better error message
- provider: fix another nil
- goreleaser: disable CGO


## 0.9.23 (June 4, 2018)

- goreleaser: fix binary name
- fix: nil pointer check
- deps: upgrade egoscale to 0.9.27
- build: cleanup makefile
- build. use goreleaser
- tests: add acceptance test for DNS domain
- examples: add DNS example


## 0.9.22 (May 15, 2018)

- `exoscale_compute.user_data` is know read from the external resource
- allow `ALL` protocol rule
- documentation fixes, thanks to @mcorbin (#116)
- upgrade egoscale to 0.9.25


## 0.9.21 (April 27, 2018)

IMPROVEMENTS:

- Upgrade egoscale to 0.9.22
- Upgrade terraform to 0.11.7

BUG FIXES:

- Fix example in documentation


## 0.9.20 (April 20, 2018)

IMPROVEMENTS:

- Allow `user_data` to be updated, #113
- Upgrade egoscale to 0.9.21


## 0.9.19 (April 13, 2018)

IMPROVEMENTS:

- Read SSH `username` from the template details (#111)
- Upgraded egoscale to 0.9.20


## 0.9.18 (March 28, 2018)

BUG FIXES:

- `compute` resource `ipv6` attribute wasn't properly set (#107)


## 0.9.17 (March 27, 2018)

IMPROVEMENTS:

- Upgrade egoscale version


## 0.9.16 (March 27, 2018)

BUG FIXES:

- `security_group_rule` may start at zero
- Compute `state="Stopped"` wasn't applied


## 0.9.15 (March 23, 2018)

IMPROVEMENTS:

- IPv6 for `compute` resources
- `ICMPv6` and `/128` CIDR for `security_group_rule` resources

BUG FIXES:

- fix: tags weren't set after creation
- fix: crash during import (nil pointer)


## 0.9.14 (March 20, 2018)

IMPROVEMENTS:

- A SSH key pair may be created
- Support timeouts on every call
- `compute` resource can retrieve the password and encrypted password

BUG FIXES:

- Error message on 40x responses


## 0.9.13 (March 13, 2018)

DEPRECATIONS:

- Use `key` instead of `token`

IMPROVEMENTS:

- `exoscale_compute` has separate `affinity_groups`/`affinity_group_names` and `security_groups`/`security_group_ids`

BUG FIXES:

- Handle missing Elastic IP when doing the import


## 0.9.12 (March 2, 2018)

IMPROVEMENTS:

- `exoscale_domain_record` offers a `hostname` field. Handy for `CNAME` records.


## 0.9.11 (March 2, 2018)

IMPROVEMENTS:

- A `compute` resource can be deployed without being started (#68)

BUG FIXES:

- Less `<nil>` values in `nic` and `network` resources
- `user_data` and `key_pair` force the creation of a new `compute` resource
- The `~/.cloudstack.ini` file is read by default


## 0.9.10 (March 1, 2018)

IMPROVEMENTS:

- Importing a `compute` resource will also import any `secondary_ipaddress` resource linked to it

CHANGES:

- Separate `user_security_group` and `user_security_group_id` within a `security_group_rule`

BUG FIXES:

- Importing a missing network failed silently
- Auto fill the `security_groups` and `affinity_groups` of a `compute` resource


## 0.9.9 (February 1, 2018)

BUG FIXES:

- Importing a missing compute fails
- Secret/token conflicts with config/provider
- Updating a compute crashes
- Network refreshes the tags


## 0.9.8 (February 1, 2018)

IMPROVEMENTS:

- A `compute` resource can be imported by its name
- `cloudstack.ini` files are supported
- Tags are supported on Security Groups, Networks and Elastic IP


## 0.9.7 (January 29, 2018)

BUG FIXES:

- _nil pointer_ error when working on missing resources
- `user_data` is auto-magically encoded in base64 without having to use `template_cloudinit_config`


## 0.9.6 (January 22, 2018)

IMPROVEMENTS:

- A Security Group may be imported using its name as well as its ID

BUG FIXES:

- Global variables documentation didn't match the actual code (#49)
- Domain record missing content field (#50)
- Importing security group rule misses `user_security_group` key (#51)


## 0.9.5 (January 19, 2018)

IMPROVEMENTS:

- Updated egoscale
- Added example cloud-init multi-part setup

BUG FIXES:

- bug fix `IP` addresses


## 0.9.4 (January 18, 2018)

FEATURES:

- `exoscale_network` and `exoscale_nic` for [multiple private networks](https://www.exoscale.ch/syslog/2018/01/17/introducing-multiple-private-networks/)

IMPROVEMENTS:

- Examples are fresh
- Using godep for managing dependencies


## 0.9.3 (January 15, 2018)

IMPROVEMENTS:

- `exoscale_affinity_group` shows the list of machines that are part of the group

BUG FIXES:

- Refreshing a resource that was deleted via the console
- Security Group rule `cidr` key


## 0.9.2 (January 11, 2018)

FEATURES:

- **New Resource:** `exoscale_domain_record`
- **New Resource:** `exoscale_security_group_rule`
- **New Resource:** `exoscale_ipaddress`

IMPROVEMENTS:

- `exoscale_compute` with import
- `exoscale_ssh_keypair` with import
- `exoscale_affinity` with import
- `exoscale_domain` with import containing the DNS records
- `exoscale_security_group` with import containing the _Security Group_ rules
- `exoscale_secondary_ipaddress` associate a compute and an elastic IP address

NOTES:

The following features are missing/unstable:

- Tags only on `compute`
- S3 bucket and objects

BREAKING CHANGES:

This version is mostly not backward compatible with the previous release of the provider.


## 0.1.0 (December 11, 2017)

First release
