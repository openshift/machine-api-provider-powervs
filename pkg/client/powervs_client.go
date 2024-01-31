package client

import (
	"context"
	"fmt"
	gohttp "net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/authentication"
	"github.com/IBM-Cloud/bluemix-go/http"
	"github.com/IBM-Cloud/bluemix-go/rest"
	bxsession "github.com/IBM-Cloud/bluemix-go/session"
	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	powerUtils "github.com/ppc64le-cloud/powervs-utils"

	configv1 "github.com/openshift/api/config/v1"
	machineapiapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
	utils "github.com/openshift/machine-api-provider-powervs/pkg/utils"
)

const (
	//TIMEOUT is default timeout used by Power VS client for operations like DeleteInstance
	TIMEOUT = time.Hour

	//DefaultCredentialNamespace is the default namespace used to create a client object
	DefaultCredentialNamespace = "openshift-machine-api"
	//DefaultCredentialSecret is the credential secret name used by node update controller to fetch API key
	DefaultCredentialSecret = "powervs-credentials"

	//InstanceStateNameShutoff is indicates the shutoff state of Power VS instance
	InstanceStateNameShutoff = "SHUTOFF"
	//InstanceStateNameActive is indicates the active state of Power VS instance
	InstanceStateNameActive = "ACTIVE"
	//InstanceStateNameBuild is indicates the build state of Power VS instance
	InstanceStateNameBuild = "BUILD"
	//InstanceBuildReason indicates that instance is in building state
	InstanceBuildReason = "InstanceBuildState"

	// globalInfrastuctureName default name for infrastructure object
	globalInfrastuctureName = "cluster"

	//powerIaaSCustomEndpointName is the short name used to fetch Power IaaS endpoint URL
	powerIaaSCustomEndpointName = "pi"

	//powerVSResourceID is Power VS power-iaas service id, can be retrieved using ibmcloud cli
	// ibmcloud catalog service power-iaas
	powerVSResourceID = "abd259f0-9990-11e8-acc8-b9f54a8f1661"

	//powerVSResourcePlanID is Power VS power-iaas plan id, can be retrieved using ibmcloud cli
	// ibmcloud catalog service power-iaas
	powerVSResourcePlanID = "f165dd34-3a40-423b-9d95-e90a23f724dd"
)

var _ Client = &powerVSClient{}

var (
	//ErrorInstanceNotFound is error type for Instance Not Found
	ErrorInstanceNotFound = errors.New("instance Not Found")

	//endPointKeyToEnvNameMap contains the endpoint key to corresponding environment variable name 	//TODO: Finalize the custom service key names
	endPointKeyToEnvNameMap = map[string]string{
		"iam": "IBMCLOUD_IAM_API_ENDPOINT",
		"rc":  "IBMCLOUD_RESOURCE_CONTROLLER_API_ENDPOINT",
		"pi":  "IBMCLOUD_POWER_API_ENDPOINT",
	}
)

// FormatProviderID formats and returns the provided instanceID
func FormatProviderID(region, zone, serviceInstanceID, vmInstanceID string) string {
	// ProviderID format: ibmpowervs://<region>/<zone>/<service_instance_id>/<powervs_machine_id>
	return fmt.Sprintf("ibmpowervs://%s/%s/%s/%s", region, zone, serviceInstanceID, vmInstanceID)
}

// PowerVSClientBuilderFuncType is function type for building the Power VS client
type PowerVSClientBuilderFuncType func(client client.Client, secretName, namespace, cloudInstanceID string,
	debug bool) (Client, error)

// MinimalPowerVSClientBuilderFuncType is function type for building the Power VS client
type MinimalPowerVSClientBuilderFuncType func(client client.Client) (Client, error)

func apiKeyFromSecret(secret *corev1.Secret) (apiKey string, err error) {
	switch {
	case len(secret.Data["ibmcloud_api_key"]) > 0:
		apiKey = string(secret.Data["ibmcloud_api_key"])
	default:
		return "", fmt.Errorf("invalid secret for powervs credentials")
	}
	return
}

