package machine

import (
	"fmt"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/openshift/cluster-api-provider-powervs/pkg/client"

	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	"github.com/openshift/machine-api-operator/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const (
	requeueAfterSeconds      = 20
	requeueAfterFatalSeconds = 180
	masterLabel              = "node-role.kubernetes.io/master"
)

// Reconciler runs the logic to reconciles a machine resource towards its desired state
type Reconciler struct {
	*machineScope
}

func newReconciler(scope *machineScope) *Reconciler {
	return &Reconciler{
		machineScope: scope,
	}
}

// create creates machine if it does not exists.
func (r *Reconciler) create() error {
	klog.Infof("%s: creating machine", r.machine.Name)

	if err := validateMachine(*r.machine); err != nil {
		return fmt.Errorf("%v: failed validating machine provider spec: %w", r.machine.GetName(), err)
	}

	// TODO: remove 45 - 59, this logic is not needed anymore
	// We explicitly do NOT want to remove stopped masters.
	isMaster, err := r.isMaster()
	if err != nil {
		// Unable to determine if a machine is a master machine.
		// Yet, it's only used to delete stopped machines that are not masters.
		// So we can safely continue to create a new machine since in the worst case
		// we just don't delete any stopped machine.
		klog.Errorf("%s: Error determining if machine is master: %v", r.machine.Name, err)
	} else {
		if !isMaster {
			// Prevent having a lot of stopped nodes sitting around.
			if err = removeStoppedMachine(r.machine, r.powerVSClient); err != nil {
				return fmt.Errorf("unable to remove stopped machines: %w", err)
			}
		}
	}

	userData, err := r.machineScope.getUserData()
	if err != nil {
		return fmt.Errorf("failed to get user data: %w", err)
	}

	instance, err := launchInstance(r.machine, r.providerSpec, userData, r.powerVSClient)
	if err != nil {
		klog.Errorf("%s: error creating machine: %v", r.machine.Name, err)
		conditionFailed := conditionFailed()
		conditionFailed.Message = err.Error()
		r.machineScope.setProviderStatus(nil, conditionFailed)
		return fmt.Errorf("failed to launch instance: %w", err)
	}

	klog.Infof("Created Machine %v", r.machine.Name)
	if err = r.setProviderID(instance); err != nil {
		return fmt.Errorf("failed to update machine object with providerID: %w", err)
	}

	if err = r.setMachineCloudProviderSpecifics(instance); err != nil {
		return fmt.Errorf("failed to set machine cloud provider specifics: %w", err)
	}

	r.machineScope.setProviderStatus(instance, conditionSuccess())

	return r.requeueIfInstanceBuilding(instance)
}

// delete deletes machine
func (r *Reconciler) delete() error {
	klog.Infof("%s: deleting machine", r.machine.Name)

	existingInstance, err := r.getMachineInstance()
	if err != nil && err != client.ErrorInstanceNotFound {
		metrics.RegisterFailedInstanceDelete(&metrics.MachineLabels{
			Name:      r.machine.Name,
			Namespace: r.machine.Namespace,
			Reason:    err.Error(),
		})
		klog.Errorf("%s: error getting existing instances: %v", r.machine.Name, err)
		return err
	} else if err == client.ErrorInstanceNotFound {
		klog.Warningf("%s: no instances found to delete for machine", r.machine.Name)
		return nil
	}

	err = r.powerVSClient.DeleteInstance(*existingInstance.PvmInstanceID)
	if err != nil {
		metrics.RegisterFailedInstanceDelete(&metrics.MachineLabels{
			Name:      r.machine.Name,
			Namespace: r.machine.Namespace,
			Reason:    err.Error(),
		})
		return fmt.Errorf("failed to delete instaces: %w", err)
	}

	klog.Infof("Deleted machine %v", r.machine.Name)

	return nil
}

// update finds a vm and reconciles the machine resource status against it.
func (r *Reconciler) update() error {
	klog.Infof("%s: updating machine", r.machine.Name)

	if err := validateMachine(*r.machine); err != nil {
		return fmt.Errorf("%v: failed validating machine provider spec: %v", r.machine.GetName(), err)
	}

	existingInstance, err := r.getMachineInstance()
	if err != nil {
		metrics.RegisterFailedInstanceUpdate(&metrics.MachineLabels{
			Name:      r.machine.Name,
			Namespace: r.machine.Namespace,
			Reason:    err.Error(),
		})
		klog.Errorf("%s: error getting existing instance: %v", r.machine.Name, err)
		return err
	}

	if err = r.setProviderID(existingInstance); err != nil {
		return fmt.Errorf("failed to update machine object with providerID: %w", err)
	}

	if err = r.setMachineCloudProviderSpecifics(existingInstance); err != nil {
		return fmt.Errorf("failed to set machine cloud provider specifics: %w", err)
	}

	klog.Infof("Updated machine %s", r.machine.Name)

	r.machineScope.setProviderStatus(existingInstance, conditionSuccess())

	return r.requeueIfInstanceBuilding(existingInstance)
}

