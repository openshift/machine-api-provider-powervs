package machine

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	"github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/client/mock"

	. "github.com/onsi/gomega"
)

func init() {
	// Add types to scheme
	machinev1beta1.AddToScheme(scheme.Scheme)
}

func TestGetMachineInstances(t *testing.T) {
	instanceID := "powerVSInstance"

	machine, err := stubMachine()
	if err != nil {
		t.Fatalf("unable to build stub machine: %v", err)
	}

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	testCases := []struct {
		testcase          string
		providerStatus    machinev1.PowerVSMachineProviderStatus
		powerVSClientFunc func(*gomock.Controller) client.Client
		exists            bool
	}{
		{
			testcase:       "get-instances",
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase: "has-status-search-by-id-running",
			providerStatus: machinev1.PowerVSMachineProviderStatus{
				InstanceID: &instanceID,
			},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubGetInstance(), nil).Times(1)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase: "has-status-search-by-id-terminated",
			providerStatus: machinev1.PowerVSMachineProviderStatus{
				InstanceID: &instanceID,
			},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)

				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(nil,
					errors.New("intentional error ")).Times(1).Times(1)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(nil,
					errors.New("intentional error ")).Times(1)

				return mockPowerVSClient
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			powerVAStatusRaw, err := RawExtensionFromProviderStatus(&tc.providerStatus)
			if err != nil {
				t.Fatal(err)
			}

			machineCopy := machine.DeepCopy()
			machineCopy.Status.ProviderStatus = powerVAStatusRaw

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machineCopy,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			_, err = reconciler.getMachineInstance()
			if err != nil && tc.exists {
				t.Errorf("Unexpected error from getMachineInstance: %v", err)
			}
		})
	}
}

func TestSetMachineCloudProviderSpecifics(t *testing.T) {
	testStatus := "testStatus"
	mockCtrl := gomock.NewController(t)
	mockPowerVSClient := mock.NewMockClient(mockCtrl)
	mockPowerVSClient.EXPECT().GetRegion().Return(testRegion)
	mockPowerVSClient.EXPECT().GetZone().Return(testZone)

	r := Reconciler{
		machineScope: &machineScope{
			machine: &machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			providerSpec:  &machinev1.PowerVSMachineProviderConfig{},
			powerVSClient: mockPowerVSClient,
		},
	}
	instance := &models.PVMInstance{
		Status: &testStatus,
	}

	r.setMachineCloudProviderSpecifics(instance)

	actualInstanceStateAnnotation := r.machine.Annotations[machinecontroller.MachineInstanceStateAnnotationName]
	if actualInstanceStateAnnotation != *instance.Status {
		t.Errorf("Expected instance state annotation: %v, got: %v", actualInstanceStateAnnotation, instance.Status)
	}

	machineRegionLabel := r.machine.Labels[machinecontroller.MachineRegionLabelName]
	if machineRegionLabel != testRegion {
		t.Errorf("Expected machine %s label value as %s: but got: %s", machinecontroller.MachineRegionLabelName, testRegion, machineRegionLabel)
	}

	machineZoneLabel := r.machine.Labels[machinecontroller.MachineAZLabelName]
	if machineRegionLabel != testRegion {
		t.Errorf("Expected machine %s label value as %s: but got: %s", machinecontroller.MachineAZLabelName, testZone, machineZoneLabel)
	}
}