// GetAPIKey will return the api key read from given secretName in a given namespace
func GetAPIKey(ctrlRuntimeClient client.Client, secretName, namespace string) (apikey string, err error) {
	if secretName == "" {
		return "", machineapiapierrors.InvalidMachineConfiguration("empty secret name")
	}
	var secret corev1.Secret
	if err := ctrlRuntimeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: secretName}, &secret); err != nil {
		if apimachineryerrors.IsNotFound(err) {
			return "", machineapiapierrors.InvalidMachineConfiguration("powervs credentials secret %s/%s: %v not found", namespace, secretName, err)
		}
		return "", err
	}
	apikey, err = apiKeyFromSecret(&secret)
	if err != nil {
		return "", fmt.Errorf("failed to create shared credentials file from Secret: %v", err)
	}
	return
}

// NewValidatedClient creates and return a new Power VS client
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, cloudInstanceID string, debug bool) (Client, error) {
	apikey, err := GetAPIKey(ctrlRuntimeClient, secretName, namespace)
	if err != nil {
		return nil, err
	}

	if err := getAndSetServiceEndpoints(ctrlRuntimeClient); err != nil {
		return nil, err
	}

	s, err := bxsession.New(&bluemix.Config{BluemixAPIKey: apikey})
	if err != nil {
		return nil, err
	}

	c := &powerVSClient{
		cloudInstanceID: cloudInstanceID,
		Session:         s,
	}

	err = authenticateAPIKey(s)
	if err != nil {
		return c, err
	}

	// Create the authenticator
	authenticator := &core.IamAuthenticator{
		ApiKey: apikey,
	}

	rcv2, err := resourcecontrollerv2.NewResourceControllerV2(&resourcecontrollerv2.ResourceControllerV2Options{
		Authenticator: authenticator,
	})
	if err != nil {
		return c, err
	}
	if rcv2 == nil {
		return c, fmt.Errorf("unable to get resource controller")
	}
	c.ResourceClient = rcv2

	resourceInstanceList, _, err := c.ResourceClient.ListResourceInstances(&resourcecontrollerv2.ListResourceInstancesOptions{GUID: &cloudInstanceID})
	if err != nil {
		return c, errors.Wrapf(err, "failed to get service instance details with id %s", cloudInstanceID)
	}

	if resourceInstanceList == nil {
		return c, fmt.Errorf("failed to get service instance details with id %s service instance returned is nil", cloudInstanceID)
	}

	if *resourceInstanceList.RowsCount != 1 {
		return c, fmt.Errorf("failed to get service instance details, expecting one service instance with id %s but got %d", cloudInstanceID, *resourceInstanceList.RowsCount)
	}

	r, err := powerUtils.GetRegion(*resourceInstanceList.Resources[0].RegionID)
	if err != nil {
		return nil, err
	}
	c.region = r
	c.zone = *resourceInstanceList.Resources[0].RegionID

	c.User, err = fetchUserDetails(s, 2)
	if err != nil {
		return c, err
	}

	// Create the session options struct
	options := &ibmpisession.IBMPIOptions{
		Authenticator: authenticator,
		UserAccount:   c.User.Account,
		Region:        c.region,
		Zone:          c.zone,
		Debug:         debug,
	}

	// Construct the session service instance
	c.session, err = ibmpisession.NewIBMPISession(options)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	c.InstanceClient = instance.NewIBMPIInstanceClient(ctx, c.session, cloudInstanceID)
	c.NetworkClient = instance.NewIBMPINetworkClient(ctx, c.session, cloudInstanceID)
	c.ImageClient = instance.NewIBMPIImageClient(ctx, c.session, cloudInstanceID)
	c.DHCPClient = instance.NewIBMPIDhcpClient(ctx, c.session, cloudInstanceID)

	vpcRegion, err := powerUtils.VPCRegionForPowerVSRegion(c.region)
	if err != nil {
		return nil, err
	}

	vpcClient, err := vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		ServiceName:   "vpcs",
		Authenticator: authenticator,
		// TODO: support custom service endpoints
		URL: fmt.Sprintf("https://%s.iaas.cloud.ibm.com/v1", vpcRegion),
	})
	if err != nil {
		return nil, err
	}

	c.VPCClient = vpcClient
	return c, err
}