// exists returns true if machine exists.
func (r *Reconciler) exists() (bool, error) {

	existingInstance, err := r.getMachineInstance()
	if err != nil && err != client.ErrorInstanceNotFound {
		// Reporting as update here, as successfull return value from the method
		// later indicases that an instance update flow will be executed.
		metrics.RegisterFailedInstanceUpdate(&metrics.MachineLabels{
			Name:      r.machine.Name,
			Namespace: r.machine.Namespace,
			Reason:    err.Error(),
		})
		klog.Errorf("%s: error getting existing instances: %v", r.machine.Name, err)
		return false, err
	}

	if existingInstance == nil {
		if r.machine.Spec.ProviderID != nil && *r.machine.Spec.ProviderID != "" && (r.machine.Status.LastUpdated == nil || r.machine.Status.LastUpdated.Add(requeueAfterSeconds*time.Second).After(time.Now())) {
			klog.Infof("%s: Possible eventual-consistency discrepancy; returning an error to requeue", r.machine.Name)
			return false, &machinecontroller.RequeueAfterError{RequeueAfter: requeueAfterSeconds * time.Second}
		}

		klog.Infof("%s: Instance does not exist", r.machine.Name)
		return false, nil
	}

	return true, nil
}

// isMaster returns true if the machine is part of a cluster's control plane
func (r *Reconciler) isMaster() (bool, error) {
	if r.machine.Status.NodeRef == nil {
		klog.Errorf("NodeRef not found in machine %s", r.machine.Name)
		return false, nil
	}
	node := &corev1.Node{}
	nodeKey := types.NamespacedName{
		Namespace: r.machine.Status.NodeRef.Namespace,
		Name:      r.machine.Status.NodeRef.Name,
	}

	err := r.client.Get(r.Context, nodeKey, node)
	if err != nil {
		return false, fmt.Errorf("failed to get node from machine %s", r.machine.Name)
	}

	if _, exists := node.Labels[masterLabel]; exists {
		return true, nil
	}
	return false, nil
}

// setProviderID adds providerID in the machine spec
func (r *Reconciler) setProviderID(instance *models.PVMInstance) error {
	existingProviderID := r.machine.Spec.ProviderID
	if instance == nil {
		return nil
	}

	//TODO: need to figure out a proper providerID here!
	providerID := client.FormatProviderID(*instance.PvmInstanceID)

	if existingProviderID != nil && *existingProviderID == providerID {
		klog.Infof("%s: ProviderID already set in the machine Spec with value:%s", r.machine.Name, *existingProviderID)
		return nil
	}
	r.machine.Spec.ProviderID = &providerID
	klog.Infof("%s: ProviderID set at machine spec: %s", r.machine.Name, providerID)
	return nil
}

func (r *Reconciler) setMachineCloudProviderSpecifics(instance *models.PVMInstance) error {
	if instance == nil {
		return nil
	}

	if r.machine.Labels == nil {
		r.machine.Labels = make(map[string]string)
	}

	if r.machine.Spec.Labels == nil {
		r.machine.Spec.Labels = make(map[string]string)
	}

	if r.machine.Annotations == nil {
		r.machine.Annotations = make(map[string]string)
	}

	if instance.Status != nil {
		r.machine.Annotations[machinecontroller.MachineInstanceStateAnnotationName] = *instance.Status
	}

	return nil
}

func (r *Reconciler) requeueIfInstanceBuilding(instance *models.PVMInstance) error {
	// If machine state is still pending, we will return an error to keep the controllers
	// attempting to update status until it hits a more permanent state. This will ensure
	// we get a public IP populated more quickly.
	if instance.Status != nil && *instance.Status == client.InstanceStateNameBuild {
		klog.Infof("%s: Instance state still building, returning an error to requeue", r.machine.Name)
		return &machinecontroller.RequeueAfterError{RequeueAfter: requeueAfterSeconds * time.Second}
	}

	return nil
}

func (r *Reconciler) getMachineInstance() (*models.PVMInstance, error) {
	// If there is a non-empty instance ID, search using that, otherwise
	// fallback to filtering based on name
	if r.providerStatus.InstanceID != nil && *r.providerStatus.InstanceID != "" {
		i, err := r.powerVSClient.GetInstance(*r.providerStatus.InstanceID)
		if err != nil {
			klog.Warningf("%s: Failed to find existing instance by id %s: %v", r.machine.Name, *r.providerStatus.InstanceID, err)
		} else {
			klog.Infof("%s: Found instance by id: %s", r.machine.Name, *r.providerStatus.InstanceID)
			return i, nil
		}
	}

	return r.powerVSClient.GetInstanceByName(r.machine.Name)
}
