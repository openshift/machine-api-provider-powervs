package client

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	configv1 "github.com/openshift/api/config/v1"

	. "github.com/onsi/gomega"
)

var (
	customEndpointsMap = map[string]string{
		"iam": "https://test.iam.cloud.ibm.com",
		"rc":  "https://test.resource-controller.cloud.ibm.com",
		"pi":  "https://test-region.power-iaas.cloud.ibm.com",
	}
	testRegion = "test-region"
)

func TestSetEnvironmentVariables(t *testing.T) {
	err := setEnvironmentVariables(regionEnvironmentalVariable, testRegion)
	if err != nil {
		t.Fatal(err)
	}

	regionFound := getEnvironmentalVariableValue(regionEnvironmentalVariable)
	if regionFound != testRegion {
		t.Fatalf("Expected region %s got %s ", testRegion, regionFound)
	}
}

func TestSetCustomEndpoints(t *testing.T) {
	if err := setCustomEndpoints(customEndpointsMap, getCustomEndPointKeys(customEndpointsMap)); err != nil {
		t.Fatal(err)
	}
	for _, key := range getCustomEndPointKeys(customEndpointsMap) {
		val := getEnvironmentalVariableValue(endPointKeyToEnvNameMap[key])
		if val != customEndpointsMap[key] {
			t.Fatalf("Expected value %s got %s ", customEndpointsMap[key], val)
		}
	}
}

func TestResolveEndpoints(t *testing.T) {
	var cfg *rest.Config
	var k8sClient client.Client
	var err error
	ctx := context.Background()

	g := NewWithT(t)

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crds"),
			filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", "config", "v1"),
		},
	}
	configv1.AddToScheme(scheme.Scheme)

	cfg, err = testEnv.Start()
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(cfg).ToNot(BeNil())

	defer func() {
		if err := testEnv.Stop(); err != nil {
			log.Fatal(err)
		}
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	g.Expect(err).ToNot(HaveOccurred())

	infraObject := stubInfrastructure()
	g.Expect(k8sClient.Create(ctx, infraObject)).To(Succeed())
	defer func() {
		g.Expect(k8sClient.Delete(ctx, infraObject)).To(Succeed())
	}()

	infraObjectName := client.ObjectKey{Name: globalInfrastuctureName}

	g.Expect(k8sClient.Get(ctx, infraObjectName, infraObject)).To(Succeed())

	infraObject.Status = stubStatus()

	g.Expect(k8sClient.Status().Update(ctx, infraObject)).To(Succeed())

	endpointsMap, err := resolveEndpoints(k8sClient)
	if err != nil {
		t.Fatal(err)
	}

	if len(endpointsMap) != 2 {
		log.Fatalf("Expected length of endpointsMap is 2 but got %d", len(endpointsMap))
	}

	endpointKeys := []string{"iam", "rc"}

	for _, key := range endpointKeys {
		if val, ok := endpointsMap[key]; !ok {
			log.Fatalf("Expected %s endpoint is not present in the customEndpointsMap", key)
		} else if val != customEndpointsMap[key] {
			log.Fatalf("Expected %s endpoint value is %s but got %s", key, customEndpointsMap["iam"], val)
		}
	}
}

func stubInfrastructure() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}
}

func stubStatus() configv1.InfrastructureStatus {
	return configv1.InfrastructureStatus{
		ControlPlaneTopology:   "HighlyAvailable",
		InfrastructureTopology: "HighlyAvailable",
		Platform:               "PowerVS",
		PlatformStatus: &configv1.PlatformStatus{
			Type: "PowerVS",
			PowerVS: &configv1.PowerVSPlatformStatus{
				ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
					{
						Name: "iam",
						URL:  customEndpointsMap["iam"],
					},
					{
						Name: "rc",
						URL:  customEndpointsMap["rc"],
					},
				},
			},
		},
	}
}

func getEnvironmentalVariableValue(key string) string {
	return os.Getenv(key)
}