func TestCreate(t *testing.T) {
	defaultMachineFunc := func() *machinev1beta1.Machine {
		machine, err := stubMachine()
		if err != nil {
			t.Fatalf("unable to build stub machine: %v", err)
		}
		return machine
	}

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	testCases := []struct {
		testcase          string
		machineFunc       func() *machinev1beta1.Machine
		providerConfig    *machinev1.PowerVSMachineProviderConfig
		providerStatus    machinev1.PowerVSMachineProviderStatus
		powerVSClientFunc func(*gomock.Controller) client.Client
		providerID        string
		expectedCondition metav1.Condition
		expectError       bool
	}{
		{
			testcase: "with invalid machine object",
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubMachine()
				if err != nil {
					t.Fatalf("unable to build stub machine: %v", err)
				}
				delete(machine.Labels, machinev1beta1.MachineClusterIDLabel)
				return machine
			},
			providerConfig: stubProviderConfig(credSecretName),
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			expectError: true,
		},
		{
			testcase:       "with create machine success",
			machineFunc:    defaultMachineFunc,
			providerConfig: stubProviderConfig(credSecretName),
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(gomock.Any()).Return(stubInstanceWithBuildState(), nil)
				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubInstanceWithBuildState(), nil)
				mockPowerVSClient.EXPECT().CreateInstance(gomock.Any()).Return(stubGetInstances(), nil)
				mockPowerVSClient.EXPECT().GetImages().Return(stubGetImages(imageNamePrefix, 3), nil)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworks(networkNamePrefix, 3), nil)
				return mockPowerVSClient
			},
			providerID:        *stubInstanceWithBuildState().PvmInstanceID,
			expectedCondition: conditionBuild(),
			expectError:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			g := NewWithT(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			providerConfig, err := RawExtensionFromProviderSpec(tc.providerConfig)
			if err != nil {
				t.Fatalf("Unexpected error")
			}

			providerStatus, err := RawExtensionFromProviderStatus(&tc.providerStatus)
			if err != nil {
				t.Fatal(err)
			}

			machine := tc.machineFunc()
			machine.Spec.ProviderSpec = machinev1beta1.ProviderSpec{Value: providerConfig}
			machine.Status.ProviderStatus = providerStatus

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machine,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			err = reconciler.create()
			if tc.expectError {
				if err == nil {
					t.Fatal("create machine expected to return an error")
				}
				if reconciler.providerStatus != nil && reconciler.providerStatus.InstanceID != nil {
					if *reconciler.providerStatus.InstanceID != tc.providerID {
						t.Error("reconciler did not set proper instance id")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error while creating machine: %v", err)
				}
			}
			if tc.expectedCondition.Type != "" {
				g.Expect(reconciler.providerStatus.Conditions).Should(HaveEach(
					SatisfyAll(
						HaveField("Type", tc.expectedCondition.Type),
						HaveField("Status", tc.expectedCondition.Status),
						HaveField("Reason", tc.expectedCondition.Reason),
						HaveField("Message", tc.expectedCondition.Message),
					)))
			}
		})
	}
}

func TestExists(t *testing.T) {
	instanceID := "powerVSInstance"

	machine, err := stubMachine()
	if err != nil {
		t.Fatalf("unable to build stub machine: %v", err)
	}

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	testCases := []struct {
		testcase          string
		providerStatus    machinev1.PowerVSMachineProviderStatus
		powerVSClientFunc func(*gomock.Controller) client.Client
		exists            bool
		expectError       bool
	}{
		{
			testcase:       "with get instance by name",
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase:       "with error getting instances by name",
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), fmt.Errorf("intentional error"))
				return mockPowerVSClient
			},
			expectError: true,
		},
		{
			testcase:       "with get instance returning known error",
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), fmt.Errorf("instance Not Found"))
				return mockPowerVSClient
			},
			expectError: true,
		},
		{
			testcase:       "with get instance returning no instances",
			providerStatus: machinev1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(nil, nil)
				return mockPowerVSClient
			},
		},
		{
			testcase: "get instances by id",
			providerStatus: machinev1.PowerVSMachineProviderStatus{
				InstanceID: &instanceID,
			},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubGetInstance(), nil).Times(1)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase: "with get instances by id returning error",
			providerStatus: machinev1.PowerVSMachineProviderStatus{
				InstanceID: &instanceID,
			},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(nil, fmt.Errorf("intentional error")).Times(1)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase: "with both get instance by id and name returning error",
			providerStatus: machinev1.PowerVSMachineProviderStatus{
				InstanceID: &instanceID,
			},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)

				mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(nil,
					errors.New("intentional error ")).Times(1).Times(1)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(nil,
					errors.New("intentional error ")).Times(1)

				return mockPowerVSClient
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			powerVSStatusRaw, err := RawExtensionFromProviderStatus(&tc.providerStatus)
			if err != nil {
				t.Fatal(err)
			}

			machineCopy := machine.DeepCopy()
			machineCopy.Status.ProviderStatus = powerVSStatusRaw

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machineCopy,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			exists, err := reconciler.exists()
			if tc.expectError {
				if err == nil {
					t.Fatal("checking machine exists expected to return an error")
				}
			} else {
				if err != nil || tc.exists != exists {
					t.Errorf("Unexpected error while checking machine exists: %v", err)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	mockCtrl := gomock.NewController(t)
	mockPowerVSClient := mock.NewMockClient(mockCtrl)

	mockPowerVSClient.EXPECT().GetInstanceByName(gomock.Any()).Return(stubGetInstance(), nil)
	mockPowerVSClient.EXPECT().DeleteInstance(gomock.Any()).Return(nil)

	machine, err := stubMachine()
	if err != nil {
		t.Fatal(err)
	}

	machineScope, err := newMachineScope(machineScopeParams{
		client:  fakeClient,
		machine: machine,
		powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
			debug bool) (client.Client, error) {
			return mockPowerVSClient, nil
		},
		powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
			return nil, nil
		},
		dhcpIPCacheStore: cache.NewTTLStore(CacheKeyFunc, CacheTTL),
	})
	if err != nil {
		t.Fatalf("failed to create new machine scope error: %v", err)
	}
	reconciler := newReconciler(machineScope)
	if err := reconciler.delete(); err != nil {
		if _, ok := err.(*machinecontroller.RequeueAfterError); !ok {
			t.Errorf("reconciler was not expected to return error: %v", err)
		}
	}
}

