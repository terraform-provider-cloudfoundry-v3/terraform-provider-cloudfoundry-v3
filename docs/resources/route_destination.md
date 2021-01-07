---
layout: "cloudfoundry"
page_title: "Cloud Foundry: cloudfoundry_route_destination"
sidebar_current: "docs-cf-resource-route-destination"
description: |-
  Provides a Cloud Foundry Route Destination resource.
---

# cloudfoundry_route_destination

Provides the mapping between a `route` and `app` resource so that traffic
is routed to the application process from the route.

## Example Usage

The following example creates an application with a route and maps the route

```hcl

data "cloudfoundry_domain" "foo" {
  name = "apps.internal"
}

resource "cloudfoundry_app" "foo" {
	name = "foo-with-route"
	# ...
}

resource "cloudfoundry_route" "foo" {
	domain_id = data.cloudfoundry_domain.foo.id
	space_id = "xxxx"
	host = "basic-test-route"
}

resource "cloudfoundry_route_destination" "foo" {
	route_id = cloudfoundry_route.foo.id
	app_id = cloudfoundry_app.foo.id
}
```

## Argument Reference

The following arguments are supported:

* `app_id` - (Required) The GUID of the associated Cloud Foundry application.
* `route_id` - (Required) The GUID of the associated Cloud Foundry route.

## Attributes Reference

The following attributes are exported along with any defaults for the inputs attributes.

* `id` - The GUID of the application
