---
layout: "cloudfoundry"
page_title: "Cloud Foundry: cloudfoundry_droplet"
sidebar_current: "docs-cf-resource-droplet"
description: |-
  Provides a Cloud Foundry Droplet resource.
---

# cloudfoundry_droplet

Provides a Cloud Foundry [application](https://docs.cloudfoundry.org/devguide/deploy-apps/deploy-app.html) resource.

## Example Usage

The following example creates a droplet (package of your built/staged application.

```hcl
resource "cloudfoundry_app" "basic" {
	provider              = cloudfoundry-v3
	name                  = "basic-buildpack"
	space_id              = data.cloudfoundry_space.myspace.id
	environment           = {MY_VAR = "1"}
	instances             = 2
	memory_in_mb          = 1024
	disk_in_mb            = 1024
	health_check_type     = "http"
	health_check_endpoint = "/"
}

resource "cloudfoundry_droplet" "basic" {
	provider         = cloudfoundry-v3
	app_id           = cloudfoundry_app.basic.id
	buildpacks       = ["binary_buildpack"]
	environment      = cloudfoundry_app.basic.environment
	command          = cloudfoundry_app.basic.command
	source_code_path = "/path/to/source.zip"
	source_code_hash = filemd5("/path/to/source.zip")
}
```

## Argument Reference

The following arguments are supported:

* `app_id` - (Required) The GUID of the associated Cloud Foundry application
* `type` - (Optional, String) The lifecycle type of the source. Should match that set in the associated `cloudfoundry_app`. For `buildpack` source types, you must supply `source_code_path` to a zip of application source code. For the `docker` source type, you must supply the `docker_image`.
* `stack` - (Optional) The GUID of the stack the application will be deployed to. Use the [`cloudfoundry_stack`](website/docs/d/stack.html.markdown) data resource to lookup the stack GUID to override Cloud Foundry default.
* `buildpacks` - (Optional, list of strings) The buildpacks used to stage the application. There are multiple options to choose from:
   * a Git URL (e.g. https://github.com/cloudfoundry/java-buildpack.git) or a Git URL with a branch or tag (e.g. https://github.com/cloudfoundry/java-buildpack.git#v3.3.0 for v3.3.0 tag)
   * an installed admin buildpack name (e.g. my-buildpack)
   * an empty blank string to use built-in buildpacks (i.e. autodetection)
* `command` - (Optional, String) A custom start command for the application (this is only used to trigger rebuild/deployment - it should be set to the output attribute from the `cloudfoundry_app` resource).
* `environment` - (Optional, String) A custom build environment for the application (this is only used to trigger rebuild/deployment - it should be set to the output attribute from the `cloudfoundry_app` resource).
* `source_code_path` - (Required) An uri or path to target a zip file. this can be in the form of unix path (`/my/path.zip`) or url path (`http://zip.com/my.zip`)
* `source_code_hash` - (Optional) Used to trigger updates. Must be set to a base64-encoded SHA256 hash of the path specified. The usual way to set this is `${base64sha256(file("file.zip"))}`,
where "file.zip" is the local filename of the lambda function source archive.
* `docker_image` - (Optional, String) The URL to the docker image with tag e.g registry.example.com:5000/user/repository/tag or docker image name from the public repo e.g. redis:4.0
* `docker_username` - (Optional, String) The username to use for accessing a private docker_image
* `docker_password` - (Optional, String) The password to use for accessing a private docker_image
