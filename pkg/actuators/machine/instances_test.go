package machine

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"go.uber.org/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/client/mock"

	. "github.com/onsi/gomega"
)

func init() {
	// Add types to scheme
	machinev1beta1.Install(scheme.Scheme)
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
		serviceInstance                machinev1.PowerVSResource
		getCloudServiceInstancesValues *resourcecontrollerv2.ResourceInstance
		getCloudServiceInstancesError  error
		expectedError                  string
	}{
		{
			name: "With ServiceInstanceID",
			serviceInstance: machinev1.PowerVSResource{
				Type: machinev1.PowerVSResourceTypeID,
				ID:   core.StringPtr(instanceID),
			},
		},
		{
			name:         "With valid ServiceInstanceName",
			instanceName: instanceName,
			serviceInstance: machinev1.PowerVSResource{
				Type: machinev1.PowerVSResourceTypeName,
				Name: core.StringPtr(instanceName),
				ID:   core.StringPtr(instanceID),
			},
			getCloudServiceInstancesValues: &resourcecontrollerv2.ResourceInstance{
				GUID: core.StringPtr(instanceID),
				Name: core.StringPtr(instanceName),
			},
		},
		{
			name:         "With zero service instances",
			instanceName: instanceName,
			serviceInstance: machinev1.PowerVSResource{
				Type: machinev1.PowerVSResourceTypeName,
				Name: core.StringPtr(instanceName),
			},
			getCloudServiceInstancesError: fmt.Errorf("does exist any cloud service instance with name %s", instanceName),
			expectedError:                 "does exist any cloud service instance with name testInstanceName",
		},
		{
			name:         "With failed to get service instances",
			instanceName: instanceName,
			serviceInstance: machinev1.PowerVSResource{
				Type: machinev1.PowerVSResourceTypeName,
				Name: core.StringPtr(instanceName),
			},
			getCloudServiceInstancesError: fmt.Errorf("failed to connect to cloud"),
			expectedError:                 "failed to connect to cloud",
		},
		{
			name:          "Without ServiceInstanceName or ID",
			expectedError: "failed to find an ServiceInstanceID, Unexpected serviceInstanceResource type:  supports only ID and Name",
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