// NewMinimalPowerVSClient is bare minimal client can be used for querying the resources
func NewMinimalPowerVSClient(ctrlRuntimeClient client.Client) (Client, error) {
	if err := getAndSetServiceEndpoints(ctrlRuntimeClient); err != nil {
		return nil, err
	}
	apiKey, err := GetAPIKey(ctrlRuntimeClient, DefaultCredentialSecret, DefaultCredentialNamespace)
	if err != nil {
		klog.Errorf("failed to read the API key from the secret: %v", err)
		return nil, err
	}
	rcv2, err := resourcecontrollerv2.NewResourceControllerV2(&resourcecontrollerv2.ResourceControllerV2Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: apiKey,
		},
	})
	if err != nil {
		return nil, err
	}
	if rcv2 == nil {
		return nil, fmt.Errorf("unable to get resource controller")
	}
	return &powerVSClient{
		ResourceClient: rcv2,
	}, nil
}

type powerVSClient struct {
	region          string
	zone            string
	cloudInstanceID string

	*bxsession.Session
	User           *User
	ResourceClient *resourcecontrollerv2.ResourceControllerV2
	session        *ibmpisession.IBMPISession
	InstanceClient *instance.IBMPIInstanceClient
	NetworkClient  *instance.IBMPINetworkClient
	ImageClient    *instance.IBMPIImageClient
	DHCPClient     *instance.IBMPIDhcpClient
	VPCClient      *vpcv1.VpcV1
}

func (p *powerVSClient) GetImages() (*models.Images, error) {
	return p.ImageClient.GetAll()
}

func (p *powerVSClient) GetNetworks() (*models.Networks, error) {
	return p.NetworkClient.GetAll()
}

func (p *powerVSClient) DeleteInstance(id string) error {
	return p.InstanceClient.Delete(id)
}

func (p *powerVSClient) CreateInstance(createParams *models.PVMInstanceCreate) (*models.PVMInstanceList, error) {
	return p.InstanceClient.Create(createParams)
}

func (p *powerVSClient) GetInstance(id string) (*models.PVMInstance, error) {
	return p.InstanceClient.Get(id)
}

func (p *powerVSClient) GetInstanceByName(name string) (*models.PVMInstance, error) {
	instances, err := p.GetInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to get the instance list, Error: %v", err)
	}

	for _, i := range instances.PvmInstances {
		if *i.ServerName == name {
			return p.GetInstance(*i.PvmInstanceID)
		}
	}
	return nil, ErrorInstanceNotFound
}

func (p *powerVSClient) GetInstances() (*models.PVMInstances, error) {
	return p.InstanceClient.GetAll()
}

func (p *powerVSClient) GetCloudServiceInstances() ([]resourcecontrollerv2.ResourceInstance, error) {
	var serviceInstancesList []resourcecontrollerv2.ResourceInstance
	f := func(start string) (bool, string, error) {
		listServiceInstanceOptions := &resourcecontrollerv2.ListResourceInstancesOptions{
			ResourceID:     pointer.String(powerVSResourceID),
			ResourcePlanID: pointer.String(powerVSResourcePlanID),
		}
		if start != "" {
			listServiceInstanceOptions.Start = &start
		}

		serviceInstances, _, err := p.ResourceClient.ListResourceInstances(listServiceInstanceOptions)
		if err != nil {
			return false, "", err
		}
		if serviceInstances != nil {
			serviceInstancesList = append(serviceInstancesList, serviceInstances.Resources...)
			nextURL, err := serviceInstances.GetNextStart()
			if err != nil {
				return false, "", err
			}
			if nextURL == nil {
				return true, "", nil
			}
			return false, *nextURL, nil
		}
		return true, "", nil
	}

	if err := utils.PagingHelper(f); err != nil {
		return nil, fmt.Errorf("error listing loadbalancer %v", err)
	}

	if serviceInstancesList == nil {
		return nil, errors.New("no service instance is retrieved")
	}
	return serviceInstancesList, nil
}

