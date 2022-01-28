package machine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/rand"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/cluster-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	powervsClient "github.com/openshift/cluster-api-provider-powervs/pkg/client"

	. "github.com/onsi/gomega"
)

func machineWithSpec(spec *v1alpha1.PowerVSMachineProviderConfig) *machinev1.Machine {
	rawSpec, err := v1alpha1.RawExtensionFromProviderSpec(spec)
	if err != nil {
		panic("Failed to encode raw extension from provider spec")
	}

	return &machinev1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "powerVS-test",
			Namespace: defaultNamespace,
		},
		Spec: machinev1.MachineSpec{
			ProviderSpec: machinev1.ProviderSpec{
				Value: rawSpec,
			},
		},
	}
}

func TestNewMachineScope(t *testing.T) {
	g := NewWithT(t)

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	userDataSecret := stubUserDataSecret(userSecretName)
	credentialsSecret := stubPowerVSCredentialsSecret(credSecretName)

	fakeClient := fake.NewFakeClient(userDataSecret, credentialsSecret)
	credSecretName = fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	validProviderSpec, err := v1alpha1.RawExtensionFromProviderSpec(stubProviderConfig(credSecretName))
	g.Expect(err).ToNot(HaveOccurred())

	cases := []struct {
		name          string
		params        machineScopeParams
		expectedError error
	}{
		{
			name: "successfully create machine scope",
			params: machineScopeParams{
				client: fakeClient,
				powerVSClientBuilder: func(client client.Client, secretName, namespace,
					cloudInstanceID string, debug bool) (powervsClient.Client, error) {
					return nil, nil
				},
				machine: &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: defaultNamespace,
						Labels: map[string]string{
							machinev1.MachineClusterIDLabel: "CLUSTERID",
						},
					},
					Spec: machinev1.MachineSpec{
						ProviderSpec: machinev1.ProviderSpec{
							Value: validProviderSpec,
						},
					}},
			},
		},
		{
			name: "fail to get provider spec",
			params: machineScopeParams{
				client: fakeClient,
				powerVSClientBuilder: func(client client.Client, secretName, namespace,
					cloudInstanceID string, debug bool) (powervsClient.Client, error) {
					return nil, nil
				},
				machine: &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: defaultNamespace,
						Labels: map[string]string{
							machinev1.MachineClusterIDLabel: "CLUSTERID",
						},
					},
					Spec: machinev1.MachineSpec{
						ProviderSpec: machinev1.ProviderSpec{
							Value: &runtime.RawExtension{
								Raw: []byte{'1'},
							},
						},
					}},
			},
			expectedError: errors.New("failed to get machine config: error unmarshalling providerSpec: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go value of type v1alpha1.PowerVSMachineProviderConfig"),
		},
		{
			name: "fail to get provider status",
			params: machineScopeParams{
				client: fakeClient,
				powerVSClientBuilder: func(client client.Client, secretName, namespace,
					cloudInstanceID string, debug bool) (powervsClient.Client, error) {
					return nil, nil
				},
				machine: &machinev1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: defaultNamespace,
						Labels: map[string]string{
							machinev1.MachineClusterIDLabel: "CLUSTERID",
						},
					},
					Spec: machinev1.MachineSpec{
						ProviderSpec: machinev1.ProviderSpec{
							Value: validProviderSpec,
						},
					},
					Status: machinev1.MachineStatus{
						ProviderStatus: &runtime.RawExtension{
							Raw: []byte{'1'},
						},
					},
				},
			},
			expectedError: errors.New("failed to get machine provider status: error unmarshalling providerStatus: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal number into Go value of type v1alpha1.PowerVSMachineProviderStatus"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewWithT(t)
			_, err := newMachineScope(tc.params)
			if tc.expectedError != nil {
				gs.Expect(err).To(HaveOccurred())
				gs.Expect(err.Error()).To(Equal(tc.expectedError.Error()))
			} else {
				gs.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestGetUserData(t *testing.T) {
	userDataSecretName := "PowerVS-ignition"

	defaultProviderSpec := &v1alpha1.PowerVSMachineProviderConfig{
		UserDataSecret: &corev1.LocalObjectReference{
			Name: userDataSecretName,
		},
	}

	testCases := []struct {
		testCase         string
		userDataSecret   *corev1.Secret
		providerSpec     *v1alpha1.PowerVSMachineProviderConfig
		expectedUserdata []byte
		expectError      bool
	}{
		{
			testCase: "all good",
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userDataSecretName,
					Namespace: defaultNamespace,
				},
				Data: map[string][]byte{
					userDataSecretKey: []byte("{}"),
				},
			},
			providerSpec:     defaultProviderSpec,
			expectedUserdata: []byte("{}"),
			expectError:      false,
		},
		{
			testCase:       "missing secret",
			userDataSecret: nil,
			providerSpec:   defaultProviderSpec,
			expectError:    true,
		},
		{
			testCase: "missing key in secret",
			userDataSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      userDataSecretName,
					Namespace: defaultNamespace,
				},
				Data: map[string][]byte{
					"badKey": []byte("{}"),
				},
			},
			providerSpec: defaultProviderSpec,
			expectError:  true,
		},
		{
			testCase:         "no provider spec",
			userDataSecret:   nil,
			providerSpec:     nil,
			expectError:      false,
			expectedUserdata: nil,
		},
		{
			testCase:         "no user-data in provider spec",
			userDataSecret:   nil,
			providerSpec:     &v1alpha1.PowerVSMachineProviderConfig{},
			expectError:      false,
			expectedUserdata: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testCase, func(t *testing.T) {
			clientObjs := []runtime.Object{}

			if tc.userDataSecret != nil {
				clientObjs = append(clientObjs, tc.userDataSecret)
			}

			client := fake.NewFakeClient(clientObjs...)

			// Can't use newMachineScope because it tries to create an API
			// session, and other things unrelated to these tests.
			ms := &machineScope{
				Context:      context.Background(),
				client:       client,
				machine:      machineWithSpec(tc.providerSpec),
				providerSpec: tc.providerSpec,
			}

			userData, err := ms.getUserData()
			if !tc.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !bytes.Equal(userData, tc.expectedUserdata) {
				t.Errorf("Got: %q, Want: %q", userData, tc.expectedUserdata)
			}
		})
	}
}

