package machine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	powervsClient "github.com/openshift/cluster-api-provider-powervs/pkg/client"
	"github.com/openshift/cluster-api-provider-powervs/pkg/client/mock"

	. "github.com/onsi/gomega"
)

func init() {
	// Add types to scheme
	machinev1beta1.AddToScheme(scheme.Scheme)
}

func TestActuatorEvents(t *testing.T) {
	g := NewWithT(t)

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	userDataSecret := stubUserDataSecret(userSecretName)
	g.Expect(k8sClient.Create(context.Background(), userDataSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.Background(), userDataSecret)).To(Succeed())
	}()

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	credentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	g.Expect(k8sClient.Create(context.Background(), credentialsSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.Background(), credentialsSecret)).To(Succeed())
	}()

	credSecretName = fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	providerSpec, err := v1alpha1.RawExtensionFromProviderSpec(stubProviderConfig(credSecretName))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(providerSpec).ToNot(BeNil())

	cases := []struct {
		name      string
		error     string
		operation func(actuator *Actuator, machine *machinev1beta1.Machine)
		event     string
	}{
		{
			name: "Create machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				machine.Spec = machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{
							Raw: []byte{'1'},
						},
					},
				}
				actuator.Create(context.Background(), machine)
			},

			event: "test: failed to create scope for machine: failed to get machine config: error unmarshalling providerSpec: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go value of type v1alpha1.PowerVSMachineProviderConfig",
		},

		{
			name: "Create machine event failed, reconciler's create failed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				machine.Labels[machinev1beta1.MachineClusterIDLabel] = ""
				actuator.Create(context.Background(), machine)
			},
			event: "test: reconciler failed to Create machine: test: failed validating machine provider spec: test: missing \"machine.openshift.io/cluster-api-cluster\" label",
		},
		{
			name: "Create machine event succeed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Create(context.Background(), machine)
			},
			event: "Created Machine test",
		},
		{
			name: "Update machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				machine.Spec = machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{
							Raw: []byte{'1'},
						},
					},
				}
				actuator.Update(context.Background(), machine)
			},
			event: "test: failed to create scope for machine: failed to get machine config: error unmarshalling providerSpec: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go value of type v1alpha1.PowerVSMachineProviderConfig",
		},
		{
			name: "Update machine event failed, reconciler's update failed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				machine.Labels[machinev1beta1.MachineClusterIDLabel] = ""
				actuator.Update(context.Background(), machine)
			},
			event: "test: reconciler failed to Update machine: test: failed validating machine provider spec: test: missing \"machine.openshift.io/cluster-api-cluster\" label",
		},
		{
			name: "Update machine event succeed and only one event is created",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Update(context.Background(), machine)
				actuator.Update(context.Background(), machine)
			},
			event: "Updated Machine test",
		},
		{
			name: "Delete machine event failed on invalid machine scope",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				machine.Spec = machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{
							Raw: []byte{'1'},
						},
					},
				}
				actuator.Delete(context.Background(), machine)
			},
			event: "test: failed to create scope for machine: failed to get machine config: error unmarshalling providerSpec: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go value of type v1alpha1.PowerVSMachineProviderConfig",
		},
		{
			name: "Delete machine event succeed",
			operation: func(actuator *Actuator, machine *machinev1beta1.Machine) {
				actuator.Delete(context.Background(), machine)
			},
			event: "Deleted machine test",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewWithT(t)

			machine := &machinev1beta1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: defaultNamespace,
					Labels: map[string]string{
						machinev1beta1.MachineClusterIDLabel: "CLUSTERID",
					},
				},
				Spec: machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: providerSpec,
					},
				}}

			// Create the machine
			gs.Expect(k8sClient.Create(context.Background(), machine)).To(Succeed())
			defer func() {
				gs.Expect(k8sClient.Delete(context.Background(), machine)).To(Succeed())
			}()

			// Ensure the machine has synced to the cache
			getMachine := func() error {
				machineKey := types.NamespacedName{Namespace: machine.Namespace, Name: machine.Name}
				return k8sClient.Get(context.Background(), machineKey, machine)
			}
			gs.Eventually(getMachine, timeout).Should(Succeed())

			mockCtrl := gomock.NewController(t)
			mockPowerVSClient := mock.NewMockClient(mockCtrl)

			powerVSClientBuilder := func(client client.Client, secretName, namespace, cloudInstanceID string,
				debug bool) (powervsClient.Client, error) {
				return mockPowerVSClient, nil
			}

			//Setup the mocks
			mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil).AnyTimes()
			mockPowerVSClient.EXPECT().CreateInstance(gomock.Any()).Return(stubGetInstances(), nil).AnyTimes()
			mockPowerVSClient.EXPECT().GetInstance(gomock.Any()).Return(stubGetInstance(), nil).AnyTimes()
			mockPowerVSClient.EXPECT().GetImages().Return(stubGetImages(imageNamePrefix, 3), nil).AnyTimes()
			mockPowerVSClient.EXPECT().GetNetworks().Return(stubGetNetworks(networkNamePrefix, 3), nil).AnyTimes()
			mockPowerVSClient.EXPECT().DeleteInstance(gomock.Any()).Return(nil).AnyTimes()

			params := ActuatorParams{
				Client:               k8sClient,
				EventRecorder:        eventRecorder,
				PowerVSClientBuilder: powerVSClientBuilder,
			}
			actuator := NewActuator(params)
			tc.operation(actuator, machine)

			eventList := &corev1.EventList{}
			waitForEvent := func() error {
				err := k8sClient.List(context.Background(), eventList, client.InNamespace(machine.Namespace))
				if err != nil {
					return err
				}

				if len(eventList.Items) != 1 {
					return fmt.Errorf("expected len 1, got %d", len(eventList.Items))
				}
				return nil
			}
			gs.Eventually(waitForEvent, timeout).Should(Succeed())
			gs.Expect(eventList.Items[0].Message).To(Equal(tc.event))
			for i := range eventList.Items {
				gs.Expect(k8sClient.Delete(context.Background(), &eventList.Items[i])).To(Succeed())
			}
		})
	}
}

