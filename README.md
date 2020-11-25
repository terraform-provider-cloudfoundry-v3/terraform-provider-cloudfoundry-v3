# Cloud Foundry Terraform Provider (v3 API)

Experimental implementation of terraform resource for rolling deployments of
cloudfoundry applications using the v3 API.


Long-term intention is to contribute back
[upstream](https://github.com/cloudfoundry-community/terraform-provider-cloudfoundry).
This is the minimum viable
chunk to meet our immediate need.

## Usage

See the entry in the [Terraform Registry](https://registry.terraform.io/providers/terraform-provider-cloudfoundry-v3/cloudfoundry-v3/latest)

```hcl
terraform {
  required_providers {
    cloudfoundry-v3 = {
      source  = "terraform-provider-cloudfoundry-v3/cloudfoundry-v3"
      version = "0.333.2"
    }
  }
  required_version = ">= 0.13"
}

provider "cloudfoundry-v3" {
  api_url      = var.cf_api_url
  user         = var.cf_username
  password     = var.cf_password
}

resource "cloudfoundry_v3_app" "basic" {
	provider              = cloudfoundry-v3
	name                  = "basic-buildpack"
	space_id              = data.cloudfoundry_v3_space.myspace.id
	environment           = {MY_VAR = "1"}
	instances             = 2
	memory_in_mb          = 1024
	disk_in_mb            = 1024
	health_check_type     = "http"
	health_check_endpoint = "/"
}

resource "cloudfoundry_v3_droplet" "basic" {
	provider         = cloudfoundry-v3
	app_id           = cloudfoundry_v3_app.basic.id
	buildpacks       = ["binary_buildpack"]
	environment      = cloudfoundry_v3_app.basic.environment
	command          = cloudfoundry_v3_app.basic.command
	source_code_path = "/path/to/source.zip"
	source_code_hash = filemd5("/path/to/source.zip")
	depends_on = [
		cloudfoundry_v3_service_binding.dmz_proxy_splunk,
		cloudfoundry_network_policy.dmz_proxy,
	]
}

resource "cloudfoundry_v3_deployment" "basic" {
	provider   = cloudfoundry-v3
	strategy   = "rolling"
	app_id     = cloudfoundry_v3_app.basic.id
	droplet_id = cloudfoundry_v3_droplet.basic.id
}
```

## Development / Releases

There were conserns that the org-wide permissions the terraform registery
requires for release were too broad, so the release process is bit funky...

* Merge to master in this repo trigger a sync to master on a repo outside the alphagov org [here](https://github.com/terraform-provider-cloudfoundry-v3/terraform-provider-cloudfoundry-v3)
* Creating a tag in this repo of the form `v0.333.X` will trigger a Github Action that performs the release: [see here](https://github.com/terraform-provider-cloudfoundry-v3/terraform-provider-cloudfoundry-v3/actions)
* The Terraform Registry entry will automatically get updated [here](https://registry.terraform.io/providers/terraform-provider-cloudfoundry-v3/cloudfoundry-v3/latest)
