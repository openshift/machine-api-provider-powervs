package controllers

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	configv1 "github.com/openshift/api/config/v1"

	. "github.com/onsi/gomega"
)

func TestResolveEndpoints(t *testing.T) {
	var cfg *rest.Config
	var k8sClient client.Client
	var err error
	ctx := context.Background()

	g := NewWithT(t)

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", "machine", "v1beta1", "zz_generated.crd-manifests", "0000_10_machine-api_01_machines-CustomNoUpgrade.crd.yaml"),
			filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", "config", "v1", "zz_generated.crd-manifests", "0000_10_config-operator_01_infrastructures-CustomNoUpgrade.crd.yaml"),
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

	cases := []struct {
		name             string
		obj              *configv1.Infrastructure
		expectedInfraObj *configv1.Infrastructure
		updateStatus     configv1.InfrastructureStatus
		expectedErr      error
	}{
		{
			name: "with no service endpoints",
			obj: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
			},
			expectedInfraObj: &configv1.Infrastructure{},
		},
		{
			name:        "with invalid status in infrastructure object",
			obj:         stubInfrastructure(),
			expectedErr: errors.New("infrastructure.config.openshift.io \"cluster\" is invalid"),
		},
		{
			name:        "with invalid Custom Service Endpoints",
			obj:         stubInvalidCustomServiceEndpoints(),
			expectedErr: errors.New("invalid value: \"https://dal.power-iaas.test.cloud.ibm.com/abc\": no path or request parameters must be provided, \"/abc\" was provided"),
		},
		{
			name: "with valid custom service endpoints",
			obj:  stubInfrastructure(),
			updateStatus: configv1.InfrastructureStatus{
				ControlPlaneTopology:   "HighlyAvailable",
				InfrastructureTopology: "HighlyAvailable",
				PlatformStatus: &configv1.PlatformStatus{
					PowerVS: &configv1.PowerVSPlatformStatus{
						ResourceGroup: "Default",
					},
				},
			},
			expectedInfraObj: expectedInfraObject(),
		},
		{
			name: "with existing custom service endpoints in infrastructure object",
			obj:  stubInfrastructure(),
			updateStatus: configv1.InfrastructureStatus{
				ControlPlaneTopology:   "HighlyAvailable",
				InfrastructureTopology: "HighlyAvailable",
				PlatformStatus: &configv1.PlatformStatus{
					PowerVS: &configv1.PowerVSPlatformStatus{
						ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
							{
								Name: "IAM",
								URL:  "https://iam.test.cloud.ibm.com",
							},
							{
								Name: "Power",
								URL:  "https://dal.power-iaas.test.cloud.ibm.com",
							},
						},
						ResourceGroup: "Default",
					},
				},
			},
			expectedInfraObj: expectedInfraObject(),
		},
		{
			name: "adding new endpoint to the existing custom service endpoints",
			obj: &configv1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.InfrastructureSpec{
					PlatformSpec: configv1.PlatformSpec{
						Type: "PowerVS",
						PowerVS: &configv1.PowerVSPlatformSpec{
							ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
								{
									Name: "IAM",
									URL:  "https://iam.test.cloud.ibm.com",
								},
								{
									Name: "Power",
									URL:  "https://dal.power-iaas.test.cloud.ibm.com",
								},
								{
									Name: "ResourceController",
									URL:  "https://test.resource-controller.cloud.ibm.com",
								},
							},
						},
					},
				},
			},
			updateStatus: configv1.InfrastructureStatus{
				ControlPlaneTopology:   "HighlyAvailable",
				InfrastructureTopology: "HighlyAvailable",
				PlatformStatus: &configv1.PlatformStatus{
					PowerVS: &configv1.PowerVSPlatformStatus{
						ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
							{
								Name: "IAM",
								URL:  "https://iam.test.cloud.ibm.com",
							},
							{
								Name: "Power",
								URL:  "https://dal.power-iaas.test.cloud.ibm.com",
							},
						},
						ResourceGroup: "Default",
					},
				},
			},
			expectedInfraObj: &configv1.Infrastructure{
				Status: configv1.InfrastructureStatus{
					ControlPlaneTopology:   "HighlyAvailable",
					InfrastructureTopology: "HighlyAvailable",
					CPUPartitioning:        configv1.CPUPartitioningNone,
					PlatformStatus: &configv1.PlatformStatus{
						PowerVS: &configv1.PowerVSPlatformStatus{
							ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
								{
									Name: "IAM",
									URL:  "https://iam.test.cloud.ibm.com",
								},
								{
									Name: "Power",
									URL:  "https://dal.power-iaas.test.cloud.ibm.com",
								},
								{
									Name: "ResourceController",
									URL:  "https://test.resource-controller.cloud.ibm.com",
								},
							},
							ResourceGroup: "Default",
						},
					},
				},
			},
		},
		{
			name: "removing existing custom service endpoints in infrastructure object",
			obj:  stubInfrastructure(),
			updateStatus: configv1.InfrastructureStatus{
				ControlPlaneTopology:   "HighlyAvailable",
				InfrastructureTopology: "HighlyAvailable",
				PlatformStatus: &configv1.PlatformStatus{
					PowerVS: &configv1.PowerVSPlatformStatus{
						ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
							{
								Name: "IAM",
								URL:  "https://iam.test.cloud.ibm.com",
							},
							{
								Name: "Power",
								URL:  "https://dal.power-iaas.test.cloud.ibm.com",
							},
							{
								Name: "ResourceController",
								URL:  "https://test.resource-controller.cloud.ibm.com",
							},
						},
						ResourceGroup: "Default",
					},
				},
			},
			expectedInfraObj: expectedInfraObject(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.Expect(k8sClient.Create(ctx, tc.obj)).To(Succeed())
			defer func() {
				g.Expect(k8sClient.Delete(ctx, tc.obj)).To(Succeed())
			}()

			if tc.updateStatus.ControlPlaneTopology != "" {
				infraObject := &configv1.Infrastructure{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: infrastructureResourceName}, infraObject)).To(Succeed())
				infraObject.Status = tc.updateStatus
				g.Expect(k8sClient.Status().Update(ctx, infraObject)).To(Succeed())
			}

			r := Reconciler{
				Client:   k8sClient,
				recorder: &record.FakeRecorder{},
				Log:      ctrl.Log.WithName("controllers").WithName("CustomServiceEndpoint"),
			}
			_, err := r.Reconcile(context.Background(), ctrl.Request{})
			if tc.expectedErr != nil {
				g.Expect(strings.ToLower(err.Error())).To(ContainSubstring(tc.expectedErr.Error()))
			} else {
				infraObject := &configv1.Infrastructure{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: infrastructureResourceName}, infraObject)).To(Succeed())
				g.Expect(infraObject.Status).To(Equal(tc.expectedInfraObj.Status))
			}
		})
	}
}

