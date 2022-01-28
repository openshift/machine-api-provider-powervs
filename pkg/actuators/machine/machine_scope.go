package machine

import (
	"context"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/power/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	powervsproviderv1 "github.com/openshift/cluster-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	powervsclient "github.com/openshift/cluster-api-provider-powervs/pkg/client"
	"github.com/openshift/cluster-api-provider-powervs/pkg/options"
	machineapierros "github.com/openshift/machine-api-operator/pkg/controller/machine"
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
	machine *machinev1.Machine
	// api server controller runtime client for the openshift-config-managed namespace
	configManagedClient runtimeclient.Client
}

type machineScope struct {
	context.Context

	// powerVSClient for interacting with powervs
	powerVSClient powervsclient.Client

	// api server controller runtime client
	client runtimeclient.Client
	// machine resource
	machine            *machinev1.Machine
	machineToBePatched runtimeclient.Patch
	providerSpec       *powervsproviderv1.PowerVSMachineProviderConfig
	providerStatus     *powervsproviderv1.PowerVSMachineProviderStatus
}

func newMachineScope(params machineScopeParams) (*machineScope, error) {
	providerSpec, err := powervsproviderv1.ProviderSpecFromRawExtension(params.machine.Spec.ProviderSpec.Value)
	if err != nil {
		return nil, machineapierros.InvalidMachineConfiguration("failed to get machine config: %v", err)
	}

	providerStatus, err := powervsproviderv1.ProviderStatusFromRawExtension(params.machine.Status.ProviderStatus)
	if err != nil {
		return nil, machineapierros.InvalidMachineConfiguration("failed to get machine provider status: %v", err.Error())
	}

	credentialsSecretName := ""
	if providerSpec.CredentialsSecret != nil {
		credentialsSecretName = providerSpec.CredentialsSecret.Name
	}

	powerVSClient, err := params.powerVSClientBuilder(params.client, credentialsSecretName, params.machine.Namespace,
		providerSpec.ServiceInstanceID, options.GetDebugMode())
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
	}, nil
}

// Patch patches the machine spec and machine status after reconciling.
func (s *machineScope) patchMachine() error {
	klog.V(3).Infof("%v: patching machine", s.machine.GetName())

	providerStatus, err := powervsproviderv1.RawExtensionFromProviderStatus(s.providerStatus)
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

func (s *machineScope) setProviderStatus(instance *models.PVMInstance, condition powervsproviderv1.PowerVSMachineProviderCondition) {
	klog.Infof("%s: Updating status", s.machine.Name)

	networkAddresses := []corev1.NodeAddress{}

	// TODO: remove 139-141, no need to clean instance id ands state
	// Instance may have existed but been deleted outside our control, clear it's status if so:
	if instance == nil {
		s.providerStatus.InstanceID = nil
		s.providerStatus.InstanceState = nil
	} else {
		s.providerStatus.InstanceID = instance.PvmInstanceID
		s.providerStatus.InstanceState = instance.Status

		for _, i := range instance.Networks {
			if i.ExternalIP != "" {
				networkAddresses = append(networkAddresses,
					corev1.NodeAddress{
						Type:    corev1.NodeExternalIP,
						Address: i.ExternalIP,
					})
			}
			if i.IPAddress != "" {
				networkAddresses = append(networkAddresses,
					corev1.NodeAddress{
						Type:    corev1.NodeInternalIP,
						Address: i.IPAddress,
					})
			}
		}
		networkAddresses = append(networkAddresses,
			corev1.NodeAddress{
				Type:    corev1.NodeInternalDNS,
				Address: *instance.ServerName,
			})
	}
	klog.Infof("%s: finished calculating PowerVS status", s.machine.Name)

	s.machine.Status.Addresses = networkAddresses
	s.providerStatus.Conditions = setPowerVSMachineProviderCondition(condition, s.providerStatus.Conditions)
}
