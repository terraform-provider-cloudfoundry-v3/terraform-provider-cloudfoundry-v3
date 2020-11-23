package cloudfoundry

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccerror"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3/constant"
	"code.cloudfoundry.org/cli/resources"
	"code.cloudfoundry.org/cli/types"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceServiceInstance() *schema.Resource {

	return &schema.Resource{

		CreateContext: resourceServiceInstanceCreate,
		ReadContext:   resourceServiceInstanceRead,
		UpdateContext: resourceServiceInstanceUpdate,
		DeleteContext: resourceServiceInstanceDelete,

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Update: schema.DefaultTimeout(15 * time.Minute),
			Delete: schema.DefaultTimeout(15 * time.Minute),
		},

		Schema: map[string]*schema.Schema{

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"service_plan": {
				Type:     schema.TypeString,
				Required: true,
			},

			"space_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"json_params": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "{}",
				ValidateFunc: validation.StringIsJSON,
			},

			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},

			"recursive_delete": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func resourceServiceInstanceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	s := meta.(*managers.Session)

	tags := make([]string, 0)
	for _, v := range d.Get("tags").([]interface{}) {
		tags = append(tags, v.(string))
	}

	jsonParameters := d.Get("json_params").(string)
	params := make(map[string]interface{})
	if len(jsonParameters) > 0 {
		err := json.Unmarshal([]byte(jsonParameters), &params)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	si := resources.ServiceInstance{
		Type:            resources.ManagedServiceInstance,
		Name:            d.Get("name").(string),
		ServicePlanGUID: d.Get("service_plan").(string),
		SpaceGUID:       d.Get("space_id").(string),
		Tags:            types.NewOptionalStringSlice(tags...),
		Parameters:      types.NewOptionalObject(params),
	}
	createJobURL, warns, err := s.ClientV3.CreateServiceInstance(si)
	diags = append(diags, diagFromClient("create-service-instance", warns, err)...)
	if diags.HasError() {
		return diags
	}

	stateConf := &resource.StateChangeConf{
		Pending:        jobPendingStates,
		Target:         jobSuccessStates,
		Refresh:        jobStateFunc(s, createJobURL),
		Timeout:        d.Timeout(schema.TimeoutCreate),
		PollInterval:   15 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 1,
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	si, _, warns, err = s.ClientV3.GetServiceInstanceByNameAndSpace(si.Name, si.SpaceGUID)
	diags = append(diags, diagFromClient("fetch-created-service-instance", warns, err)...)
	if diags.HasError() {
		return diags
	}

	d.SetId(si.GUID)

	return nil
}

func resourceServiceInstanceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	s := meta.(*managers.Session)

	name := d.Get("name").(string)
	spaceGUID := d.Get("space_id").(string)

	serviceInstances, _, warns, err := s.ClientV3.GetServiceInstances(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{name}},
		ccv3.Query{Key: ccv3.SpaceGUIDFilter, Values: []string{spaceGUID}},
	)
	diags = append(diags, diagFromClient("get-service-instances", warns, err)...)
	if diags.HasError() {
		return diags
	}
	if len(serviceInstances) == 0 {
		d.SetId("")
		return diags
	}
	si := serviceInstances[0]

	// since we don't have a "get-by-guid" we should
	// check that we are reading from the expected service
	if si.GUID != d.Id() {
		d.SetId("")
		return diags
	}

	params, warns, err := s.ClientV3.GetServiceInstanceParameters(si.GUID)
	diags = append(diags, diagFromClient("get-service-instance-params", warns, err)...)
	if diags.HasError() {
		return diags
	}
	paramsBytes, err := params.MarshalJSON()
	diags = append(diags, diagFromClient("marshal-service-instance-params", warns, err)...)
	if diags.HasError() {
		return diags
	}

	_ = d.Set("name", si.Name)
	_ = d.Set("service_plan", si.ServicePlanGUID)
	_ = d.Set("space_id", si.SpaceGUID)
	_ = d.Set("json_params", string(paramsBytes))

	if si.Tags.IsSet {
		tags := make([]interface{}, len(si.Tags.Value))
		for i, v := range si.Tags.Value {
			tags[i] = v
		}
		_ = d.Set("tags", tags)
	} else {
		_ = d.Set("tags", nil)
	}

	return nil
}

func resourceServiceInstanceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	s := meta.(*managers.Session)

	var (
		id     string
		name   string
		tags   types.OptionalStringSlice
		params types.OptionalObject
	)

	id = d.Id()
	name = d.Get("name").(string)
	servicePlan := d.Get("service_plan").(string)
	jsonParameters := d.Get("json_params").(string)

	if len(jsonParameters) > 0 {
		err := json.Unmarshal([]byte(jsonParameters), &params)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	for _, v := range d.Get("tags").([]interface{}) {
		tags.Value = append(tags.Value, v.(string))
		tags.IsSet = true
	}

	updateJobURL, warns, err := s.ClientV3.UpdateServiceInstance(id, resources.ServiceInstance{
		Name:            name,
		ServicePlanGUID: servicePlan,
		Parameters:      params,
		Tags:            tags,
	})
	diags = append(diags, diagFromClient("update-service-instance", warns, err)...)
	if diags.HasError() {
		return diags
	}

	stateConf := &resource.StateChangeConf{
		Pending:        jobPendingStates,
		Target:         jobSuccessStates,
		Refresh:        jobStateFunc(s, updateJobURL),
		Timeout:        d.Timeout(schema.TimeoutUpdate),
		PollInterval:   30 * time.Second,
		Delay:          5 * time.Second,
		NotFoundChecks: 3, // if we don't find the service instance in CF during an update, something is definitely wrong
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceServiceInstanceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) (diags diag.Diagnostics) {
	s := meta.(*managers.Session)
	id := d.Id()

	deleteJobURL, warns, err := s.ClientV3.DeleteServiceInstance(id)
	diags = append(diags, diagFromClient("delete-service-instance", warns, err)...)
	if diags.HasError() {
		return diags
	}

	stateConf := &resource.StateChangeConf{
		Pending:      jobPendingStates,
		Target:       jobSuccessStates,
		Refresh:      jobStateFunc(s, deleteJobURL),
		Timeout:      d.Timeout(schema.TimeoutDelete),
		PollInterval: 15 * time.Second,
		Delay:        5 * time.Second,
	}
	if _, err = stateConf.WaitForStateContext(ctx); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// func resourceServiceInstanceImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
// 	s := meta.(*managers.Session)

// 	serviceinstance, _, err := s.ClientV2.GetServiceInstance(d.Id())

// 	if err != nil {
// 		return nil, err
// 	}

// 	d.Set("name", serviceinstance.Name)
// 	d.Set("service_plan", serviceinstance.ServicePlanGUID)
// 	d.Set("space_id", serviceinstance.SpaceGUID)
// 	d.Set("tags", serviceinstance.Tags)

// 	// json_param can't be retrieved from CF, please inject manually if necessary
// 	d.Set("json_param", "")

// 	return ImportRead(resourceServiceInstanceRead)(d, meta)
// }

func jobStateFunc(s *managers.Session, jobURL ccv3.JobURL) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {

		job, _, err := s.ClientV3.GetJob(jobURL)
		if err != nil {
			return nil, "", err
		}

		if job.HasFailed() {
			if len(job.Errors()) > 0 {
				return job, string(job.State), job.Errors()[0]
			} else {
				return job, "", ccerror.JobFailedNoErrorError{
					JobGUID: job.GUID,
				}
			}
		}

		return job, string(job.State), nil
	}
}

var jobPendingStates = []string{
	string(constant.JobPolling),
	string(constant.JobProcessing),
}

var jobSuccessStates = []string{
	string(constant.JobComplete),
}
