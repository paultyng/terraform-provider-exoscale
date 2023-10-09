---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "exoscale_iam_role Resource - terraform-provider-exoscale"
subcategory: ""
description: |-
  Manage Exoscale IAM https://community.exoscale.com/documentation/iam/ Role.
---

# exoscale_iam_role (Resource)

Manage Exoscale [IAM](https://community.exoscale.com/documentation/iam/) Role.



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of IAM Role.

### Optional

- `description` (String) A free-form text describing the IAM Role
- `editable` (Boolean) Defines if IAM Role Policy is editable or not.
- `labels` (Map of String) IAM Role labels.
- `permissions` (List of String) IAM Role permissions.
- `policy` (Attributes) IAM Policy. (see [below for nested schema](#nestedatt--policy))
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- `id` (String) The ID of this resource.

<a id="nestedatt--policy"></a>
### Nested Schema for `policy`

Optional:

- `default_service_strategy` (String) Default service strategy (`allow` or `deny`).
- `services` (Attributes Map) IAM policy services. (see [below for nested schema](#nestedatt--policy--services))

<a id="nestedatt--policy--services"></a>
### Nested Schema for `policy.services`

Optional:

- `rules` (Attributes List) List of IAM service rules (if type is `rules`). (see [below for nested schema](#nestedatt--policy--services--rules))
- `type` (String) Service type (`rules`, `allow`, or `deny`).

<a id="nestedatt--policy--services--rules"></a>
### Nested Schema for `policy.services.rules`

Optional:

- `action` (String) IAM policy rule action (`allow` or `deny`).
- `expression` (String) IAM policy rule expression.
- `resources` (List of String) List of resources that IAM policy rule applies to.




<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `read` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Read operations occur during any refresh or planning operation when refresh is enabled.

-> The symbol ❗ in an attribute indicates that modifying it, will force the creation of a new resource.

