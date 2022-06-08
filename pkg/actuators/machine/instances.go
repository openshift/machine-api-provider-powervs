package machine

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/IBM-Cloud/power-go-client/power/models"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	mapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
)

// removeStoppedMachine removes all instances of a specific machine that are in a stopped state.
func removeStoppedMachine(machine *machinev1beta1.Machine, client powervsclient.Client) error {
	instance, err := client.GetInstanceByName(machine.Name)
	if err != nil && err != powervsclient.ErrorInstanceNotFound {
		klog.Errorf("Error getting instance by name: %s, err: %v", machine.Name, err)
		return fmt.Errorf("error getting instance by name: %s, err: %v", machine.Name, err)
	} else if err == powervsclient.ErrorInstanceNotFound {
		klog.Infof("Instance not found with name: %s", machine.Name)
		return nil
	}

	if instance != nil && *instance.Status == powervsclient.InstanceStateNameShutoff {
		return client.DeleteInstance(*instance.PvmInstanceID)
	}
	return nil
}

func launchInstance(machine *machinev1beta1.Machine, machineProviderConfig *machinev1.PowerVSMachineProviderConfig, userData []byte, client powervsclient.Client) (*models.PVMInstance, error) {
	var processors float64
	var err error

	switch machineProviderConfig.Processors.Type {
	case intstr.Int:
		processors = float64(machineProviderConfig.Processors.IntVal)
	case intstr.String:
		processors, err = strconv.ParseFloat(machineProviderConfig.Processors.StrVal, 64)
		if err != nil {
			return nil, mapierrors.InvalidMachineConfiguration("failed to convert Processors %s to float64", machineProviderConfig.Processors.StrVal)
		}
	}

	imageID, err := getImageID(machineProviderConfig.Image, client)
	if err != nil {
		return nil, mapierrors.InvalidMachineConfiguration("error getting image ID: %v", err)
	}

	networkID, err := getNetworkID(machineProviderConfig.Network, client)
	if err != nil {
		return nil, mapierrors.InvalidMachineConfiguration("error getting network ID: %v", err)
	}

	var nets = []*models.PVMInstanceAddNetwork{
		{NetworkID: networkID},
	}

	memory := float64(machineProviderConfig.MemoryGiB)
	procType := strings.ToLower(string(machineProviderConfig.ProcessorType))

	params := &models.PVMInstanceCreate{
		ImageID:     imageID,
		KeyPairName: machineProviderConfig.KeyPairName,
		Networks:    nets,
		ServerName:  &machine.Name,
		Memory:      &memory,
		Processors:  &processors,
		ProcType:    &procType,
		SysType:     strings.ToLower(machineProviderConfig.SystemType),
		UserData:    base64.StdEncoding.EncodeToString(userData),
	}

	instances, err := client.CreateInstance(params)
	if err != nil {
		return nil, mapierrors.CreateMachine("error creating powervs instance: %v", err)
	}

	insIDs := make([]string, 0)
	for _, in := range *instances {
		insID := in.PvmInstanceID
		insIDs = append(insIDs, *insID)
	}

	if len(insIDs) == 0 {
		return nil, mapierrors.CreateMachine("error getting the instance ID post deployment for: %s", machine.Name)
	}

	instance, err := client.GetInstance(insIDs[0])
	if err != nil {
		return nil, mapierrors.CreateMachine("error getting the instance for ID: %s", insIDs[0])
	}
	return instance, nil
}

func getImageID(imageResource machinev1.PowerVSResource, client powervsclient.Client) (*string, error) {
	switch imageResource.Type {
	case machinev1.PowerVSResourceTypeID:
		if imageResource.ID == nil {
			return nil, fmt.Errorf("imageResource reference is specified as ID but it is nil")
		}
		return imageResource.ID, nil
	case machinev1.PowerVSResourceTypeName:
		if imageResource.Name == nil {
			return nil, fmt.Errorf("imageResource reference is specified as Name but it is nil")
		}
		images, err := client.GetImages()
		if err != nil {
			klog.Errorf("failed to get images, err: %v", err)
			return nil, err
		}
		for _, img := range images.Images {
			if *imageResource.Name == *img.Name {
				klog.Infof("image %s found with ID: %s", *imageResource.Name, *img.ImageID)
				return img.ImageID, nil
			}
		}
		return nil, fmt.Errorf("failed to find an image ID with name %s", *imageResource.Name)
	default:
		return nil, fmt.Errorf("failed to find an image ID, Unexpected imageResource type %s supports only %s and %s", imageResource.Type, machinev1.PowerVSResourceTypeID, machinev1.PowerVSResourceTypeName)
	}
}

