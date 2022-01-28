package nodeupdate

import (
	"context"
	"fmt"
	"sync"
	"time"

	bluemixmodels "github.com/IBM-Cloud/bluemix-go/models"
	"github.com/IBM-Cloud/power-go-client/power/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	powervsclient "github.com/openshift/cluster-api-provider-powervs/pkg/client"
	"github.com/openshift/cluster-api-provider-powervs/pkg/options"
)

const (
	requeueDurationWhenVMNotReady = 1 * time.Minute
	requeueDurationWhenVMNotFound = 2 * time.Minute
	//TODO: should this value be flag driven
	concurrencyLimit = 5
)

var (
	_ reconcile.Reconciler = &providerIDReconciler{}
)

type providerIDReconciler struct {
	client               client.Client
	PowerVSClientBuilder powervsclient.PowerVSClientBuilderFuncType
}

// Reconcile make sure a node has a ProviderID set. The providerID is the ID
// of the machine on powervs. The ID is the PvmInstanceID
// as its guarantee to be unique in a cluster.
func (r *providerIDReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("%s: Reconciling node", request.NamespacedName)

	// Fetch the Node instance
	node := corev1.Node{}
	err := r.client.Get(context.Background(), request.NamespacedName, &node)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			klog.Infof("%s: Node not found - do nothing", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, fmt.Errorf("error getting node: %v", err)
	}

	if node.Spec.ProviderID != "" {
		return reconcile.Result{}, nil
	}

	klog.Infof("%s: ProviderID is not updated in the node - update it", node.Name)

	apiKey, err := powervsclient.GetAPIKey(r.client, powervsclient.DefaultCredentialSecret, powervsclient.DefaultCredentialNamespace)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to read the API key from the secret: %v", err)
	}

	c, err := powervsclient.NewClientMinimal(r.client, apiKey)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("%s: failed to create NewClientMinimal, with error: %v", node.Name, err)
	}

	serviceInstances, err := c.GetCloudServiceInstances()
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("%s: failed to get the cloud instances, with error: %v", node.Name, err)
	}

	var instance *models.PVMInstanceReference
	//TODO: Consider initializing at package level, if needed
	concurrencyController := make(chan struct{}, concurrencyLimit)
	resultChan := make(chan serviceInstanceResult, len(serviceInstances))
	var errMsg error
	var producerWg, receiverWg sync.WaitGroup
	receiverWg.Add(1)
	for index, i := range serviceInstances {
		producerWg.Add(1)
		//Setting the concurrency controller
		concurrencyController <- struct{}{}

		//Get service instances
		go func(serviceInstance bluemixmodels.ServiceInstanceV2) {
			defer producerWg.Done()
			r.getInstances(serviceInstance, node.Name, resultChan, concurrencyController, options.GetDebugMode())
		}(i)

		//Call only once in a loop and read the result channel to process the output
		if index == 0 {
			go func() {
				defer receiverWg.Done()
				for i := 1; i <= len(serviceInstances); i++ {
					res := <-resultChan
					if res.err != nil {
						klog.Error(res.err)
						errMsg = res.err
						return
					} else if res.instance != nil {
						instance = res.instance
						return
					}
				}
			}()
		}
		if instance != nil {
			break
		} else if errMsg != nil {
			return reconcile.Result{}, errMsg
		}
	}
	//Wait for the completion of called getInstance method
	producerWg.Wait()
	//Wait for the completion of reading results
	receiverWg.Wait()
	if instance != nil {
		node.Spec.ProviderID = powervsclient.FormatProviderID(*instance.PvmInstanceID)
	} else {
		// TODO: enable this block later
		//klog.Infof("%s: Virtual Machine of this node doesn't exists - delete the node", node.Name)
		//if err := r.client.Delete(context.Background(), &node); err != nil {
		//	return reconcile.Result{}, fmt.Errorf("%s: Error deleting Node, with error: %v", node.Name, err)
		//}
		//return reconcile.Result{}, nil
		klog.Infof("%s: Virtual Machine of this node doesn't exists, requeuing after 2 mins", node.Name)
		return reconcile.Result{Requeue: true, RequeueAfter: requeueDurationWhenVMNotFound}, nil
	}

	if *instance.Status != powervsclient.InstanceStateNameActive {
		klog.Infof("%s: Virtual Machine of this node isn't ready - requeue for 1 minute", node.Name)
		return reconcile.Result{Requeue: true, RequeueAfter: requeueDurationWhenVMNotReady}, nil
	}

	if err = r.client.Update(context.Background(), &node); err != nil {
		return reconcile.Result{}, fmt.Errorf("%s: failed updating node, with error: %v", node.Name, err)
	}

	return reconcile.Result{}, nil
}

type serviceInstanceResult struct {
	err      error
	instance *models.PVMInstanceReference
}

func (r *providerIDReconciler) getInstances(serviceInstance bluemixmodels.ServiceInstanceV2, nodeName string,
	resultChan chan serviceInstanceResult, concurrencyController chan struct{}, debug bool) {
	defer func() {
		<-concurrencyController
	}()
	var result serviceInstanceResult
	var instance *models.PVMInstanceReference
	cl, err := powervsclient.NewValidatedClient(r.client, powervsclient.DefaultCredentialSecret,
		powervsclient.DefaultCredentialNamespace, serviceInstance.Guid, debug)
	if err != nil {
		result = serviceInstanceResult{
			err: fmt.Errorf("%s: failed to create NewValidatedClient, with error: %v", nodeName, err),
		}
		resultChan <- result
		return
	}

	ins, err := cl.GetInstances()
	if ins == nil {
		result = serviceInstanceResult{
			err: fmt.Errorf("%s: failed to GetInstances for %s, with error: %v", nodeName, serviceInstance.Name, err),
		}
		resultChan <- result
		return
	}
	for _, in := range ins.PvmInstances {
		if *in.ServerName == nodeName {
			instance = in
			break
		}
	}
	result = serviceInstanceResult{
		instance: instance,
	}
	resultChan <- result
}

// Add registers a new provider ID reconciler controller with the controller manager
func Add(mgr manager.Manager) error {
	reconciler, err := newProviderIDReconciler(mgr)

	if err != nil {
		return fmt.Errorf("error building reconciler: %v", err)
	}

	c, err := controller.New("providerID-controller", mgr, controller.Options{Reconciler: reconciler})
	if err != nil {
		return err
	}

	//Watch node changes
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// newProviderIDReconcilerv creates a new providerID reconciler
func newProviderIDReconciler(mgr manager.Manager) (*providerIDReconciler, error) {
	r := providerIDReconciler{
		client: mgr.GetClient(),
	}
	return &r, nil
}
