package controllers

import (
	"context"
	"fmt"
	"sort"

	"net/url"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	configv1 "github.com/openshift/api/config/v1"
)

const infrastructureResourceName = "cluster"

// Reconciler reconciles infrastructure object.
type Reconciler struct {
	Client client.Client
	Log    logr.Logger

	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// SetupWithManager creates a new controller for a manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	_, err := ctrl.NewControllerManagedBy(mgr).
		For(&configv1.Infrastructure{}).
		WithOptions(options).
		Build(r)

	if err != nil {
		return fmt.Errorf("failed setting up with a controller manager: %w", err)
	}
	r.recorder = mgr.GetEventRecorderFor("custom-service-endpoint-controller")
	r.scheme = mgr.GetScheme()
	return nil
}

// Reconcile implements controller runtime Reconciler interface.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("CustomServiceEndpoint", req.Name, "namespace", req.Namespace)
	logger.V(3).Info("CustomServiceEndpoint Reconciling")

	infra := &configv1.Infrastructure{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: infrastructureResourceName}, infra); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("infrastructure resource not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	currentInfra := infra.DeepCopy()
	var services []configv1.PowerVSServiceEndpoint
	if currentInfra.Spec.PlatformSpec.PowerVS != nil {
		services = append(services, currentInfra.Spec.PlatformSpec.PowerVS.ServiceEndpoints...)
	}

	if len(services) == 0 {
		logger.Info("no custom service endpoints are specified")
		return ctrl.Result{}, nil
	}

	logger.Info("custom service endpoints in platformSpec", "endpoints", services)
	if err := validateServiceEndpoints(services); err != nil {
		r.recorder.Eventf(currentInfra, corev1.EventTypeWarning, "Invalid spec.platformSpec.powervs.serviceEndpoints provided for infrastructures.cluster", "%v", err)
		return ctrl.Result{}, err
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	var existingServices []configv1.PowerVSServiceEndpoint
	if currentInfra.Status.PlatformStatus != nil && currentInfra.Status.PlatformStatus.PowerVS != nil {
		existingServices = append(existingServices, currentInfra.Status.PlatformStatus.PowerVS.ServiceEndpoints...)
	}
	logger.Info("existing custom service endpoints in platformStatus", "endpoints", existingServices)
	if equality.Semantic.DeepEqual(existingServices, services) {
		return ctrl.Result{}, nil // nothing to do now
	}

	if currentInfra.Status.PlatformStatus == nil {
		currentInfra.Status.PlatformStatus = &configv1.PlatformStatus{}
	}
	if currentInfra.Status.PlatformStatus.PowerVS == nil {
		currentInfra.Status.PlatformStatus.PowerVS = &configv1.PowerVSPlatformStatus{}
	}

	currentInfra.Status.PlatformStatus.PowerVS.ServiceEndpoints = services
	logger.Info("updating the platformStatus", "services", services)
	err := r.Client.Status().Update(ctx, currentInfra)
	return ctrl.Result{}, err
}

func validateServiceEndpoints(endpoints []configv1.PowerVSServiceEndpoint) error {
	fldPath := field.NewPath("spec", "platformSpec", "powervs", "serviceEndpoints")

	allErrs := field.ErrorList{}
	tracker := map[string]int{}
	for idx, e := range endpoints {
		fldp := fldPath.Index(idx)
		if eidx, ok := tracker[e.Name]; ok {
			allErrs = append(allErrs, field.Invalid(fldp.Child("name"), e.Name, fmt.Sprintf("duplicate service endpoint not allowed for %s, service endpoint already defined at %s", e.Name, fldPath.Index(eidx))))
		} else {
			tracker[e.Name] = idx
		}

		if err := validateServiceURL(e.URL); err != nil {
			allErrs = append(allErrs, field.Invalid(fldp.Child("url"), e.URL, err.Error()))
		}
	}
	return allErrs.ToAggregate()
}

func validateServiceURL(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	if u.Hostname() == "" {
		return fmt.Errorf("host cannot be empty, empty host provided")
	}
	if s := u.Scheme; s != "https" {
		return fmt.Errorf("invalid scheme %s, only https allowed", s)
	}
	if r := u.RequestURI(); r != "/" {
		return fmt.Errorf("no path or request parameters must be provided, %q was provided", r)
	}

	return nil
}