func stubInfrastructure() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.InfrastructureSpec{
			PlatformSpec: configv1.PlatformSpec{
				Type: "PowerVS",
				PowerVS: &configv1.PowerVSPlatformSpec{
					ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
						{
							Name: "IAM",
							URL:  "https://iam.test.cloud.ibm.com",
						},
						{
							Name: "Power",
							URL:  "https://dal.power-iaas.test.cloud.ibm.com",
						},
					},
				},
			},
		},
	}
}

func stubInvalidCustomServiceEndpoints() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.InfrastructureSpec{
			PlatformSpec: configv1.PlatformSpec{
				Type: "PowerVS",
				PowerVS: &configv1.PowerVSPlatformSpec{
					ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
						{
							Name: "Power",
							URL:  "https://dal.power-iaas.test.cloud.ibm.com/abc",
						},
					},
				},
			},
		},
	}
}

func expectedInfraObject() *configv1.Infrastructure {
	return &configv1.Infrastructure{
		Status: configv1.InfrastructureStatus{
			CPUPartitioning:        configv1.CPUPartitioningNone,
			ControlPlaneTopology:   "HighlyAvailable",
			InfrastructureTopology: "HighlyAvailable",
			PlatformStatus: &configv1.PlatformStatus{
				PowerVS: &configv1.PowerVSPlatformStatus{
					ServiceEndpoints: []configv1.PowerVSServiceEndpoint{
						{
							Name: "IAM",
							URL:  "https://iam.test.cloud.ibm.com",
						},
						{
							Name: "Power",
							URL:  "https://dal.power-iaas.test.cloud.ibm.com",
						},
					},
					ResourceGroup: "Default",
				},
			},
		},
	}
}
