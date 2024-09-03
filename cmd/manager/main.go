/*
Copyright 2021 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	apifeatures "github.com/openshift/api/features"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	"github.com/openshift/library-go/pkg/features"
	"github.com/openshift/machine-api-operator/pkg/controller/machine"
	"github.com/openshift/machine-api-operator/pkg/metrics"
	machineactuator "github.com/openshift/machine-api-provider-powervs/pkg/actuators/machine"
	machinesetcontroller "github.com/openshift/machine-api-provider-powervs/pkg/actuators/machineset"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
	"github.com/openshift/machine-api-provider-powervs/pkg/controllers"
	"github.com/openshift/machine-api-provider-powervs/pkg/options"
	"github.com/openshift/machine-api-provider-powervs/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/util/feature"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/cache"
	"k8s.io/component-base/featuregate"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// The default durations for the leader electrion operations.
var (
	leaseDuration = 120 * time.Second
	renewDealine  = 110 * time.Second
	retryPeriod   = 20 * time.Second
	syncPeriod    = 10 * time.Minute
)

func main() {
	printVersion := flag.Bool(
		"version",
		false,
		"print version and exit",
	)

	metricsAddress := flag.String(
		"metrics-bind-address",
		metrics.DefaultMachineMetricsAddress,
		"Address for hosting metrics",
	)

	watchNamespace := flag.String(
		"namespace",
		"",
		"Namespace that the controller watches to reconcile machine-api objects. If unspecified, the controller watches for machine-api objects across all namespaces.",
	)

	leaderElectResourceNamespace := flag.String(
		"leader-elect-resource-namespace",
		"",
		"The namespace of resource object that is used for locking during leader election. If unspecified and running in cluster, defaults to the service account namespace for the controller. Required for leader-election outside of a cluster.",
	)

	leaderElect := flag.Bool(
		"leader-elect",
		false,
		"Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.",
	)

	leaderElectLeaseDuration := flag.Duration(
		"leader-elect-lease-duration",
		leaseDuration,
		"The duration that non-leader candidates will wait after observing a leadership renewal until attempting to acquire leadership of a led but unrenewed leader slot. This is effectively the maximum duration that a leader can be stopped before it is replaced by another candidate. This is only applicable if leader election is enabled.",
	)

	healthAddr := flag.String(
		"health-addr",
		":9440",
		"The address for health checking.",
	)

	options.Debug = flag.Bool(
		"debug",
		false,
		"WARNING!!! Prints debug information like API calls/responses to the IBM Cloud, should be enabled only in development mode, may expose the sensitive data like tokens",
	)

	if *options.Debug {
		fmt.Println("WARNING!!! debug has been enabled and it should be used only for development")
	}

	// Sets up feature gates
	defaultMutableGate := feature.DefaultMutableFeatureGate
	gateOpts, err := features.NewFeatureGateOptions(defaultMutableGate, apifeatures.SelfManaged, apifeatures.FeatureGateMachineAPIMigration)
	if err != nil {
		klog.Fatalf("Error setting up feature gates: %v", err)
	}

	// Add the --feature-gates flag
	gateOpts.AddFlagsToGoFlagSet(nil)

	klog.InitFlags(nil)
	flag.Set("logtostderr", "true")
	flag.Parse()

	if *printVersion {
		fmt.Println(version.String)
		os.Exit(0)
	}

	var watchNamespaces map[string]ctrlcache.Config
	if *watchNamespace != "" {
		watchNamespaces = map[string]ctrlcache.Config{
			*watchNamespace: {},
		}
	}

	ctrlOptions := ctrl.Options{
		Cache: ctrlcache.Options{
			SyncPeriod:        &syncPeriod,
			DefaultNamespaces: watchNamespaces,
		},
		LeaderElection:          *leaderElect,
		LeaderElectionNamespace: *leaderElectResourceNamespace,
		LeaderElectionID:        "machine-api-provider-powervs-leader",
		LeaseDuration:           leaderElectLeaseDuration,
		RenewDeadline:           &renewDealine,
		RetryPeriod:             &retryPeriod,
		Metrics: server.Options{
			BindAddress: *metricsAddress,
		},
		HealthProbeBindAddress: *healthAddr,
	}

	restConfig := ctrl.GetConfigOrDie()

	// Sets feature gates from flags
	klog.Infof("Initializing feature gates: %s", strings.Join(defaultMutableGate.KnownFeatures(), ", "))
	warnings, err := gateOpts.ApplyTo(defaultMutableGate)
	if err != nil {
		klog.Fatalf("Error setting feature gates from flags: %v", err)
	}
	if len(warnings) > 0 {
		klog.Infof("Warnings setting feature gates from flags: %v", warnings)
	}

	klog.Infof("FeatureGateMachineAPIMigration initialised: %t", defaultMutableGate.Enabled(featuregate.Feature(apifeatures.FeatureGateMachineAPIMigration)))

	mgr, err := ctrl.NewManager(restConfig, ctrlOptions)
	if err != nil {
		klog.Fatalf("Error creating manager: %v", err)
	}

	// Setup Scheme for all resources
	if err := machinev1beta1.Install(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error setting up scheme: %v", err)
	}

	if err := configv1.Install(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}

	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatal(err)
	}
	// Create the cachestore to hold the DHCP IP
	cacheStore := cache.NewTTLStore(machineactuator.CacheKeyFunc, machineactuator.CacheTTL)
	// Initialize machine actuator.
	machineActuator := machineactuator.NewActuator(machineactuator.ActuatorParams{
		Client:               mgr.GetClient(),
		EventRecorder:        mgr.GetEventRecorderFor("powervscontroller"),
		PowerVSClientBuilder: powervsclient.NewValidatedClient,
		PowerVSMinimalClient: powervsclient.NewMinimalPowerVSClient,
		DHCPIPCacheStore:     cacheStore,
	})

	if err := machine.AddWithActuator(mgr, machineActuator, defaultMutableGate); err != nil {
		klog.Fatalf("Error adding actuator: %v", err)
	}

	ctrl.SetLogger(klogr.New())
	setupLog := ctrl.Log.WithName("setup")
	if err = (&machinesetcontroller.Reconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("MachineSet"),
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MachineSet")
		os.Exit(1)
	}

	if err := (&controllers.Reconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("CustomServiceEndpoint"),
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "CustomServiceEndpoint")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		klog.Fatal(err)
	}

	// Start the Cmd
	err = mgr.Start(ctrl.SetupSignalHandler())
	if err != nil {
		klog.Fatalf("Error starting manager: %v", err)
	}
}
