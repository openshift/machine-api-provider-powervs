package client

import (
	"context"
	"fmt"
	gohttp "net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/IBM-Cloud/bluemix-go"
	"github.com/IBM-Cloud/bluemix-go/api/resource/resourcev2/controllerv2"
	"github.com/IBM-Cloud/bluemix-go/authentication"
	"github.com/IBM-Cloud/bluemix-go/http"
	bluemixmodels "github.com/IBM-Cloud/bluemix-go/models"
	"github.com/IBM-Cloud/bluemix-go/rest"
	bxsession "github.com/IBM-Cloud/bluemix-go/session"
	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_networks"
	"github.com/IBM-Cloud/power-go-client/power/models"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
	utils "github.com/ppc64le-cloud/powervs-utils"

	corev1 "k8s.io/api/core/v1"
	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	machineapiapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
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

	//PowerServiceType is the power-iaas service type of IBM Cloud
	PowerServiceType = "power-iaas"

	// globalInfrastuctureName default name for infrastructure object
	globalInfrastuctureName = "cluster"

	//regionEnvironmentalVariable is the environmental variable to set the region
	regionEnvironmentalVariable = "IBMCLOUD_REGION"

	//powerIaaSCustomEndpointName is the short name used to fetch Power IaaS endpoint URL
	powerIaaSCustomEndpointName = "pi"
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

//FormatProviderID formats and returns the provided instanceID
func FormatProviderID(instanceID string) string {
	return fmt.Sprintf("ibmpowervs:///%s", instanceID)
}

//PowerVSClientBuilderFuncType is function type for building the Power VS client
type PowerVSClientBuilderFuncType func(client client.Client, secretName, namespace, cloudInstanceID string,
	debug bool) (Client, error)

func apiKeyFromSecret(secret *corev1.Secret) (apiKey string, err error) {
	switch {
	case len(secret.Data["ibmcloud_api_key"]) > 0:
		apiKey = string(secret.Data["ibmcloud_api_key"])
	default:
		return "", fmt.Errorf("invalid secret for powervs credentials")
	}
	return
}

//GetAPIKey will return the api key read from given secretName in a given namespace
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

//NewValidatedClient creates and return a new Power VS client
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, cloudInstanceID string,
	debug bool) (Client, error) {
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

	c.User, err = fetchUserDetails(s, 2)
	if err != nil {
		return c, err
	}

	ctrlv2, err := controllerv2.New(s)
	if err != nil {
		return c, err
	}

	c.ResourceClient = ctrlv2.ResourceServiceInstanceV2()

	resource, err := c.ResourceClient.GetInstance(cloudInstanceID)
	if err != nil {
		return nil, err
	}
	r, err := utils.GetRegion(resource.RegionID)
	if err != nil {
		return nil, err
	}

	//Setting the region here so that corresponding power endpoint can be fetched from it
	err = setEnvironmentVariables(regionEnvironmentalVariable, r)
	if err != nil {
		return nil, err
	}

	zone := resource.RegionID

	c.session, err = ibmpisession.New(c.Config.IAMAccessToken, r, debug, c.User.Account, zone)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	c.InstanceClient = instance.NewIBMPIInstanceClient(ctx, c.session, cloudInstanceID)
	c.NetworkClient = instance.NewIBMPINetworkClient(ctx, c.session, cloudInstanceID)
	c.ImageClient = instance.NewIBMPIImageClient(ctx, c.session, cloudInstanceID)
	return c, err
}

// NewClientMinimal is bare minimal client can be used for quarrying the resources
func NewClientMinimal(ctrlRuntimeClient client.Client, apiKey string) (Client, error) {

	if err := getAndSetServiceEndpoints(ctrlRuntimeClient); err != nil {
		return nil, err
	}

	s, err := bxsession.New(&bluemix.Config{BluemixAPIKey: apiKey})
	if err != nil {
		return nil, err
	}

	c := &powerVSClient{
		Session: s,
	}

	ctrlv2, err := controllerv2.New(s)
	if err != nil {
		return c, err
	}

	c.ResourceClient = ctrlv2.ResourceServiceInstanceV2()

	return c, nil
}

type powerVSClient struct {
	region          string
	zone            string
	cloudInstanceID string

	*bxsession.Session
	User           *User
	ResourceClient controllerv2.ResourceServiceInstanceRepository
	session        *ibmpisession.IBMPISession
	InstanceClient *instance.IBMPIInstanceClient
	NetworkClient  *instance.IBMPINetworkClient
	ImageClient    *instance.IBMPIImageClient
}

func (p *powerVSClient) GetImages() (*models.Images, error) {
	return p.ImageClient.GetAll()
}

func (p *powerVSClient) GetNetworks() (*models.Networks, error) {
	params := p_cloud_networks.NewPcloudNetworksGetallParamsWithTimeout(TIMEOUT).WithCloudInstanceID(p.cloudInstanceID)
	resp, err := p.session.Power.PCloudNetworks.PcloudNetworksGetall(params, ibmpisession.NewAuth(p.session, p.cloudInstanceID))

	if err != nil || resp.Payload == nil {
		return nil, err
	}
	return resp.Payload, nil
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
		return nil, fmt.Errorf("failed to get the instance list")
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

func (p *powerVSClient) GetCloudServiceInstances() ([]bluemixmodels.ServiceInstanceV2, error) {
	var instances []bluemixmodels.ServiceInstanceV2
	svcs, err := p.ResourceClient.ListInstances(controllerv2.ServiceInstanceQuery{
		Type: "service_instance",
	})
	if err != nil {
		return svcs, fmt.Errorf("failed to list the service instances: %v", err)
	}
	for _, svc := range svcs {
		if svc.Crn.ServiceName == PowerServiceType {
			instances = append(instances, svc)
		}
	}
	return instances, nil
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

//User is used to hold the user details
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
