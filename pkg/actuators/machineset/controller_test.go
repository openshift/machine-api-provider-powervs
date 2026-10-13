package machineset

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	gtypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MachineSet Reconciler", func() {
	var c client.Client
	var stopMgr context.CancelFunc
	var fakeRecorder *record.FakeRecorder
	var namespace *corev1.Namespace

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: config.Controller{SkipNameValidation: ptr.To(true)}})
		Expect(err).ToNot(HaveOccurred())

		r := Reconciler{
			Client: mgr.GetClient(),
			Log:    log.Log,
		}
		Expect(r.SetupWithManager(mgr, controller.Options{})).To(Succeed())

		fakeRecorder = record.NewFakeRecorder(1)
		r.recorder = fakeRecorder

		c = mgr.GetClient()
		stopMgr = StartTestManager(mgr)

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "mhc-test-"}}
		Expect(c.Create(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(deleteMachineSets(c, namespace.Name)).To(Succeed())
		stopMgr()
	})

	type reconcileTestCase = struct {
		processors          intstr.IntOrString
		memoryGiB           int32
		existingAnnotations map[string]string
		expectedAnnotations map[string]string
		expectedEvents      []string
	}

	DescribeTable("when reconciling MachineSets", func(rtc reconcileTestCase) {
		machineSet, err := newTestMachineSet(namespace.Name, rtc.processors, rtc.memoryGiB, rtc.existingAnnotations)
		Expect(err).ToNot(HaveOccurred())

		Expect(c.Create(ctx, machineSet)).To(Succeed())

		Eventually(func() map[string]string {
			m := &machinev1beta1.MachineSet{}
			key := client.ObjectKey{Namespace: machineSet.Namespace, Name: machineSet.Name}
			err := c.Get(ctx, key, m)
			if err != nil {
				return nil
			}
			annotations := m.GetAnnotations()
			if annotations != nil {
				return annotations
			}
			// Return an empty map to distinguish between empty annotations and errors
			return make(map[string]string)
		}, timeout).Should(Equal(rtc.expectedAnnotations))

		// Check which event types were sent
		Eventually(fakeRecorder.Events, timeout).Should(HaveLen(len(rtc.expectedEvents)))
		receivedEvents := []string{}
		eventMatchers := []gtypes.GomegaMatcher{}
		for _, ev := range rtc.expectedEvents {
			receivedEvents = append(receivedEvents, <-fakeRecorder.Events)
			eventMatchers = append(eventMatchers, ContainSubstring(fmt.Sprintf("%s", ev)))
		}
		Expect(receivedEvents).To(ConsistOf(eventMatchers))
	},
		Entry("with 0.5cpu 32GiB memory", reconcileTestCase{
			processors:          intstr.FromString("0.5"),
			memoryGiB:           32,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "32768",
			},
			expectedEvents: []string{},
		}),
		Entry("with 1cpu 64GiB", reconcileTestCase{
			processors:          intstr.FromString("1"),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "65536",
			},
			expectedEvents: []string{},
		}),
		Entry("with 2.5cpu 64GiB", reconcileTestCase{
			processors:          intstr.FromString("2.5"),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "24",
				memoryKey: "65536",
			},
			expectedEvents: []string{},
		}),
		Entry("with invalid cpu value", reconcileTestCase{
			processors:          intstr.FromString("invalid_cpu"),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{},
			expectedEvents:      []string{"Warning ReconcileError failed to get cpu value: strconv.ParseFloat: parsing \"invalid_cpu\": invalid syntax"},
		}),
	)
})