func TestActuatorExists(t *testing.T) {
	machine, err := stubMachine()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		expectError bool
	}{
		{
			name: "succefuly call reconciler exists",
		},
		{
			name:        "fail to call reconciler exists",
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectError {
				machine.Spec = machinev1beta1.MachineSpec{
					ProviderSpec: machinev1beta1.ProviderSpec{
						Value: &runtime.RawExtension{
							Raw: []byte{'1'},
						},
					},
				}
			}
			mockCtrl := gomock.NewController(t)
			mockPowerVSClient := mock.NewMockClient(mockCtrl)

			powerVSClientBuilder := func(client client.Client, secretName, namespace, cloudInstanceID string,
				debug bool) (powervsClient.Client, error) {
				return mockPowerVSClient, nil
			}

			//Setup the mocks
			mockPowerVSClient.EXPECT().GetInstanceByName(machine.GetName()).Return(stubGetInstance(), nil).AnyTimes()

			params := ActuatorParams{
				Client:               k8sClient,
				EventRecorder:        eventRecorder,
				PowerVSClientBuilder: powerVSClientBuilder,
			}
			actuator := NewActuator(params)

			_, err := actuator.Exists(nil, machine)

			if tc.expectError {
				if err == nil {
					t.Fatal("actuator exists expected to return an error")
				}
			} else {
				if err != nil {
					t.Fatal("actuator exists is not expected to return an error")
				}
			}
		})
	}

}

func TestHandleMachineErrors(t *testing.T) {
	machine, err := stubMachine()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		eventAction string
		event       string
	}{
		{
			name:        "Create event when event action is present",
			eventAction: "testAction",
			event:       "Warning FailedtestAction testError",
		},
		{
			name:        "Don't event when there is no event action",
			eventAction: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eventsChannel := make(chan string, 1)

			params := ActuatorParams{
				// use fake recorder and store an event into one item long buffer for subsequent check
				EventRecorder: &record.FakeRecorder{
					Events: eventsChannel,
				},
			}

			actuator := NewActuator(params)

			actuator.handleMachineError(machine, errors.New("testError"), tc.eventAction)

			select {
			case event := <-eventsChannel:
				if event != tc.event {
					t.Errorf("Expected %q event, got %q", tc.event, event)
				}
			default:
				if tc.event != "" {
					t.Errorf("Expected %q event, got none", tc.event)
				}
			}
		})
	}
}
