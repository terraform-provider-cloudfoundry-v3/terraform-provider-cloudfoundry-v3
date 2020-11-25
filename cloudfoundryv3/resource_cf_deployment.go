package cloudfoundry

import (
	"context"
	"fmt"
	"log"
	"time"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

const (
	MaxDeploymentAttempts = 3
)

func resourceDeployment() *schema.Resource {

	return &schema.Resource{
		Description: "deployments associate the desired active droplet to use for a running application and handle managing the changes without interuption",

		CreateContext: resourceDeploymentCreate,
		ReadContext:   resourceDeploymentRead,
		DeleteContext: resourceDeploymentDelete,

		// Importer: &schema.ResourceImporter{
		// 	State: ImportRead(resourceDeploymentRead),
		// },

		Schema: map[string]*schema.Schema{

			"strategy": {
				Description:  "deployment method (currently only 'rolling' supported)",
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"rolling"}, false),
			},

			"app_id": {
				Description:  "id of the application to perform deployment on",
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},

			"droplet_id": {
				Description:  "the application's droplet. droplet must be in ready state (successfully staged)",
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.NoZeroValues,
			},
		},
	}
}

func resourceDeploymentCreate(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)
	appGUID := d.Get("app_id").(string)
	desiredDropletGUID := d.Get("droplet_id").(string)
	waitTimeout := d.Timeout(schema.TimeoutCreate)

	desiredDroplet, warns, err := s.ClientV3.GetDroplet(desiredDropletGUID)
	diags = append(diags, diagFromClient("get-desired-droplet-for-deployment", warns, err)...)
	if diags.HasError() {
		return diags
	}

	app, exists, errs := getApplication(s, appGUID)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if !exists {
		d.SetId("")
		return diags
	}

	// currentDroplet, _, _ := s.ClientV3.GetApplicationDropletCurrent(app.GUID)
	desiredApplicationState := constant.ApplicationStarted

	// deployments often fail on the first attempt
	// I have no idea why, so we try a few times
	deployment, errs := createDeploymentFromDropletWithRetry(ctx, s, *app, desiredDroplet, waitTimeout)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	d.SetId(deployment.GUID)

	// now start or stop the application it's unclear if this should be part of the
	// deployment or if it is required at all

	app, exists, errs = getApplication(s, app.GUID)
	diags = append(diags, errs...)
	if diags.HasError() {
		return diags
	}
	if !exists {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("app (%s) went missing during update, removing from state", app.GUID),
			Detail:   "get-application-state-during-deployment",
		})
		d.SetId("")
		return diags
	}
	if desiredApplicationState != app.State {
		switch desiredApplicationState {
		case constant.ApplicationStopped:
			log.Printf("[%s] stopping application... OK!\n", app.Name)
			_, warns, err := s.ClientV3.UpdateApplicationStop(app.GUID)
			diags = append(diags, diagFromClient("update-application", warns, err)...)
			if diags.HasError() {
				return diags
			}
		case constant.ApplicationStarted:
			if desiredDroplet.GUID != "" {
				log.Printf("[%s] starting application... OK!\n", app.Name)
				_, warns, err := s.ClientV3.UpdateApplicationStart(app.GUID)
				diags = append(diags, diagFromClient("update-application", warns, err)...)
				if diags.HasError() {
					return diags
				}
				processes, warns, err := s.ClientV3.GetApplicationProcesses(d.Id())
				diags = append(diags, diagFromClient("get-application-processes", warns, err)...)
				if diags.HasError() {
					return diags
				}

				for _, process := range processes {
					jobState := &resource.StateChangeConf{
						Pending:        processInstancePendingStates,
						Target:         processInstanceSuccessStates,
						Refresh:        processInstanceStateFunc(s, process),
						Timeout:        d.Timeout(schema.TimeoutCreate),
						PollInterval:   5 * time.Second,
						Delay:          5 * time.Second,
						NotFoundChecks: 2,
					}
					if _, err = jobState.WaitForStateContext(ctx); err != nil {
						return diag.FromErr(err)
					}
				}
			}
		}
	}

	return resourceDeploymentRead(ctx, d, m)
}

func createDeploymentFromDropletWithRetry(ctx context.Context, s *managers.Session, app resources.Application, desiredDroplet resources.Droplet, waitTimeout time.Duration) (_ *resources.Deployment, diags diag.Diagnostics) {
	currentDeployAttempt := 0
	for {
		currentDeployAttempt += 1
		deployment, errs := createDeploymentFromDroplet(ctx, s, app, desiredDroplet, waitTimeout)
		if errs.HasError() {
			if currentDeployAttempt < MaxDeploymentAttempts {
				continue
			}
			diags = append(diags, errs...)
			return nil, diags
		}

		return deployment, diags
	}
}

