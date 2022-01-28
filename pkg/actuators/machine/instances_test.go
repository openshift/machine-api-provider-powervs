package machine

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/golang/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	powervsclient "github.com/openshift/cluster-api-provider-powervs/pkg/client"
	"github.com/openshift/cluster-api-provider-powervs/pkg/client/mock"
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