func getNetworkID(networkResource machinev1.PowerVSResource, client powervsclient.Client) (*string, error) {
	switch networkResource.Type {
	case machinev1.PowerVSResourceTypeID:
		if networkResource.ID == nil {
			return nil, fmt.Errorf("networkResource reference is specified as ID but it is nil")
		}
		return networkResource.ID, nil
	case machinev1.PowerVSResourceTypeName:
		if networkResource.Name == nil {
			return nil, fmt.Errorf("networkResource reference is specified as Name but it is nil")
		}
		networks, err := client.GetNetworks()
		if err != nil {
			klog.Errorf("failed to get networks, err: %v", err)
			return nil, err
		}
		for _, nw := range networks.Networks {
			if *networkResource.Name == *nw.Name {
				klog.Infof("network %s found with ID: %s", *networkResource.Name, *nw.NetworkID)
				return nw.NetworkID, nil
			}
		}
		return nil, fmt.Errorf("failed to find an network ID with name %s", *networkResource.Name)
	case machinev1.PowerVSResourceTypeRegEx:
		if networkResource.RegEx == nil {
			return nil, fmt.Errorf("networkResource reference is specified as RegEx but it is nil")
		}
		var (
			re  *regexp.Regexp
			err error
		)
		networks, err := client.GetNetworks()
		if err != nil {
			klog.Errorf("failed to get networks, err: %v", err)
			return nil, err
		}
		if re, err = regexp.Compile(*networkResource.RegEx); err != nil {
			return nil, err
		}
		for _, nw := range networks.Networks {
			if match := re.Match([]byte(*nw.Name)); match {
				klog.Infof("DHCP network %s found with ID: %s", *nw.Name, *nw.NetworkID)
				return nw.NetworkID, nil
			}
		}
		return nil, fmt.Errorf("failed to find an network ID with RegEx %s", *networkResource.RegEx)
	default:
		return nil, fmt.Errorf("failed to find an network ID, Unexpected networkResource type %s supports only %s, %s and %s", networkResource.Type, machinev1.PowerVSResourceTypeID, machinev1.PowerVSResourceTypeName, machinev1.PowerVSResourceTypeRegEx)
	}
}

func getServiceInstanceID(serviceInstanceResource machinev1.PowerVSResource, client powervsclient.Client) (*string, error) {
	switch serviceInstanceResource.Type {
	case machinev1.PowerVSResourceTypeID:
		if serviceInstanceResource.ID == nil {
			return nil, fmt.Errorf("serviceInstanceResource reference is specified as ID but it is nil")
		}
		klog.Infof("Found the service with ID %s", *serviceInstanceResource.ID)
		return serviceInstanceResource.ID, nil
	case machinev1.PowerVSResourceTypeName:
		if serviceInstanceResource.Name == nil {
			return nil, fmt.Errorf("serviceInstanceResource reference is specified as Name but it is nil")
		}
		serviceInstances, err := client.GetCloudServiceInstanceByName(*serviceInstanceResource.Name)
		if err != nil {
			klog.Errorf("failed to get serviceInstances, err: %v", err)
			return nil, err
		}
		// log useful error message
		switch len(serviceInstances) {
		case 0:
			errStr := fmt.Errorf("does exist any cloud service instance with name %s", *serviceInstanceResource.Name)
			klog.Errorf(errStr.Error())
			return nil, errStr
		case 1:
			klog.Infof("serviceInstance %s found with ID: %s", *serviceInstanceResource.Name, serviceInstances[0].Guid)
			return &serviceInstances[0].Guid, nil
		default:
			errStr := fmt.Errorf("there exist more than one service instance ID with with same name %s, Try setting serviceInstance.ID", *serviceInstanceResource.Name)
			klog.Errorf(errStr.Error())
			return nil, errStr
		}
	default:
		return nil, fmt.Errorf("failed to find an ServiceInstanceID, Unexpected serviceInstanceResource type: %s supports only %s and %s", serviceInstanceResource.Type, machinev1.PowerVSResourceTypeID, machinev1.PowerVSResourceTypeName)
	}
}
