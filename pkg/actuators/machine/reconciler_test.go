package machine

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/golang/mock/gomock"
	"k8s.io/utils/pointer"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	"github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/client/mock"
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

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, powerVSCredentialsSecret, userDataSecret)
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
	// mock aws API calls
	mockCtrl := gomock.NewController(t)
	mockPowerVSClient := mock.NewMockClient(mockCtrl)
	mockPowerVSClient.EXPECT().GetInstanceByName(gomock.Any()).Return(stubGetInstance(), nil)
	mockPowerVSClient.EXPECT().CreateInstance(gomock.Any()).Return(stubGetInstances(), nil)
	mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubGetInstance(), nil)
	mockPowerVSClient.EXPECT().DeleteInstance(gomock.Any()).Return(nil).AnyTimes()
	mockPowerVSClient.EXPECT().GetImages().Return(stubGetImages(imageNamePrefix, 3), nil)
	mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworks(networkNamePrefix, 3), nil)
	mockPowerVSClient.EXPECT().GetRegion().Return(testRegion).AnyTimes()
	mockPowerVSClient.EXPECT().GetZone().Return(testZone).AnyTimes()

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	testCases := []struct {
		testcase                 string
		providerConfig           *machinev1.PowerVSMachineProviderConfig
		userDataSecret           *corev1.Secret
		powerVSCredentialsSecret *corev1.Secret
		expectedError            error
	}{
		{
			testcase:                 "Create succeed",
			providerConfig:           stubProviderConfig(credSecretName),
			userDataSecret:           stubUserDataSecret(userSecretName),
			powerVSCredentialsSecret: stubPowerVSCredentialsSecret(credSecretName),
			expectedError:            nil,
		},
	}
	for _, tc := range testCases {
		// create fake resources
		t.Logf("testCase: %v", tc.testcase)

		machine, err := stubMachine()
		if err != nil {
			t.Fatal(err)
		}
		encodedProviderConfig, err := RawExtensionFromProviderSpec(tc.providerConfig)
		if err != nil {
			t.Fatalf("Unexpected error")
		}
		providerStatus, err := RawExtensionFromProviderStatus(stubProviderStatus(powerVSProviderID))
		if err != nil {
			t.Fatalf("Failed to set providerStatus")
		}
		machine.Spec.ProviderSpec = machinev1beta1.ProviderSpec{Value: encodedProviderConfig}
		machine.Status.ProviderStatus = providerStatus

		fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, tc.powerVSCredentialsSecret, tc.userDataSecret)

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

		// test create
		err = reconciler.create()
		log.Printf("Error is %v", err)
		if tc.expectedError != nil {
			if err == nil {
				t.Error("reconciler was expected to return error")
			}
			if err != nil && err.Error() != tc.expectedError.Error() {
				t.Errorf("Expected: %v, got %v", tc.expectedError, err)
			}
		} else {
			if err != nil {
				t.Errorf("reconciler was not expected to return error: %v", err)
			}
		}
	}
}

func TestExists(t *testing.T) {
	fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme)
	mockCtrl := gomock.NewController(t)
	mockPowerVSClient := mock.NewMockClient(mockCtrl)

	mockPowerVSClient.EXPECT().GetInstanceByName(gomock.Any()).Return(stubGetInstance(), nil)

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
	})
	if err != nil {
		t.Fatalf("failed to create new machine scope error: %v", err)
	}
	reconciler := newReconciler(machineScope)
	exists, err := reconciler.exists()
	if err != nil || exists != true {
		t.Errorf("reconciler was not expected to return error: %v", err)
	}
}

func TestDelete(t *testing.T) {
	fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme)
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

			fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, powerVSCredentialsSecret, userDataSecret)
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