func (p *powerVSClient) GetCloudServiceInstanceByName(name string) (*resourcecontrollerv2.ResourceInstance, error) {
	var serviceInstancesList []resourcecontrollerv2.ResourceInstance
	f := func(start string) (bool, string, error) {
		listServiceInstanceOptions := &resourcecontrollerv2.ListResourceInstancesOptions{
			Name:           &name,
			ResourceID:     pointer.String(powerVSResourceID),
			ResourcePlanID: pointer.String(powerVSResourcePlanID),
		}
		if start != "" {
			listServiceInstanceOptions.Start = &start
		}

		serviceInstances, _, err := p.ResourceClient.ListResourceInstances(listServiceInstanceOptions)
		if err != nil {
			return false, "", err
		}
		if serviceInstances != nil {
			serviceInstancesList = append(serviceInstancesList, serviceInstances.Resources...)
			nextURL, err := serviceInstances.GetNextStart()
			if err != nil {
				return false, "", err
			}
			if nextURL == nil {
				return true, "", nil
			}
			return false, *nextURL, nil
		}
		return true, "", nil
	}

	if err := utils.PagingHelper(f); err != nil {
		return nil, fmt.Errorf("error listing service instances %v", err)
	}
	// log useful error message
	switch len(serviceInstancesList) {
	case 0:
		errStr := fmt.Errorf("does exist any cloud service instance with name %s", name)
		klog.Errorf(errStr.Error())
		return nil, fmt.Errorf("does exist any cloud service instance with name %s", name)
	case 1:
		klog.Infof("serviceInstance %s found with ID: %v", name, serviceInstancesList[0].GUID)
		return &serviceInstancesList[0], nil
	default:
		errStr := fmt.Errorf("there exist more than one service instance ID with with same name %s, Try setting serviceInstance.ID", name)
		klog.Errorf(errStr.Error())
		return nil, errStr
	}
}

func (p *powerVSClient) GetDHCPServers() (models.DHCPServers, error) {
	return p.DHCPClient.GetAll()
}

func (p *powerVSClient) GetDHCPServerByID(id string) (*models.DHCPServerDetail, error) {
	return p.DHCPClient.Get(id)
}

func (p *powerVSClient) GetZone() string {
	return p.zone
}

func (p *powerVSClient) GetRegion() string {
	return p.region
}

func (p *powerVSClient) ListLoadBalancers(listLoadBalancersOptions *vpcv1.ListLoadBalancersOptions) (*vpcv1.LoadBalancerCollection, *core.DetailedResponse, error) {
	return p.VPCClient.ListLoadBalancers(listLoadBalancersOptions)
}

func (p *powerVSClient) GetLoadBalancer(getLoadBalancerOptions *vpcv1.GetLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
	return p.VPCClient.GetLoadBalancer(getLoadBalancerOptions)
}

func (p *powerVSClient) ListLoadBalancerPoolMembers(listLoadBalancerPoolMembersOptions *vpcv1.ListLoadBalancerPoolMembersOptions) (*vpcv1.LoadBalancerPoolMemberCollection, *core.DetailedResponse, error) {
	return p.VPCClient.ListLoadBalancerPoolMembers(listLoadBalancerPoolMembersOptions)
}

func (p *powerVSClient) CreateLoadBalancerPoolMember(createLoadBalancerPoolMemberOptions *vpcv1.CreateLoadBalancerPoolMemberOptions) (*vpcv1.LoadBalancerPoolMember, *core.DetailedResponse, error) {
	return p.VPCClient.CreateLoadBalancerPoolMember(createLoadBalancerPoolMemberOptions)
}

func (p *powerVSClient) DeleteLoadBalancerPoolMember(deleteLoadBalancerPoolMemberOptions *vpcv1.DeleteLoadBalancerPoolMemberOptions) (*core.DetailedResponse, error) {
	return p.VPCClient.DeleteLoadBalancerPoolMember(deleteLoadBalancerPoolMemberOptions)
}

