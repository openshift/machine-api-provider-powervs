package machine

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	bluemixmodels "github.com/IBM-Cloud/bluemix-go/models"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/golang/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	powervsproviderv1 "github.com/openshift/machine-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/client/mock"

	. "github.com/onsi/gomega"
)

func init() {
	// Add types to scheme
	machinev1beta1.AddToScheme(scheme.Scheme)
}

func TestRemoveStoppedMachine(t *testing.T) {
	machine, err := stubMachine()
	if err != nil {
		t.Fatalf("Unable to build test machine manifest: %v", err)
	}
	failedInstance := stubGetInstance()
	failed := powervsclient.InstanceStateNameShutoff
	failedInstance.Status = &failed

	cases := []struct {
		name        string
		output      *models.PVMInstance
		err         error
		expectError bool
	}{
		{
			name:        "Get instance by name with error",
			output:      &models.PVMInstance{},
			err:         fmt.Errorf("error getting instance by name"),
			expectError: true,
		},
		{
			name:        "Get instance by name with error Instance Not Found",
			output:      &models.PVMInstance{},
			err:         powervsclient.ErrorInstanceNotFound,
			expectError: false,
		},
		{
			name:        "Get instance with status ACTIVE",
			output:      stubGetInstance(),
			err:         nil,
			expectError: false,
		},
		{
			name:        "Get instance with status FAILED",
			output:      failedInstance,
			err:         nil,
			expectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockPowerVSClient := mock.NewMockClient(mockCtrl)
			mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(tc.output, tc.err).AnyTimes()
			mockPowerVSClient.EXPECT().DeleteInstance(gomock.Any()).Return(nil).AnyTimes()
			err = removeStoppedMachine(machine, mockPowerVSClient)
			if tc.expectError {
				if err == nil {
					t.Fatal("removeStoppedMachine expected to return an error")
				}
			} else {
				if err != nil {
					t.Fatal("removeStoppedMachine is not expected to return an error")
				}
			}
		})
	}
}

func TestLaunchInstance(t *testing.T) {
	machine, err := stubMachine()
	if err != nil {
		t.Fatalf("Unable to build test machine manifest: %v", err)
	}
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	providerConfig := stubProviderConfig(credSecretName)

	cases := []struct {
		name              string
		createInstanceErr error
		instancesErr      error
		expectError       bool
	}{
		{
			name:              "Create instance error",
			createInstanceErr: fmt.Errorf("create instnace failed "),
			instancesErr:      nil,
			expectError:       true,
		},
		{
			name:              "Get instance error",
			createInstanceErr: nil,
			instancesErr:      fmt.Errorf("get instance failed "),
			expectError:       true,
		},
		{
			name:              "Success test",
			createInstanceErr: nil,
			instancesErr:      nil,
			expectError:       false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockPowerVSClient := mock.NewMockClient(mockCtrl)

			//Setup the mocks
			mockPowerVSClient.EXPECT().CreateInstance(gomock.Any()).Return(stubGetInstances(), tc.createInstanceErr).AnyTimes()
			mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubGetInstance(), tc.instancesErr).AnyTimes()
			mockPowerVSClient.EXPECT().GetImages().Return(stubGetImages(imageNamePrefix, 3), nil).AnyTimes()
			mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworks(networkNamePrefix, 3), nil).AnyTimes()

			_, launchErr := launchInstance(machine, providerConfig, nil, mockPowerVSClient)
			t.Log(launchErr)
			if tc.expectError {
				if launchErr == nil {
					t.Errorf("Call to launchInstance did not fail as expected")
				}
			} else {
				if launchErr != nil {
					t.Errorf("Call to launchInstance did not succeed as expected")
				}
			}
		})
	}
}

func TestGetServiceInstanceID(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		name                           string
		instanceName                   string
		serviceInstance                powervsproviderv1.PowerVSResourceReference
		getCloudServiceInstancesValues []bluemixmodels.ServiceInstanceV2
		getCloudServiceInstancesError  error
		expectedError                  string
	}{
		{
			name: "With ServiceInstanceID",
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				ID: core.StringPtr(instanceID),
			},
		},
		{
			name:         "With valid ServiceInstanceName",
			instanceName: instanceName,
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				Name: core.StringPtr(instanceName),
				ID:   core.StringPtr(instanceID),
			},
			getCloudServiceInstancesValues: []bluemixmodels.ServiceInstanceV2{
				{
					ServiceInstance: bluemixmodels.ServiceInstance{
						MetadataType: &bluemixmodels.MetadataType{
							ID: instanceID,
						},
						Name: instanceName,
					},
				},
			},
		},
		{
			name:         "With not existing service instance",
			instanceName: inValidInstance,
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				Name: core.StringPtr(inValidInstance),
			},
			expectedError: "does exist any cloud service instance with name testInValidInstanceName",
		},
		{
			name:         "With two service instance with same name ",
			instanceName: instanceName,
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				Name: core.StringPtr(instanceName),
			},
			getCloudServiceInstancesValues: []bluemixmodels.ServiceInstanceV2{
				{
					ServiceInstance: bluemixmodels.ServiceInstance{
						Name: instanceName,
						MetadataType: &bluemixmodels.MetadataType{
							Guid: instanceGUID,
						},
					},
				},
				{
					ServiceInstance: bluemixmodels.ServiceInstance{
						Name: instanceName,
						MetadataType: &bluemixmodels.MetadataType{
							Guid: "TestGUIDOne",
						},
					},
				},
			},
			expectedError: "there exist more than one service instance ID with with same name testInstanceName, Try setting serviceInstance.ID",
		},
		{
			name:         "With zero service instances",
			instanceName: instanceName,
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				Name: core.StringPtr(instanceName),
			},
			expectedError: "does exist any cloud service instance with name testInstanceName",
		},
		{
			name:         "With failed to get service instances",
			instanceName: instanceName,
			serviceInstance: powervsproviderv1.PowerVSResourceReference{
				Name: core.StringPtr(instanceName),
			},
			getCloudServiceInstancesError: fmt.Errorf("failed to connect to cloud"),
			expectedError:                 "failed to connect to cloud",
		},
		{
			name:          "Without ServiceInstanceName or ID",
			expectedError: "failed to find serviceinstanceID both ServiceInstanceID and ServiceInstanceName can't be nil",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			mockPowerVSClient := mock.NewMockClient(mockCtrl)
			mockPowerVSClient.EXPECT().GetCloudServiceInstanceByName(tc.instanceName).Return(tc.getCloudServiceInstancesValues, tc.getCloudServiceInstancesError).AnyTimes()

			instance, err := getServiceInstanceID(tc.serviceInstance, mockPowerVSClient)
			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("Call to getServiceInstanceID did not fail as expected")
				}
				g.Expect(err).ToNot(BeNil())
				g.Expect(err.Error()).To(BeEquivalentTo(tc.expectedError))
			} else {
				g.Expect(err).To(BeNil())
				if tc.serviceInstance.ID != nil {
					g.Expect(*tc.serviceInstance.ID).To(BeEquivalentTo(*instance))
				}
			}
		})
	}
}
