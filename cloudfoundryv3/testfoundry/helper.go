package testfoundry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/cli/api/cloudcontroller/ccv3"
	"code.cloudfoundry.org/cli/resources"
	"github.com/terraform-providers/terraform-provider-cloudfoundry/cloudfoundryv3/managers"
)

type TestEnv struct {
	Session         *managers.Session
	Organization    resources.Organization
	Space           resources.Space
	Domain          resources.Domain
	ServiceBroker   resources.ServiceBroker
	ServiceOffering resources.ServiceOffering
	ServicePlan     resources.ServicePlan
}

func NewTestEnv() *TestEnv {
	session := getTestSession()
	org := getTestOrganization(session)
	space := getTestSpace(session, org.GUID)
	serviceOffering := getTestServiceOffering(session)
	return &TestEnv{
		Session:         session,
		Organization:    org,
		Space:           space,
		Domain:          getTestDomain(session),
		ServiceBroker:   getTestServiceBroker(session),
		ServiceOffering: serviceOffering,
		ServicePlan:     getTestServicePlan(session, serviceOffering),
	}
}

func (*TestEnv) AssetPath(a ...string) string {
	return filepath.Join(assetDir(), filepath.Join(a...))
}

func getTestSession() *managers.Session {
	c := managers.Config{
		Endpoint: os.Getenv("CF_API_URL"),
		User:     os.Getenv("CF_USER"),
		Password: os.Getenv("CF_PASSWORD"),
	}
	session, err := managers.NewSession(c)
	if err != nil {
		panic(err)
	}
	return session
}

func getTestDomain(session *managers.Session) resources.Domain {
	domainName := os.Getenv("TEST_DOMAIN_NAME")
	if domainName == "" {
		panic(fmt.Errorf("testenv: TEST_DOMAIN_NAME must be set"))
	}
	domains, _, err := session.ClientV3.GetDomains(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{domainName}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(domains) != 1 {
		panic(fmt.Errorf("testenv: domain '%s' does not exist", domainName))
	}

	return domains[0]
}

func getTestOrganization(session *managers.Session) resources.Organization {
	orgName := os.Getenv("TEST_ORG_NAME")
	if orgName == "" {
		panic(fmt.Errorf("testenv: TEST_ORG_NAME must be set"))
	}
	orgs, _, err := session.ClientV3.GetOrganizations(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{orgName}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(orgs) != 1 {
		panic(fmt.Errorf("testenv: org '%s' does not exist", orgName))
	}
	org := orgs[0]

	return org
}

func getTestSpace(session *managers.Session, orgGUID string) resources.Space {
	spaceName := os.Getenv("TEST_SPACE_NAME")
	if spaceName == "" {
		panic(fmt.Errorf("testenv: TEST_SPACE_NAME must be set"))
	}

	spaces, _, _, err := session.ClientV3.GetSpaces(
		ccv3.Query{Key: ccv3.OrganizationGUIDFilter, Values: []string{orgGUID}},
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{spaceName}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(spaces) != 1 {
		panic(fmt.Errorf("testenv: space '%s' does not exist", spaceName))
	}
	space := spaces[0]

	return space
}

func getTestServiceBroker(session *managers.Session) resources.ServiceBroker {
	brokerName := os.Getenv("TEST_SERVICE_BROKER_NAME")
	if brokerName == "" {
		panic(fmt.Errorf("testenv: TEST_SERVICE_BROKER_NAME must be set"))
	}
	serviceBrokers, _, err := session.ClientV3.GetServiceBrokers(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{brokerName}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(serviceBrokers) != 1 {
		panic(fmt.Errorf("testenv: broker '%s' does not exist", brokerName))
	}

	return serviceBrokers[0]
}

func getTestServiceOffering(session *managers.Session) resources.ServiceOffering {
	name := os.Getenv("TEST_SERVICE_NAME")
	if name == "" {
		panic(fmt.Errorf("testenv: TEST_SERVICE_NAME must be set"))
	}
	services, _, err := session.ClientV3.GetServiceOfferings(
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{name}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(services) != 1 {
		panic(fmt.Errorf("testenv: broker '%s' does not exist", name))
	}

	return services[0]
}

func getTestServicePlan(session *managers.Session, so resources.ServiceOffering) resources.ServicePlan {
	name := os.Getenv("TEST_SERVICE_PLAN_NAME")
	if name == "" {
		panic(fmt.Errorf("testenv: TEST_SERVICE_PLAN_NAME must be set"))
	}
	plans, _, err := session.ClientV3.GetServicePlans(
		ccv3.Query{Key: ccv3.ServiceOfferingGUIDsFilter, Values: []string{so.GUID}},
		ccv3.Query{Key: ccv3.NameFilter, Values: []string{name}},
	)
	if err != nil {
		panic(fmt.Errorf("testenv: %s", err))
	} else if len(plans) == 0 {
		panic(fmt.Errorf("testenv: plan '%s' not found", name))
	}

	return plans[0]
}

func defaultBaseDir() string {
	_, file, _, _ := runtime.Caller(1)
	return filepath.Dir(filepath.Dir(file))
}

func testDir() string {
	return filepath.Join(defaultBaseDir(), "..", "tests")
}

func assetDir() string {
	return filepath.Join(testDir(), "cf-acceptance-tests", "assets")
}
