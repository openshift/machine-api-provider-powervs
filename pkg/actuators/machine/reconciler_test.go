package machine

import (
	"errors"
	"fmt"
	"log"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	"github.com/openshift/cluster-api-provider-powervs/pkg/client"
	"github.com/openshift/cluster-api-provider-powervs/pkg/client/mock"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
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
		providerStatus    v1alpha1.PowerVSMachineProviderStatus
		powerVSClientFunc func(*gomock.Controller) client.Client
		exists            bool
	}{
		{
			testcase:       "get-instances",
			providerStatus: v1alpha1.PowerVSMachineProviderStatus{},
			powerVSClientFunc: func(ctrl *gomock.Controller) client.Client {
				mockPowerVSClient := mock.NewMockClient(ctrl)
				mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil)
				return mockPowerVSClient
			},
			exists: true,
		},
		{
			testcase: "has-status-search-by-id-running",
			providerStatus: v1alpha1.PowerVSMachineProviderStatus{
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
			providerStatus: v1alpha1.PowerVSMachineProviderStatus{
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

			powerVAStatusRaw, err := v1alpha1.RawExtensionFromProviderStatus(&tc.providerStatus)
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

	r := Reconciler{
		machineScope: &machineScope{
			machine: &machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{},
			},
			providerSpec: &v1alpha1.PowerVSMachineProviderConfig{},
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

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	testCases := []struct {
		testcase                 string
		providerConfig           *v1alpha1.PowerVSMachineProviderConfig
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

		encodedProviderConfig, err := v1alpha1.RawExtensionFromProviderSpec(tc.providerConfig)
		if err != nil {
			t.Fatalf("Unexpected error")
		}
		machine, err := stubMachine()
		if err != nil {
			t.Fatal(err)
		}
		machine.Spec.ProviderSpec = machinev1beta1.ProviderSpec{Value: encodedProviderConfig}

		fakeClient := fake.NewFakeClientWithScheme(scheme.Scheme, machine, tc.powerVSCredentialsSecret, tc.userDataSecret)

		machineScope, err := newMachineScope(machineScopeParams{
			client:  fakeClient,
			machine: machine,
			powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
				debug bool) (client.Client, error) {
				return mockPowerVSClient, nil
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

	machineScope, err := newMachineScope(machineScopeParams{
		client:  fakeClient,
		machine: &machinev1beta1.Machine{},
		powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
			debug bool) (client.Client, error) {
			return mockPowerVSClient, nil
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

	machineScope, err := newMachineScope(machineScopeParams{
		client:  fakeClient,
		machine: &machinev1beta1.Machine{},
		powerVSClientBuilder: func(client runtimeclient.Client, secretName, namespace, cloudInstanceID string,
			debug bool) (client.Client, error) {
			return mockPowerVSClient, nil
		},
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
