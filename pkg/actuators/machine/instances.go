package machine

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"

	"github.com/IBM-Cloud/power-go-client/power/models"

	"k8s.io/klog/v2"

	machinev1 "github.com/openshift/api/machine/v1beta1"
	mapierrors "github.com/openshift/machine-api-operator/pkg/controller/machine"
	powervsproviderv1 "github.com/openshift/machine-api-provider-powervs/pkg/apis/powervsprovider/v1alpha1"
	powervsclient "github.com/openshift/machine-api-provider-powervs/pkg/client"
)

// removeStoppedMachine removes all instances of a specific machine that are in a stopped state.
func removeStoppedMachine(machine *machinev1.Machine, client powervsclient.Client) error {
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

func launchInstance(machine *machinev1.Machine, machineProviderConfig *powervsproviderv1.PowerVSMachineProviderConfig, userData []byte, client powervsclient.Client) (*models.PVMInstance, error) {
	// code for powervs

	memory, err := strconv.ParseFloat(machineProviderConfig.Memory, 64)
	if err != nil {
		return nil, mapierrors.InvalidMachineConfiguration("failed to convert memory(%s) to float64", machineProviderConfig.Memory)
	}
	processors, err := strconv.ParseFloat(machineProviderConfig.Processors, 64)
	if err != nil {
		return nil, mapierrors.InvalidMachineConfiguration("failed to convert Cores(%s) to float64", machineProviderConfig.Processors)
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

	params := &models.PVMInstanceCreate{
		ImageID:     imageID,
		KeyPairName: machineProviderConfig.KeyPairName,
		Networks:    nets,
		ServerName:  &machine.Name,
		Memory:      &memory,
		Processors:  &processors,
		ProcType:    &machineProviderConfig.ProcType,
		SysType:     machineProviderConfig.SysType,
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

func getImageID(image powervsproviderv1.PowerVSResourceReference, client powervsclient.Client) (*string, error) {
	if image.ID != nil {
		return image.ID, nil
	} else if image.Name != nil {
		images, err := client.GetImages()
		if err != nil {
			klog.Errorf("failed to get images, err: %v", err)
			return nil, err
		}
		for _, img := range images.Images {
			if *image.Name == *img.Name {
				klog.Infof("image %s found with ID: %s", *image.Name, *img.ImageID)
				return img.ImageID, nil
			}
		}
	} else {
		return nil, fmt.Errorf("both ID and Name can't be nil")
	}
	return nil, fmt.Errorf("failed to find an image ID")
}

func getNetworkID(network powervsproviderv1.PowerVSResourceReference, client powervsclient.Client) (*string, error) {
	if network.ID != nil {
		return network.ID, nil
	}
	networks, err := client.GetNetworks()
	if err != nil {
		klog.Errorf("failed to get networks, err: %v", err)
		return nil, err
	}
	if network.Name != nil {
		for _, nw := range networks.Networks {
			if *network.Name == *nw.Name {
				klog.Infof("network %s found with ID: %s", *network.Name, *nw.NetworkID)
				return nw.NetworkID, nil
			}
		}
	} else if network.RegEx != nil {
		var (
			re  *regexp.Regexp
			err error
		)

		if re, err = regexp.Compile(*network.RegEx); err != nil {
			return nil, err
		}
		for _, nw := range networks.Networks {
			if match := re.Match([]byte(*nw.Name)); match {
				klog.Infof("DHCP network %s found with ID: %s", *nw.Name, *nw.NetworkID)
				return nw.NetworkID, nil
			}
		}
	} else {
		return nil, fmt.Errorf("specify one of ID, Name, or RegEx")
	}
	return nil, fmt.Errorf("failed to find a network ID")
}
