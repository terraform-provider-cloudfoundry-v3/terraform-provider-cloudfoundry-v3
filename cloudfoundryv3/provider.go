package cloudfoundry

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

// Provider -
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_API_URL", ""),
			},
			"user": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_USER", "admin"),
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_PASSWORD", ""),
			},
			"sso_passcode": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_SSO_PASSCODE", ""),
			},
			"cf_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_CLIENT_ID", ""),
			},
			"cf_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_CLIENT_SECRET", ""),
			},
			"uaa_client_id": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_UAA_CLIENT_ID", ""),
			},
			"uaa_client_secret": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_UAA_CLIENT_SECRET", "admin"),
			},
			"skip_ssl_validation": {
				Type:        schema.TypeBool,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_SKIP_SSL_VALIDATION", false),
			},
			"default_quota_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the default quota",
				DefaultFunc: schema.EnvDefaultFunc("CF_DEFAULT_QUOTA_NAME", "default"),
			},
			"app_logs_max": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Number of logs message which can be see when app creation is errored (-1 means all messages stored)",
				DefaultFunc: schema.EnvDefaultFunc("CF_APP_LOGS_MAX", 30),
			},
			"purge_when_delete": {
				Type:        schema.TypeBool,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_PURGE_WHEN_DELETE", false),
				Description: "Set to true to purge when deleting a resource (e.g.: service instance, service broker)",
			},
			"store_tokens_path": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("CF_STORE_TOKENS_PATH", ""),
				Description: "Path to a file to store tokens used for login. (this is useful for sso, this avoid requiring each time sso passcode)",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"cloudfoundry_v3_domain": dataSourceDomain(),
			"cloudfoundry_v3_org":    dataSourceOrg(),
			"cloudfoundry_v3_space":  dataSourceSpace(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"cloudfoundry_v3_route":             resourceRoute(),
			"cloudfoundry_v3_route_destination": resourceRouteDestination(),
			"cloudfoundry_v3_app":               resourceApp(),
			"cloudfoundry_v3_droplet":           resourceDroplet(),
			"cloudfoundry_v3_deployment":        resourceDeployment(),
			"cloudfoundry_v3_service_instance":  resourceServiceInstance(),
			"cloudfoundry_v3_service_binding":   resourceServiceBinding(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	c := managers.Config{
		Endpoint:          strings.TrimSuffix(d.Get("api_url").(string), "/"),
		User:              d.Get("user").(string),
		Password:          d.Get("password").(string),
		SSOPasscode:       d.Get("sso_passcode").(string),
		CFClientID:        d.Get("cf_client_id").(string),
		CFClientSecret:    d.Get("cf_client_secret").(string),
		UaaClientID:       d.Get("uaa_client_id").(string),
		UaaClientSecret:   d.Get("uaa_client_secret").(string),
		SkipSslValidation: d.Get("skip_ssl_validation").(bool),
		AppLogsMax:        d.Get("app_logs_max").(int),
		DefaultQuotaName:  d.Get("default_quota_name").(string),
		StoreTokensPath:   d.Get("store_tokens_path").(string),
	}

	session, err := managers.NewSession(c)
	if err != nil {
		panic(err)
		// return nil, diag.FromErr(err)
	}

	return session, nil
}
