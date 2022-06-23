package machine

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/power/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineapierros "github.com/openshift/machine-api-operator/pkg/controller/machine"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/options"
)

const (
	userDataSecretKey = "userData"
)

// dhcpDomainKeyName is a variable so we can reference it in unit tests.
var dhcpDomainKeyName = "domain-name"

// machineScopeParams defines the input parameters used to create a new MachineScope.
type machineScopeParams struct {
	context.Context

	powerVSClientBuilder powervsclient.PowerVSClientBuilderFuncType
	// api server controller runtime client
	client runtimeclient.Client
	// machine resource
	machine *machinev1beta1.Machine
	// api server controller runtime client for the openshift-config-managed namespace
	configManagedClient  runtimeclient.Client
	powerVSMinimalClient powervsclient.MinimalPowerVSClientBuilderFuncType
	dhcpIPCacheStore     cache.Store
}

type machineScope struct {
	context.Context

	// powerVSClient for interacting with powervs
	powerVSClient powervsclient.Client

	// api server controller runtime client
	client runtimeclient.Client
	// machine resource
	machine            *machinev1beta1.Machine
	machineToBePatched runtimeclient.Patch
	providerSpec       *machinev1.PowerVSMachineProviderConfig
	providerStatus     *machinev1.PowerVSMachineProviderStatus
	dhcpIPCacheStore   cache.Store
}

func newMachineScope(params machineScopeParams) (*machineScope, error) {
	providerSpec, err := ProviderSpecFromRawExtension(params.machine.Spec.ProviderSpec.Value)
	if err != nil {
		return nil, machineapierros.InvalidMachineConfiguration("failed to get machine config: %v", err)
	}

	providerStatus, err := ProviderStatusFromRawExtension(params.machine.Status.ProviderStatus)
	if err != nil {
		return nil, machineapierros.InvalidMachineConfiguration("failed to get machine provider status: %v", err.Error())
	}

	credentialsSecretName := ""
	if providerSpec.CredentialsSecret != nil {
		credentialsSecretName = providerSpec.CredentialsSecret.Name
	}

	var serviceInstanceID *string
	if providerStatus.ServiceInstanceID != nil {
		serviceInstanceID = providerStatus.ServiceInstanceID
		klog.Infof("Found ServiceInstanceID from providerStatus %s", *serviceInstanceID)
	} else {
		minimalPowerVSClient, err := params.powerVSMinimalClient(params.client)
		if err != nil {
			return nil, machineapierros.InvalidMachineConfiguration("failed creating minimalPowerVS client Error: %v", err)
		}
		serviceInstanceID, err = getServiceInstanceID(providerSpec.ServiceInstance, minimalPowerVSClient)
		if err != nil {
			return nil, machineapierros.InvalidMachineConfiguration("error getting ServiceInstance ID, Error: %v", err)
		}
		// Setting ServiceInstanceID here so that in the next iteration it will be fetched early
		providerStatus.ServiceInstanceID = serviceInstanceID
	}

	powerVSClient, err := params.powerVSClientBuilder(params.client, credentialsSecretName, params.machine.Namespace,
		*serviceInstanceID, options.GetDebugMode())
	if err != nil {
		return nil, machineapierros.InvalidMachineConfiguration("failed to create powervs client: %v", err.Error())
	}

	return &machineScope{
		Context:            params.Context,
		powerVSClient:      powerVSClient,
		client:             params.client,
		machine:            params.machine,
		machineToBePatched: runtimeclient.MergeFrom(params.machine.DeepCopy()),
		providerSpec:       providerSpec,
		providerStatus:     providerStatus,
		dhcpIPCacheStore:   params.dhcpIPCacheStore,
	}, nil
}

// Patch patches the machine spec and machine status after reconciling.
func (s *machineScope) patchMachine() error {
	klog.V(3).Infof("%v: patching machine", s.machine.GetName())

	providerStatus, err := RawExtensionFromProviderStatus(s.providerStatus)
	if err != nil {
		return machineapierros.InvalidMachineConfiguration("failed to get machine provider status: %v", err.Error())
	}
	s.machine.Status.ProviderStatus = providerStatus

	statusCopy := *s.machine.Status.DeepCopy()

	// patch machine
	if err := s.client.Patch(context.Background(), s.machine, s.machineToBePatched); err != nil {
		klog.Errorf("Failed to patch machine %q: %v", s.machine.GetName(), err)
		return err
	}

	s.machine.Status = statusCopy

	// patch status
	if err := s.client.Status().Patch(context.Background(), s.machine, s.machineToBePatched); err != nil {
		klog.Errorf("Failed to patch machine status %q: %v", s.machine.GetName(), err)
		return err
	}

	return nil
}

// getUserData fetches the user-data from the secret referenced in the Machine's
// provider spec, if one is set.
func (s *machineScope) getUserData() ([]byte, error) {
	if s.providerSpec == nil || s.providerSpec.UserDataSecret == nil {
		return nil, nil
	}

	userDataSecret := &corev1.Secret{}

	objKey := runtimeclient.ObjectKey{
		Namespace: s.machine.Namespace,
		Name:      s.providerSpec.UserDataSecret.Name,
	}

	if err := s.client.Get(s.Context, objKey, userDataSecret); err != nil {
		return nil, err
	}

	userData, exists := userDataSecret.Data[userDataSecretKey]
	if !exists {
		return nil, fmt.Errorf("secret %s missing %s key", objKey, userDataSecretKey)
	}

	return userData, nil
}

func (s *machineScope) setProviderStatus(instance *models.PVMInstance, condition metav1.Condition) {
	klog.Infof("%s: Updating status", s.machine.Name)
	// TODO: remove 139-141, no need to clean instance id ands state
	// Instance may have existed but been deleted outside our control, clear it's status if so:
	if instance == nil {
		s.providerStatus.InstanceID = nil
		s.providerStatus.InstanceState = nil
	} else {
		s.providerStatus.InstanceID = instance.PvmInstanceID
		s.providerStatus.InstanceState = instance.Status
	}
	klog.Infof("%s: finished calculating PowerVS status", s.machine.Name)
	s.providerStatus.Conditions = setPowerVSMachineProviderCondition(condition, s.providerStatus.Conditions)
}