func createDeploymentFromDroplet(ctx context.Context, s *managers.Session, app resources.Application, desiredDroplet resources.Droplet, waitTimeout time.Duration) (_ *resources.Deployment, diags diag.Diagnostics) {

	log.Printf("[%s] rolling deployment...\n", app.Name)

	deploymentGUID, warns, err := s.ClientV3.CreateApplicationDeployment(app.GUID, desiredDroplet.GUID)
	diags = append(diags, diagFromClient("create-deployment droplet:"+desiredDroplet.GUID, warns, err)...)
	if diags.HasError() {
		return nil, diags
	}
	deploymentState := &resource.StateChangeConf{
		Pending:        deploymentPendingStates,
		Target:         deploymentSuccessStates,
		Refresh:        deploymentStateFunc(s, deploymentGUID),
		Timeout:        waitTimeout,
		PollInterval:   5 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 2,
	}
	lastDeploymentResponse, err := deploymentState.WaitForStateContext(ctx)
	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
		return nil, diags
	} else if lastDeploymentResponse == nil {
		diags = append(diags, diag.FromErr(fmt.Errorf("invalid response from deployment state watcher, expected a deployment got nil"))...)
		return nil, diags
	}

	deployment, ok := lastDeploymentResponse.(resources.Deployment)
	if !ok {
		diags = append(diags, diag.FromErr(fmt.Errorf("invalid response from deployment state watcher, expected a deployment got %#v", deployment))...)
		return nil, diags
	}

	log.Printf("[%s] rolling deployment... OK!\n", app.Name)

	processes, warns, err := s.ClientV3.GetNewApplicationProcesses(app.GUID, deploymentGUID)
	diags = append(diags, diagFromClient("get-new-application-processes", warns, err)...)
	if diags.HasError() {
		return nil, diags
	}

	for _, process := range processes {
		log.Printf("[%s] waiting for %s process to stablise... \n", app.Name, process.Type)

		jobState := &resource.StateChangeConf{
			Pending:        processInstancePendingStates,
			Target:         processInstanceSuccessStates,
			Refresh:        processInstanceStateFunc(s, process),
			Timeout:        waitTimeout,
			PollInterval:   2 * time.Second,
			Delay:          2 * time.Second,
			NotFoundChecks: 2,
		}
		if _, err = jobState.WaitForStateContext(ctx); err != nil {
			diags = append(diags, diag.FromErr(err)...)
			return nil, diags
		}

		log.Printf("[%s] waiting for %s process to stablise... OK!\n", app.Name, process.Type)
	}

	return &deployment, diags
}

func resourceDeploymentRead(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	s := m.(*managers.Session)
	appGUID := d.Get("app_id").(string)
	deployments, warns, err := s.ClientV3.GetDeployments(
		ccv3.Query{Key: ccv3.AppGUIDFilter, Values: []string{appGUID}},
	)
	diags = append(diags, diagFromClient("get-new-application-processes", warns, err)...)
	if diags.HasError() {
		return diags
	}
	var deployment *resources.Deployment
	for _, appDeployment := range deployments {
		if appDeployment.GUID == d.Id() {
			deployment = &appDeployment
			break
		}
	}
	if deployment == nil {
		d.SetId("")
	}

	return diags
}

func resourceDeploymentDelete(ctx context.Context, d *schema.ResourceData, m interface{}) (diags diag.Diagnostics) {
	// s := m.(*managers.Session)

	// You don't really delete deployments, so this is a no-op
	// TODO: should deleting a deployment resource STOP the app?
	// TODO: should deleting a deployment resource trigger a "cancel"?

	return diags
}

func deploymentStateFunc(s *managers.Session, deploymentGUID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		deployment, _, err := s.ClientV3.GetDeployment(deploymentGUID)
		if err != nil {
			return nil, "", err
		}

		switch deployment.StatusValue {
		case constant.DeploymentStatusValueFinalized:
			switch deployment.StatusReason {
			case constant.DeploymentStatusReasonDeployed:
				return deployment, string(constant.DeploymentDeployed), nil
			default:
				return nil, string(constant.DeploymentFailed), fmt.Errorf("deployment failed: %s", deployment.StatusReason)
			}
		default:
			return deployment, string(constant.DeploymentDeploying), nil
		}
	}
}

var deploymentPendingStates = []string{
	string(constant.DeploymentDeploying),
}

var deploymentSuccessStates = []string{
	string(constant.DeploymentDeployed),
}
