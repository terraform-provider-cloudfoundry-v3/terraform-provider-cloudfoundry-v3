---
layout: "cloudfoundry"
page_title: "Cloud Foundry: cloudfoundry_route"
sidebar_current: "docs-cf-resource-route"
description: |-
  Provides a Cloud Foundry route resource.
---

# cloudfoundry_route

Provides a Cloud Foundry resource for managing Cloud Foundry application [routes](https://docs.cloudfoundry.org/devguide/deploy-apps/routes-domains.html).

## Example Usage

The following example creates an route for an application.

```hcl
resource "cloudfoundry_route" "default" {
    domain_id = data.cloudfoundry_domain.apps.domain.id
    space_id = data.cloudfoundry_space.dev.id
    host = "myapp"
}
```

## Argument Reference

The following arguments are supported:

- `domain_id` - (Required, String) The ID of the domain to map the host name to. If not provided the default application domain will be used.
- `space_id` - (Required, String) The ID of the space to create the route in.
- `host` - (Required, Optional) The application's host name. This is required for shared domains.
- `path` - (Optional) A path for a HTTP route.

The following maps the route to an application.

- `target` - (Optional, Set) One or more route mapping(s) that will map this route to application(s). Can be repeated multiple times to load balance route traffic among multiple applications.<br/>
The `target` block supports:
  - `app` - (Required, String) The ID of the [application](/docs/providers/cloudfoundry/r/app.html) to map this route to.
  - `port` - (Optional, Int) A port that the application will be listening on. If this argument is not provided then the route will be associated with the application's default port.

~> **NOTE:** Route mappings can be controlled from either the `cloudfoundry_routes.target` or the `cloudfoundry_app.routes` attributes.
~> **NOTE:** Resource only handles `target` previously created by resource (i.e. it does not destroy nor modifies target set by other resources like cloudfoundry_application).

## Attributes Reference

The following attributes are exported along with any defaults for the inputs attributes.

* `id` - The GUID of the route
* `endpoint` - The complete endpoint with path if set for the route

## Import

The current Route can be imported using the `route`, e.g.

```bash
$ terraform import cloudfoundry_route.default a-guid
```
