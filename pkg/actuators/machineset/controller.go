package machineset

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
	machineactuator "github.com/openshift/machine-api-provider-powervs/pkg/actuators/machine"
)

const (
	// These exposes compute information based on the providerSpec input.
	// This is needed by the autoscaler to foresee upcoming capacity when scaling from zero.
	// https://github.com/openshift/enhancements/pull/186
	cpuKey    = "machine.openshift.io/vCPU"
	memoryKey = "machine.openshift.io/memoryMb"
	// defaultSMT is the default value of simultaneous multithreading
	defaultSMT = 8
)

// Reconciler reconciles machineSets.
type Reconciler struct {
	Client client.Client
	Log    logr.Logger

	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// SetupWithManager creates a new controller for a manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&machinev1beta1.MachineSet{}).
		WithOptions(options).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("machineset-controller")
	r.scheme = mgr.GetScheme()
	return nil
}

// Reconcile implements controller runtime Reconciler interface.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("machineset", req.Name, "namespace", req.Namespace)
	logger.V(3).Info("Reconciling")

	machineSet := &machinev1beta1.MachineSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, machineSet); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Ignore deleted MachineSets, this can happen when foregroundDeletion
	// is enabled
	if !machineSet.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	originalMachineSetToPatch := client.MergeFrom(machineSet.DeepCopy())

	result, err := reconcile(machineSet)
	if err != nil {
		logger.Error(err, "Failed to reconcile MachineSet")
		r.recorder.Eventf(machineSet, corev1.EventTypeWarning, "ReconcileError", "%v", err)
		// we don't return here, so we want to attempt to patch the machine regardless of an error.
	}

	if err := r.Client.Patch(ctx, machineSet, originalMachineSetToPatch); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch machineSet: %w", err)
	}

	if isInvalidConfigurationError(err) {
		// For situations where requeuing won't help we don't return error.
		// https://github.com/kubernetes-sigs/controller-runtime/issues/617
		return result, nil
	}
	return result, err
}

func isInvalidConfigurationError(err error) bool {
	switch t := err.(type) {
	case *mapierrors.MachineError:
		if t.Reason == machinev1beta1.InvalidConfigurationMachineError {
			return true
		}
	}
	return false
}

func reconcile(machineSet *machinev1beta1.MachineSet) (ctrl.Result, error) {
	providerConfig, err := machineactuator.ProviderSpecFromRawExtension(machineSet.Spec.Template.Spec.ProviderSpec.Value)
	if err != nil {
		return ctrl.Result{}, mapierrors.InvalidMachineConfiguration("failed to get providerConfig: %v", err)
	}

	if machineSet.Annotations == nil {
		machineSet.Annotations = make(map[string]string)
	}
	cpu, err := getPowerVSProcessorValue(providerConfig.Processors)
	if err != nil {
		return ctrl.Result{}, mapierrors.InvalidMachineConfiguration("failed to get cpu value: %v", err)
	}
	machineSet.Annotations[cpuKey] = cpu
	// Convert Memory from GiB to Mb
	machineSet.Annotations[memoryKey] = strconv.FormatInt(int64(providerConfig.MemoryGiB)*1024, 10)
	return ctrl.Result{}, nil
}

func getPowerVSProcessorValue(processor intstr.IntOrString) (string, error) {
	var cores float64
	var virtualProcessors string
	var err error
	switch processor.Type {
	case intstr.Int:
		cores = float64(processor.IntVal)
	case intstr.String:
		if cores, err = strconv.ParseFloat(processor.StrVal, 64); err != nil {
			return virtualProcessors, err
		}
	}
	// There is a core-to-lCPU ratio of 1:1 for Dedicated processors. For shared processors, fractional cores round up to the nearest whole number. For example, 1.25 cores equals 2 lCPUs.
	// VM with 1 dedicated processor will see = 1 * SMT = 1 * 8 = 8 cpus in OS
	// VM with 1.5 shared processor will see = 2 * SMT = 2 * 8 = 16 cpus in OS
	// Here SMT: simultaneous multithreading which is default to 8
	// Here lCPU: number of online logical processors
	// example: on a Power VS machine with 0.5 cores
	// $ lparstat
	//	  System Configuration
	//	  type=Shared mode=Uncapped smt=8 lcpu=1 mem=33413760 kB cpus=20 ent=0.50
	virtualProcessors = fmt.Sprintf("%v", math.Ceil(cores)*defaultSMT)
	return virtualProcessors, nil
}
