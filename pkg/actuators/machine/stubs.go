package machine

import (
	"fmt"
	"strconv"

	"k8s.io/utils/pointer"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"

	machinev1 "github.com/openshift/api/machine/v1"
	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
)

const (
	defaultNamespace      = "default"
	credentialsSecretName = "powervs-credentials"
	userDataSecretName    = "powervs-actuator-user-data-secret"
	nameLength            = 5
	imageNamePrefix       = "test-image"
	networkNamePrefix     = "test-network"
	testRegion            = "test-region"
	testZone              = "test-zone"
	instanceID            = "testInstanceID"
	instanceName          = "testInstanceName"
	inValidInstance       = "testInValidInstanceName"
	instanceGUID          = "testGUID"
)

var powerVSProviderID = fmt.Sprintf("powervs://test-region/test-zone/test-service-instanceid/test-instanceid")

func stubUserDataSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Data: map[string][]byte{
			userDataSecretKey: []byte("userDataBlob"),
		},
	}
}

func stubPowerVSCredentialsSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaultNamespace,
		},
		Data: map[string][]byte{
			"ibmcloud_api_key": []byte("api_key"),
		},
	}
}

func stubMachine() (*machinev1beta1.Machine, error) {

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	providerSpec, err := RawExtensionFromProviderSpec(stubProviderConfig(credSecretName))
	if err != nil {
		return nil, err
	}

	return &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: defaultNamespace,
			Labels: map[string]string{
				machinev1beta1.MachineClusterIDLabel: "CLUSTERID",
			},
		},
		Spec: machinev1beta1.MachineSpec{
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: providerSpec,
			},
		}}, nil
}

func stubProviderConfig(name string) *machinev1.PowerVSMachineProviderConfig {
	testKeyPair := "Test-KeyPair"
	return &machinev1.PowerVSMachineProviderConfig{
		CredentialsSecret: &machinev1.PowerVSSecretReference{
			Name: name,
		},
		MemoryGiB:   32,
		Processors:  intstr.FromString("0.5"),
		KeyPairName: testKeyPair,
		Image: machinev1.PowerVSResource{
			Type: machinev1.PowerVSResourceTypeName,
			Name: core.StringPtr(imageNamePrefix + "-1"),
		},
		Network: machinev1.PowerVSResource{
			Type: machinev1.PowerVSResourceTypeName,
			Name: core.StringPtr(networkNamePrefix + "-1"),
		},
		ServiceInstance: machinev1.PowerVSResource{
			Type: machinev1.PowerVSResourceTypeID,
			ID:   core.StringPtr(instanceID),
		},
	}
}

func stubProviderStatus(serviceInstanceID string) *machinev1.PowerVSMachineProviderStatus {
	return &machinev1.PowerVSMachineProviderStatus{
		ServiceInstanceID: core.StringPtr(serviceInstanceID),
	}
}

func stubGetInstances() *models.PVMInstanceList {
	return &models.PVMInstanceList{stubGetInstance()}
}

func stubGetInstance() *models.PVMInstance {
	dummyInstanceID := "instance-id"
	status := "ACTIVE"
	return &models.PVMInstance{
		PvmInstanceID: &dummyInstanceID,
		Status:        &status,
		ServerName:    core.StringPtr("instance"),
	}
}

func stubInstanceWithBuildState() *models.PVMInstance {
	return &models.PVMInstance{
		PvmInstanceID: pointer.String("instance-id"),
		Status:        pointer.String("BUILD"),
		ServerName:    core.StringPtr("instance"),
	}
}

func stubGetImages(nameprefix string, count int) *models.Images {
	images := &models.Images{
		Images: []*models.ImageReference{},
	}
	for i := 0; i < count; i++ {
		images.Images = append(images.Images,
			&models.ImageReference{
				Name:    core.StringPtr(nameprefix + "-" + strconv.Itoa(i)),
				ImageID: core.StringPtr("ID-" + nameprefix + "-" + strconv.Itoa(i)),
			})
	}
	return images
}

func stubGetNetworks(nameprefix string, count int) *models.Networks {
	images := &models.Networks{
		Networks: []*models.NetworkReference{},
	}
	for i := 0; i < count; i++ {
		images.Networks = append(images.Networks,
			&models.NetworkReference{
				Name:      core.StringPtr(nameprefix + "-" + strconv.Itoa(i)),
				NetworkID: core.StringPtr("ID-" + nameprefix + "-" + strconv.Itoa(i)),
			})
	}
	return images
}

func stubGetDHCPServers(serverID, networkID string) models.DHCPServers {
	return models.DHCPServers{
		&models.DHCPServer{
			ID: pointer.StringPtr(serverID),
			Network: &models.DHCPServerNetwork{
				ID: pointer.StringPtr(networkID),
			},
		},
	}
}

func stubGetDHCPServerByID(id, ip, mac string) *models.DHCPServerDetail {
	return &models.DHCPServerDetail{
		ID: pointer.StringPtr(id),
		Leases: []*models.DHCPServerLeases{
			{
				InstanceIP:         pointer.StringPtr(ip),
				InstanceMacAddress: pointer.StringPtr(mac),
			},
		},
	}
}

func stubGetNetworkWithName(name, id string) *models.Networks {
	networks := &models.Networks{
		Networks: []*models.NetworkReference{
			{
				Name:      pointer.StringPtr(name),
				NetworkID: pointer.StringPtr(id),
			},
		},
	}
	return networks
}

func stubControlPlaneMachine(loadBalancerNames []string, loadBalancerType machinev1.IBMVPCLoadBalancerType) (*machinev1beta1.Machine, error) {

	credSecretName := fmt.Sprintf("%s-%s", credentialsSecretName, rand.String(nameLength))
	cpMachine := stubProviderConfig(credSecretName)

	for _, lb := range loadBalancerNames {
		cpMachine.LoadBalancers = append(cpMachine.LoadBalancers, machinev1.LoadBalancerReference{
			Name: lb,
			Type: loadBalancerType,
		})
	}
	providerSpec, err := RawExtensionFromProviderSpec(cpMachine)
	if err != nil {
		return nil, err
	}

	return &machinev1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vm",
			Namespace: defaultNamespace,
			Labels: map[string]string{
				machinev1beta1.MachineClusterIDLabel: "CLUSTERID",
			},
		},
		Spec: machinev1beta1.MachineSpec{
			ProviderSpec: machinev1beta1.ProviderSpec{
				Value: providerSpec,
			},
		}}, nil
}

func stubGetLoadBalancerResult() *vpcv1.LoadBalancer {
	return &vpcv1.LoadBalancer{
		Name:               pointer.String("test-lb"),
		ID:                 pointer.String("id"),
		ProvisioningStatus: pointer.String(loadBalancerActiveState),
	}
}

func stubGetLoadBalancerCollections() *vpcv1.LoadBalancerCollection {
	return &vpcv1.LoadBalancerCollection{
		LoadBalancers: []vpcv1.LoadBalancer{
			{
				Name:               pointer.String("test-lb"),
				ID:                 pointer.String("id"),
				ProvisioningStatus: pointer.String(loadBalancerActiveState),
				Pools: []vpcv1.LoadBalancerPoolReference{
					{
						ID:   pointer.String("pool-id"),
						Name: pointer.String("pool-name"),
					},
				},
			},
		},
	}
}