func authenticateAPIKey(sess *bxsession.Session) error {
	config := sess.Config
	tokenRefresher, err := authentication.NewIAMAuthRepository(config, &rest.Client{
		DefaultHeader: gohttp.Header{
			"User-Agent": []string{http.UserAgent()},
		},
	})
	if err != nil {
		return err
	}
	return tokenRefresher.AuthenticateAPIKey(config.BluemixAPIKey)
}

// User is used to hold the user details
type User struct {
	ID         string
	Email      string
	Account    string
	cloudName  string `default:"bluemix"`
	cloudType  string `default:"public"`
	generation int    `default:"2"`
}

func fetchUserDetails(sess *bxsession.Session, generation int) (*User, error) {
	config := sess.Config
	user := User{}
	var bluemixToken string

	if strings.HasPrefix(config.IAMAccessToken, "Bearer") {
		bluemixToken = config.IAMAccessToken[7:len(config.IAMAccessToken)]
	} else {
		bluemixToken = config.IAMAccessToken
	}

	token, err := jwt.Parse(bluemixToken, func(token *jwt.Token) (interface{}, error) {
		return "", nil
	})
	if err != nil && !strings.Contains(err.Error(), "key is of invalid type") {
		return &user, err
	}

	claims := token.Claims.(jwt.MapClaims)
	if email, ok := claims["email"]; ok {
		user.Email = email.(string)
	}
	user.ID = claims["id"].(string)
	user.Account = claims["account"].(map[string]interface{})["bss"].(string)
	iss := claims["iss"].(string)
	if strings.Contains(iss, "https://iam.cloud.ibm.com") {
		user.cloudName = "bluemix"
	} else {
		user.cloudName = "staging"
	}
	user.cloudType = "public"

	user.generation = generation
	return &user, nil
}

func resolveEndpoints(ctrlRuntimeClient client.Client) (map[string]string, error) {
	infra := &configv1.Infrastructure{}
	infraName := client.ObjectKey{Name: globalInfrastuctureName}

	if err := ctrlRuntimeClient.Get(context.Background(), infraName, infra); err != nil {
		return nil, err
	}

	// Do nothing when custom endpoints are missing
	if infra.Status.PlatformStatus == nil || infra.Status.PlatformStatus.PowerVS == nil {
		return nil, nil
	}

	customEndpointsMap := make(map[string]string)

	//Build the custom endpoint map
	for _, customEndpoint := range infra.Status.PlatformStatus.PowerVS.ServiceEndpoints {
		// Power VS client expects the IBMCLOUD_POWER_API_ENDPOINT variable to be set without scheme (https)
		if customEndpoint.Name == powerIaaSCustomEndpointName {
			peURL, err := url.Parse(customEndpoint.URL)
			if err != nil {
				return nil, err
			}
			customEndpoint.URL = peURL.Host
		}
		customEndpointsMap[customEndpoint.Name] = customEndpoint.URL
	}
	return customEndpointsMap, nil
}

func setCustomEndpoints(customEndpointsMap map[string]string, keys []string) error {
	//TODO(Doubt): Is it required to validate the endpoints again before setting it or blindly trust installer
	for _, key := range keys {
		if val, ok := customEndpointsMap[key]; ok {
			if err := setEnvironmentVariables(endPointKeyToEnvNameMap[key], val); err != nil {
				return err
			}
		}
	}
	return nil
}

func setEnvironmentVariables(key, val string) error {
	if err := os.Setenv(key, val); err != nil {
		return err
	}
	return nil
}

func getEnvironmentalVariableValue(key string) string {
	return os.Getenv(key)
}

func getAndSetServiceEndpoints(ctrlRuntimeClient client.Client) error {
	customEndpointsMap, err := resolveEndpoints(ctrlRuntimeClient)
	if err != nil {
		return err
	}

	if len(customEndpointsMap) > 0 {
		if err = setCustomEndpoints(customEndpointsMap, getCustomEndPointKeys(customEndpointsMap)); err != nil {
			return err
		}
	}
	return nil
}

func getCustomEndPointKeys(customEndpointsMap map[string]string) []string {
	keys := make([]string, 0, len(endPointKeyToEnvNameMap))
	for key := range customEndpointsMap {
		keys = append(keys, key)
	}
	return keys
}