func TestPatchMachine(t *testing.T) {
	g := NewWithT(t)

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	powerVSCredentialsSecret := stubPowerVSCredentialsSecret(credSecretName)
	g.Expect(k8sClient.Create(context.TODO(), powerVSCredentialsSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.TODO(), powerVSCredentialsSecret)).To(Succeed())
	}()

	userSecretName := fmt.Sprintf("%s-%s", userDataSecretName, rand.String(nameLength))
	userDataSecret := stubUserDataSecret(userSecretName)
	g.Expect(k8sClient.Create(context.TODO(), userDataSecret)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(context.TODO(), userDataSecret)).To(Succeed())
	}()

	failedPhase := "Failed"

	providerStatus := &v1alpha1.PowerVSMachineProviderStatus{}

	testCases := []struct {
		name   string
		mutate func(*machinev1.Machine)
		expect func(*machinev1.Machine) error
	}{
		//TODO: Investigate why test fails with inclusion of race flag
		//{
		//	name: "Test changing labels",
		//	mutate: func(m *machinev1.Machine) {
		//		m.ObjectMeta.Labels["testlabel"] = "test"
		//	},
		//	expect: func(m *machinev1.Machine) error {
		//		if m.ObjectMeta.Labels["testlabel"] != "test" {
		//			return fmt.Errorf("label \"testlabel\" %q not equal expected \"test\"", m.ObjectMeta.Labels["test"])
		//		}
		//		return nil
		//	},
		//},
		{
			name: "Test setting phase",
			mutate: func(m *machinev1.Machine) {
				m.Status.Phase = &failedPhase
			},
			expect: func(m *machinev1.Machine) error {
				if m.Status.Phase != nil && *m.Status.Phase == failedPhase {
					return nil
				}
				return fmt.Errorf("phase is nil or not equal expected \"Failed\"")
			},
		},
		{
			name: "Test setting provider status",
			mutate: func(m *machinev1.Machine) {
				instanceID := "123"
				instanceState := "running"
				providerStatus.InstanceID = &instanceID
				providerStatus.InstanceState = &instanceState
			},
			expect: func(m *machinev1.Machine) error {
				providerStatus, err := v1alpha1.ProviderStatusFromRawExtension(m.Status.ProviderStatus)
				if err != nil {
					return fmt.Errorf("unable to get provider status: %v", err)
				}

				if providerStatus.InstanceID == nil || *providerStatus.InstanceID != "123" {
					return fmt.Errorf("instanceID is nil or not equal expected \"123\"")
				}

				if providerStatus.InstanceState == nil || *providerStatus.InstanceState != "running" {
					return fmt.Errorf("instanceState is nil or not equal expected \"running\"")
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gs := NewWithT(t)

			machine, err := stubMachine()
			gs.Expect(err).ToNot(HaveOccurred())
			gs.Expect(machine).ToNot(BeNil())

			ctx := context.TODO()

			// Create the machine
			gs.Expect(k8sClient.Create(ctx, machine)).To(Succeed())
			defer func() {
				gs.Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
			}()

			// Ensure the machine has synced to the cache
			getMachine := func() error {
				machineKey := types.NamespacedName{Namespace: machine.Namespace, Name: machine.Name}
				return k8sClient.Get(ctx, machineKey, machine)
			}
			gs.Eventually(getMachine, timeout).Should(Succeed())

			machineScope, err := newMachineScope(machineScopeParams{
				client:  k8sClient,
				machine: machine,
				powerVSClientBuilder: func(client client.Client, secretName, namespace, cloudInstanceID string,
					debug bool) (powervsClient.Client, error) {
					return nil, nil
				},
			})

			if err != nil {
				t.Fatal(err)
			}

			tc.mutate(machineScope.machine)

			machineScope.providerStatus = providerStatus

			// Patch the machine and check the expectation from the test case
			gs.Expect(machineScope.patchMachine()).To(Succeed())
			checkExpectation := func() error {
				if err := getMachine(); err != nil {
					return err
				}
				return tc.expect(machine)
			}
			gs.Eventually(checkExpectation, timeout).Should(Succeed())

			// Check that resource version doesn't change if we call patchMachine() again
			machineResourceVersion := machine.ResourceVersion

			gs.Expect(machineScope.patchMachine()).To(Succeed())
			gs.Eventually(getMachine, timeout).Should(Succeed())
			gs.Expect(machine.ResourceVersion).To(Equal(machineResourceVersion))
		})
	}
}