func TestSetMachineAddresses(t *testing.T) {
	instanceName := "test_vm"
	networkName := "test-network-1"
	networkID := "56a112ff-b2d7-4236-ad7b-c5772ffe9cb5"
	dhcpServerID := "45a112gg-b2d7-4336-ad7b-cdf72f34e9cb5"
	leaseIP := "192.168.0.10"
	instanceMac := "ff:11:33:dd:00:22"

	machine, err := stubMachine()
	if err != nil {
		t.Fatalf("unable to build stub machine: %v", err)
	}

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	defaultDhcpCacheStoreFunc := func() cache.Store {
		return cache.NewTTLStore(CacheKeyFunc, CacheTTL)
	}
	defaultExpectedMachineAddress := []corev1.NodeAddress{
		{
			Type:    corev1.NodeInternalDNS,
			Address: instanceName,
		},
	}

	testCases := []struct {
		testcase               string
		powerVSClientFunc      func(*gomock.Controller) client.Client
		pvmInstance            *models.PVMInstance
		expectedMachineAddress []corev1.NodeAddress
		expectedError          error
		dhcpCacheStoreFunc     func() cache.Store
	}{
		{
			testcase: "when instance is nil",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			pvmInstance:        nil,
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "with instance external ip set",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				Networks: []*models.PVMInstanceNetwork{
					{
						ExternalIP: "10.11.2.3",
					},
				},
				ServerName: pointer.StringPtr(instanceName),
			},
			expectedMachineAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeExternalIP,
				Address: "10.11.2.3",
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "with instance internal ip set",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "192.168.1.2",
					},
				},
				ServerName: pointer.StringPtr(instanceName),
			},
			expectedMachineAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: "192.168.1.2",
			}),
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
		},
		{
			testcase: "error while getting network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), fmt.Errorf("intentional error")).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("failed to fetch network id from network resource for VM: test error: intentional error"),
		},
		{
			testcase: "no network id associated with network name",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName("test-network", networkID), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("failed to fetch network id from network resource for VM: test error: failed to find an network ID with name %s", networkName),
		},
		{
			testcase: "not able to find network matching vm network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("failed to get network attached to VM test_vm with id %s", networkID),
		},
		{
			testcase: "error on fetching DHCP server details",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, networkID), fmt.Errorf("intentional error")).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "",
						NetworkID: networkID,
					},
				},
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("failed to get DHCP server error: intentional error"),
		},
		{
			testcase: "DHCP server details not found associated to network id",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, "networkID"), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress: "",
						NetworkID: networkID,
					},
				},
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("DHCP server detailis not found for network with ID %s", networkID),
		},
		{
			testcase: "error on getting DHCP server details",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServerByID(dhcpServerID).Return(stubGetDHCPServerByID(dhcpServerID, leaseIP, instanceMac),
					fmt.Errorf("intentional error")).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "",
						NetworkID:  networkID,
						MacAddress: instanceMac,
					},
				},
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError:          fmt.Errorf("failed to get DHCP server details with DHCP server ID: %s error: intentional error", dhcpServerID),
		},
		{
			testcase: "DHCP lease does not have lease for instance",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServerByID(dhcpServerID).Return(stubGetDHCPServerByID(dhcpServerID, leaseIP, "ff:11:33:dd:00:33"), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "",
						NetworkID:  networkID,
						MacAddress: instanceMac,
					},
				},
			},
			expectedMachineAddress: defaultExpectedMachineAddress,
			dhcpCacheStoreFunc:     defaultDhcpCacheStoreFunc,
			expectedError: fmt.Errorf("failed to get internal IP, DHCP lease not found for VM test_vm with MAC %s in DHCP network %s",
				"ff:11:33:dd:00:22", dhcpServerID),
		},
		{
			testcase: "success in fetching DHCP IP from server",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServerByID(dhcpServerID).Return(stubGetDHCPServerByID(dhcpServerID, leaseIP, instanceMac), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "",
						NetworkID:  networkID,
						MacAddress: instanceMac,
					},
				},
			},
			dhcpCacheStoreFunc: defaultDhcpCacheStoreFunc,
			expectedMachineAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
		},
		{
			testcase: "ip in cache expired, Fetch from dhcp server",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworkWithName(networkName, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServers().Return(stubGetDHCPServers(dhcpServerID, networkID), nil).Times(1)
				mockPowerVSClient.EXPECT().GetDHCPServerByID(dhcpServerID).Return(stubGetDHCPServerByID(dhcpServerID, leaseIP, instanceMac), nil).Times(1)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "",
						NetworkID:  networkID,
						MacAddress: instanceMac,
					},
				},
			},
			dhcpCacheStoreFunc: func() cache.Store {
				cacheStore := cache.NewTTLStore(CacheKeyFunc, time.Second)
				cacheStore.Add(vmIP{
					name: instanceName,
					ip:   leaseIP,
				})
				time.Sleep(time.Second)
				return cacheStore
			},
			expectedMachineAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
		},
		{
			testcase: "success in fetching DHCP IP from cache",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			pvmInstance: &models.PVMInstance{
				ServerName: pointer.StringPtr(instanceName),
				Networks: []*models.PVMInstanceNetwork{
					{
						IPAddress:  "",
						NetworkID:  networkID,
						MacAddress: instanceMac,
					},
				},
			},
			dhcpCacheStoreFunc: func() cache.Store {
				cacheStore := cache.NewTTLStore(CacheKeyFunc, CacheTTL)
				cacheStore.Add(vmIP{
					name: instanceName,
					ip:   leaseIP,
				})
				return cacheStore
			},
			expectedMachineAddress: append(defaultExpectedMachineAddress, corev1.NodeAddress{
				Type:    corev1.NodeInternalIP,
				Address: leaseIP,
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			machineCopy := machine.DeepCopy()
			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machineCopy,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
				dhcpIPCacheStore: tc.dhcpCacheStoreFunc(),
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			err = reconciler.setMachineAddresses(tc.pvmInstance)
			if err != nil && tc.expectedError == nil {
				t.Errorf("Unexpected error from getMachineInstance: %v", err)
			}
			if tc.expectedError != nil {
				if err == nil {
					t.Fatal("Expecting error but got nil")
				}
				if !reflect.DeepEqual(tc.expectedError.Error(), err.Error()) {
					t.Errorf("expected %v, got: %v", tc.expectedError.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}

			if tc.expectedMachineAddress != nil {
				if !reflect.DeepEqual(machineCopy.Status.Addresses, tc.expectedMachineAddress) {
					t.Errorf("expected %v, got: %v", tc.expectedMachineAddress, machineCopy.Status.Addresses)
				}
			}
		})
	}
}

func TestUpdateLoadBalancers(t *testing.T) {
	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	testCases := []struct {
		testcase          string
		powerVSClientFunc func(*gomock.Controller) client.Client
		expectedError     error
		machineFunc       func() *machinev1beta1.Machine
		internalIP        string
	}{
		{
			testcase: "when loadBalancer is not configured",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine(nil, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase: "when different loadBalancer type is specified",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-loadBalancer"}, "network")
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase: "when listLoadBalancer fails",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to list loadBalancers")).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: failed to get loadbalancers details from cloud error listing loadbalancer failed to list loadBalancers"),
		},
		{
			testcase: "when there no loadBalancers in cloud",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(nil, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: failed to get loadbalancers details from cloud no loadbalancer is retrieved"),
		},
		{
			testcase: "when configured loadBalancer not present in cloud",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name: pointer.String("name"),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: failed to get loadbalancers details from cloud not able to find all [test-lb] loadbalancer in cloud"),
		},
		{
			testcase: "when loadBalancer not in active state",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String("busy"),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: cannot update load balancer test-lb, load balancer is not in active state"),
		},
		{
			testcase: "when configured loadBalancer does not have pool",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String(loadBalancerActiveState),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: no pools exist for the load balancer test-lb"),
		},
		{
			testcase: "failed to list loadBalancer pool",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to get pool from cloud")).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: failed to list pool-name LoadBalancer pool error: failed to get pool from cloud"),
		},
		{
			testcase:   "internal IP already registered in loadBalancer pool",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
							Target: &vpcv1.LoadBalancerPoolMemberTarget{
								Address: pointer.String("192.168.0.11"),
							},
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase:   "failed to get loadBalancer details",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to get loadbalancer"))
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: error getting loadbalancer details with id: id error: failed to get loadbalancer"),
		},
		{
			testcase:   "failed to update pool member, loadBalancer is not in active state",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					Name:               pointer.String("test-lb"),
					ID:                 pointer.String("id"),
					ProvisioningStatus: pointer.String("busy"),
				}, nil, nil)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: requeue in: 20s"),
		},
		{
			testcase:   "failed to create loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(stubGetLoadBalancerResult(), nil, nil)
				mockPowerVSClient.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to create pool memeber"))
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("test-vm: Failed to register application load balancers: error creating LoadBalacner pool member failed to create pool memeber"),
		},
		{
			testcase:   "successfully to create loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(stubGetLoadBalancerResult(), nil, nil)
				mockPowerVSClient.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(nil, nil, nil)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase:   "successfully to create multiple loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String(loadBalancerActiveState),
							Pools: []vpcv1.LoadBalancerPoolReference{
								{
									ID:   pointer.String("pool-id"),
									Name: pointer.String("pool-name"),
								},
								{
									ID:   pointer.String("pool-id-2"),
									Name: pointer.String("pool-name-2"),
								},
							},
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
						},
					},
				}, nil, nil).Times(2)
				mockPowerVSClient.EXPECT().GetLoadBalancer(gomock.Any()).Return(stubGetLoadBalancerResult(), nil, nil).Times(2)
				mockPowerVSClient.EXPECT().CreateLoadBalancerPoolMember(gomock.Any()).Return(nil, nil, nil).Times(2)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			machine := tc.machineFunc()

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machine,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			err = reconciler.updateLoadBalancers(tc.internalIP)
			if err != nil && tc.expectedError == nil {
				t.Errorf("Unexpected error from updateLoadBalancers: %v", err)
			}
			if tc.expectedError != nil {
				if err == nil {
					t.Fatal("Expecting error but got nil")
				}
				if !reflect.DeepEqual(tc.expectedError.Error(), err.Error()) {
					t.Errorf("expected %v, got: %v", tc.expectedError.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}

func TestRemoveFromApplicationLoadBalancer(t *testing.T) {
	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	userDataSecret := stubUserDataSecret(userSecretName)

	testCases := []struct {
		testcase          string
		powerVSClientFunc func(*gomock.Controller) client.Client
		expectedError     error
		machineFunc       func() *machinev1beta1.Machine
		internalIP        string
	}{
		{
			testcase: "when loadBalancer is not configured",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine(nil, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase: "when different loadBalancer type is specified",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-loadBalancer"}, "network")
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase: "when listLoadBalancer fails",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to list loadBalancers")).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("failed to get loadbalancers details from cloud error listing loadbalancer failed to list loadBalancers"),
		},
		{
			testcase: "when there no loadBalancers in cloud",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(nil, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("failed to get loadbalancers details from cloud no loadbalancer is retrieved"),
		},
		{
			testcase: "when configured loadBalancer not present in cloud",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name: pointer.String("name"),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("failed to get loadbalancers details from cloud not able to find all [test-lb] loadbalancer in cloud"),
		},
		{
			testcase: "when loadBalancer not in active state",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String("busy"),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("cannot update load balancer test-lb, load balancer is not in active state"),
		},
		{
			testcase: "when configured loadBalancer does not have pool",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String(loadBalancerActiveState),
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("no pools exist for the load balancer test-lb"),
		},
		{
			testcase: "failed to list loadBalancer pool",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(nil, nil, fmt.Errorf("failed to get pool from cloud")).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("failed to list pool-name LoadBalancer pool error: failed to get pool from cloud"),
		},
		{
			testcase:   "internal IP not registered in loadBalancer pool",
			internalIP: "192.168.0.10",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							Port: pointer.Int64(6443),
							Target: &vpcv1.LoadBalancerPoolMemberTarget{
								Address: pointer.String("192.168.0.11"),
							},
						},
					},
				}, nil, nil).Times(1)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase:   "failed to delete loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							ID:   pointer.String("test-id"),
							Port: pointer.Int64(6443),
							Target: &vpcv1.LoadBalancerPoolMemberTarget{
								Address: pointer.String("192.168.0.11"),
							},
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().DeleteLoadBalancerPoolMember(gomock.Any()).Return(nil, fmt.Errorf("failed to delete pool memeber"))
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
			expectedError: fmt.Errorf("error deleting LoadBalacner pool member failed to delete pool memeber"),
		},
		{
			testcase:   "successfully to delete loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(stubGetLoadBalancerCollections(), nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							ID:   pointer.String("test-id"),
							Port: pointer.Int64(6443),
							Target: &vpcv1.LoadBalancerPoolMemberTarget{
								Address: pointer.String("192.168.0.11"),
							},
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().DeleteLoadBalancerPoolMember(gomock.Any()).Return(nil, nil)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
		{
			testcase:   "successfully to delete multiple loadBalancer pool member",
			internalIP: "192.168.0.11",
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().ListLoadBalancers(gomock.Any()).Return(&vpcv1.LoadBalancerCollection{
					LoadBalancers: []vpcv1.LoadBalancer{
						{
							Name:               pointer.String("test-lb"),
							ID:                 pointer.String("id"),
							ProvisioningStatus: pointer.String(loadBalancerActiveState),
							Pools: []vpcv1.LoadBalancerPoolReference{
								{
									ID:   pointer.String("pool-id"),
									Name: pointer.String("pool-name"),
								},
								{
									ID:   pointer.String("pool-id-2"),
									Name: pointer.String("pool-name-2"),
								},
							},
						},
					},
				}, nil, nil).Times(1)
				mockPowerVSClient.EXPECT().ListLoadBalancerPoolMembers(gomock.Any()).Return(&vpcv1.LoadBalancerPoolMemberCollection{
					Members: []vpcv1.LoadBalancerPoolMember{
						{
							ID:   pointer.String("test-id"),
							Port: pointer.Int64(6443),
							Target: &vpcv1.LoadBalancerPoolMemberTarget{
								Address: pointer.String("192.168.0.11"),
							},
						},
					},
				}, nil, nil).Times(2)
				mockPowerVSClient.EXPECT().DeleteLoadBalancerPoolMember(gomock.Any()).Return(nil, nil).Times(2)
				return mockPowerVSClient
			},
			machineFunc: func() *machinev1beta1.Machine {
				machine, err := stubControlPlaneMachine([]string{"test-lb"}, machinev1.ApplicationLoadBalancerType)
				if err != nil {
					t.Fatalf("unable to build stub control plane machine: %v", err)
				}
				return machine
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testcase, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			machine := tc.machineFunc()

			fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(machine, powerVSCredentialsSecret, userDataSecret).Build()
			mockPowerVSClient := tc.powerVSClientFunc(ctrl)

			machineScope, err := newMachineScope(machineScopeParams{
				client:  fakeClient,
				machine: machine,
				powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (client.Client, error) {
					return mockPowerVSClient, nil
				},
				powerVSMinimalClient: func(client runtimeclient.Client) (client.Client, error) {
					return nil, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			reconciler := newReconciler(machineScope)

			err = reconciler.removeFromApplicationLoadBalancers(tc.internalIP)
			if err != nil && tc.expectedError == nil {
				t.Errorf("Unexpected error from updateLoadBalancers: %v", err)
			}
			if tc.expectedError != nil {
				if err == nil {
					t.Fatal("Expecting error but got nil")
				}
				if !reflect.DeepEqual(tc.expectedError.Error(), err.Error()) {
					t.Errorf("expected %v, got: %v", tc.expectedError.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}