func deleteMachineSets(c client.Client, namespaceName string) error {
	machineSets := &machinev1beta1.MachineSetList{}
	err := c.List(ctx, machineSets, client.InNamespace(namespaceName))
	if err != nil {
		return err
	}

	for _, ms := range machineSets.Items {
		err := c.Delete(ctx, &ms)
		if err != nil {
			return err
		}
	}

	Eventually(func() error {
		machineSets := &machinev1beta1.MachineSetList{}
		err := c.List(ctx, machineSets)
		if err != nil {
			return err
		}
		if len(machineSets.Items) > 0 {
			return fmt.Errorf("machineSets not deleted")
		}
		return nil
	}, timeout).Should(Succeed())

	return nil
}

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name                string
		processors          intstr.IntOrString
		memoryGiB           int32
		existingAnnotations map[string]string
		expectedAnnotations map[string]string
		expectErr           bool
	}{
		{
			name:                "with 0.5cpu 32GiB memory",
			processors:          intstr.FromString("0.5"),
			memoryGiB:           32,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "32768",
			},
			expectErr: false,
		},
		{
			name:                "with 1cpu 64GiB memory",
			processors:          intstr.FromInt(1),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "65536",
			},
			expectErr: false,
		},
		{
			name:                "with 1.5cpu 64GiB memory",
			processors:          intstr.FromString("1.5"),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "16",
				memoryKey: "65536",
			},
			expectErr: false,
		},
		{
			name:                "with cpu value as string",
			processors:          intstr.FromString("invalid_cpu"),
			memoryGiB:           64,
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{},
			expectErr:           true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)

			machineSet, err := newTestMachineSet("default", tc.processors, tc.memoryGiB, tc.existingAnnotations)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = reconcile(machineSet)
			g.Expect(err != nil).To(Equal(tc.expectErr))
			g.Expect(machineSet.Annotations).To(Equal(tc.expectedAnnotations))
		})
	}
}

func newTestMachineSet(namespace string, processors intstr.IntOrString, memoryGiB int32, existingAnnotations map[string]string) (*machinev1beta1.MachineSet, error) {

	// Copy annotations map so we don't modify the input
	annotations := make(map[string]string)
	for k, v := range existingAnnotations {
		annotations[k] = v
	}

	machineProviderSpec := &machinev1.PowerVSMachineProviderConfig{
		Processors: processors,
		MemoryGiB:  memoryGiB,
	}
	providerSpec, err := providerSpecFromMachine(machineProviderSpec)
	if err != nil {
		return nil, err
	}

	replicas := int32(1)
	return &machinev1beta1.MachineSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:  annotations,
			GenerateName: "test-machineset-",
			Namespace:    namespace,
		},
		Spec: machinev1beta1.MachineSetSpec{
			Replicas: &replicas,
			Template: machinev1beta1.MachineTemplateSpec{
				Spec: machinev1beta1.MachineSpec{
					ProviderSpec: providerSpec,
				},
			},
		},
	}, nil
}

func providerSpecFromMachine(in *machinev1.PowerVSMachineProviderConfig) (machinev1beta1.ProviderSpec, error) {
	bytes, err := json.Marshal(in)
	if err != nil {
		return machinev1beta1.ProviderSpec{}, err
	}
	return machinev1beta1.ProviderSpec{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func TestGetPowerVSProcessorValue(t *testing.T) {
	testCases := []struct {
		name        string
		processors  intstr.IntOrString
		expectedCPU string
		expectErr   bool
	}{
		{
			name:        "with cpu in string and fractional",
			processors:  intstr.FromString("0.5"),
			expectedCPU: "8",
		},
		{
			name:        "with cpu in string and fractional value greater than 1",
			processors:  intstr.FromString("1.25"),
			expectedCPU: "16",
		},
		{
			name:        "with cpu in string and whole number",
			processors:  intstr.FromString("2"),
			expectedCPU: "16",
		},
		{
			name:        "with cpu in int format",
			processors:  intstr.FromInt(1),
			expectedCPU: "8",
		},
		{
			name:       "with invalid cpu",
			processors: intstr.FromString("invalid_cpu"),
			expectErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)
			cpu, err := getPowerVSProcessorValue(tc.processors)
			if tc.expectErr {
				if err == nil {
					t.Fatal("getPowerVSProcessorValue expected to return an error")
				}
			} else {
				if err != nil {
					t.Fatalf("getPowerVSProcessorValue is not expected to return an error, error: %v", err)
				}
				g.Expect(cpu).To(Equal(tc.expectedCPU))
			}
		})
	}
}
