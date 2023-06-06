package machine

import (
	"fmt"
	"strings"
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	machineapierros "github.com/openshift/machine-api-operator/pkg/controller/machine"
	machinecontroller "github.com/openshift/machine-api-operator/pkg/controller/machine"
	"github.com/openshift/machine-api-operator/pkg/metrics"
	"github.com/openshift/machine-api-provider-powervs/pkg/client"
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
			Reason:    "error getting existing instance",
		})
		klog.Errorf("%s: error getting existing instances: %v", r.machine.Name, err)
		return err
	} else if err == client.ErrorInstanceNotFound {
		klog.Warningf("%s: no instances found to delete for machine", r.machine.Name)
		// Remove the cached VM IP
		err = r.machineScope.dhcpIPCacheStore.Delete(vmIP{name: r.machine.Name})
		if err != nil {
			klog.Errorf("failed to delete the VM: %s entry from DHCP cache store err: %v", r.machine.Name, err)
		}
		return nil
	}

	err = r.powerVSClient.DeleteInstance(*existingInstance.PvmInstanceID)
	if err != nil {
		metrics.RegisterFailedInstanceDelete(&metrics.MachineLabels{
			Name:      r.machine.Name,
			Namespace: r.machine.Namespace,
			Reason:    "failed to delete instance",
		})
		return fmt.Errorf("failed to delete instaces: %w", err)
	}
	// Remove the cached VM IP
	err = r.machineScope.dhcpIPCacheStore.Delete(vmIP{name: r.machine.Name})
	if err != nil {
		klog.Errorf("failed to delete the VM: %s entry from DHCP cache store err: %v", r.machine.Name, err)
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
			Reason:    "error getting existing instance",
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

	r.machineScope.setProviderStatus(existingInstance, conditionSuccess())

	// Fetch and update the IP for machine object
	if err := r.setMachineAddresses(existingInstance); err != nil {
		klog.Errorf("Failed to fetch and update an IP address for the machine: %s error: %v", r.machine.Name, err)
	}
	klog.Infof("Updated machine %s", r.machine.Name)
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
			Reason:    "error getting existing instance",
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
	providerStatus, err := ProviderStatusFromRawExtension(r.machine.Status.ProviderStatus)
	if err != nil {
		return machineapierros.InvalidMachineConfiguration("failed to get machine provider status: %v", err.Error())
	}

	var serviceInstanceID string
	if providerStatus.ServiceInstanceID != nil {
		serviceInstanceID = *providerStatus.ServiceInstanceID
		klog.Infof("Found ServiceInstanceID from providerStatus %s", serviceInstanceID)
	} else {
		errStr := fmt.Errorf("serviceInstanceID is empty, Cannot set providerID")
		klog.Errorf(errStr.Error())
		return errStr
	}

	providerID := client.FormatProviderID(r.powerVSClient.GetRegion(), r.powerVSClient.GetZone(), serviceInstanceID, *instance.PvmInstanceID)

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

	region := r.powerVSClient.GetRegion()
	if region != "" {
		r.machine.Labels[machinecontroller.MachineRegionLabelName] = region
	}

	zone := r.powerVSClient.GetZone()
	if zone != "" {
		r.machine.Labels[machinecontroller.MachineAZLabelName] = zone
	}

	if instance.SysType != "" {
		r.machine.Labels[machinecontroller.MachineInstanceTypeLabelName] = instance.SysType
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

func (r *Reconciler) setMachineAddresses(instance *models.PVMInstance) error {
	if instance == nil {
		klog.Infof("VM instance is nil, Cannot fetch VM IP")
		return nil
	}
	var networkAddresses []corev1.NodeAddress

	// Set the NodeInternalDNS as VM name
	networkAddresses = append(networkAddresses,
		corev1.NodeAddress{
			Type:    corev1.NodeInternalDNS,
			Address: *instance.ServerName,
		})

	// Try to fetch the IP from instance networks fields
	for _, network := range instance.Networks {
		if strings.TrimSpace(network.ExternalIP) != "" {
			networkAddresses = append(networkAddresses,
				corev1.NodeAddress{
					Type:    corev1.NodeExternalIP,
					Address: strings.TrimSpace(network.ExternalIP),
				})
		}
		if strings.TrimSpace(network.IPAddress) != "" {
			networkAddresses = append(networkAddresses,
				corev1.NodeAddress{
					Type:    corev1.NodeInternalIP,
					Address: strings.TrimSpace(network.IPAddress),
				})
		}
	}
	r.machineScope.machine.Status.Addresses = networkAddresses
	if len(networkAddresses) > 1 {
		// If the networkAddress length is more than 1 means, either NodeInternalIP or NodeExternalIP is updated so return
		return nil
	}
	// In this case there is no IP found under instance.Networks, So try to fetch the IP from cache or DHCP server
	// Look for DHCP IP from the cache
	obj, exists, err := r.machineScope.dhcpIPCacheStore.GetByKey(*instance.ServerName)
	if err != nil {
		klog.Errorf("failed to fetch the DHCP IP address for VM : %s from cache store, error: %v", *instance.ServerName, err)
	}
	if exists {
		klog.Infof("found IP: %s for VM: %s from DHCP cache", obj.(vmIP).ip, *instance.ServerName)
		networkAddresses = append(networkAddresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: obj.(vmIP).ip,
		})
		r.machineScope.machine.Status.Addresses = networkAddresses
		return nil
	}
	// Fetch the VM network ID
	networkID, err := getNetworkID(r.providerSpec.Network, r.powerVSClient)
	if err != nil {
		errStr := fmt.Errorf("failed to fetch network id from network resource for VM: %s error: %v", r.machine.Name, err)
		klog.Errorf(errStr.Error())
		return errStr
	}
	// Fetch the details of the network attached to the VM
	var pvmNetwork *models.PVMInstanceNetwork
	for _, network := range instance.Networks {
		if network.NetworkID == *networkID {
			pvmNetwork = network
			klog.Infof("found network with ID %s attached to VM %s", network.NetworkID, *instance.ServerName)
		}
	}
	if pvmNetwork == nil {
		errStr := fmt.Errorf("failed to get network attached to VM %s with id %s", *instance.ServerName, *networkID)
		klog.Errorf(errStr.Error())
		return errStr
	}
	// Get all the DHCP servers
	dhcpServer, err := r.powerVSClient.GetDHCPServers()
	if err != nil {
		errStr := fmt.Errorf("failed to get DHCP server error: %v", err)
		klog.Errorf(errStr.Error())
		return errStr
	}
	// Get the Details of DHCP server associated with the network
	var dhcpServerDetails *models.DHCPServerDetail
	for _, server := range dhcpServer {
		if *server.Network.ID == *networkID {
			klog.Infof("found DHCP server with ID %s for network ID %s", *server.Network.ID, *networkID)
			dhcpServerDetails, err = r.powerVSClient.GetDHCPServerByID(*server.ID)
			if err != nil || dhcpServerDetails == nil {
				errStr := fmt.Errorf("failed to get DHCP server details with DHCP server ID: %s error: %v", *server.ID, err)
				klog.Errorf(errStr.Error())
				return errStr
			}
			break
		}
	}
	if dhcpServerDetails == nil {
		errStr := fmt.Errorf("DHCP server detailis not found for network with ID %s", *networkID)
		klog.Errorf(errStr.Error())
		return errStr
	}

	// Fetch the VM IP using VM's mac from DHCP server lease
	var internalIP *string
	for _, lease := range dhcpServerDetails.Leases {
		if *lease.InstanceMacAddress == pvmNetwork.MacAddress {
			klog.Infof("found internal ip %s for VM %s from DHCP lease", *lease.InstanceIP, *instance.ServerName)
			internalIP = lease.InstanceIP
			break
		}
	}
	if internalIP == nil {
		errStr := fmt.Errorf("failed to get internal IP, DHCP lease not found for VM %s with MAC %s in DHCP network %s", *instance.ServerName,
			pvmNetwork.MacAddress, *dhcpServerDetails.ID)
		klog.Errorf(errStr.Error())
		return errStr
	}
	klog.Infof("found internal IP: %s for VM: %s from DHCP lease", *internalIP, *instance.ServerName)
	networkAddresses = append(networkAddresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalIP,
		Address: *internalIP,
	})
	// Update the cache with the ip and VM name
	err = r.machineScope.dhcpIPCacheStore.Add(vmIP{
		name: *instance.ServerName,
		ip:   *internalIP,
	})
	if err != nil {
		klog.Errorf("failed to update the DHCP cache store with the IP for VM %s error %v", *instance.ServerName, err)
	}
	r.machineScope.machine.Status.Addresses = networkAddresses
	return nil
}
