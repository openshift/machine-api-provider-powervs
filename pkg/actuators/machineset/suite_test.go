package machineset

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func init() {
	klog.InitFlags(nil)
	logf.SetLogger(klogr.New())

	// Register required object kinds with global scheme.
	_ = machinev1beta1.AddToScheme(scheme.Scheme)
}

const (
	timeout = 20 * time.Second
)

var (
	cfg     *rest.Config
	testEnv *envtest.Environment
	ctx     = context.Background()
)

func TestMachinesetController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "MachineSet Controller Suite")
}

var _ = BeforeSuite(func() {
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "vendor", "github.com", "openshift", "api", "machine", "v1beta1"),
		},
	}
	machinev1beta1.AddToScheme(scheme.Scheme)

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager) context.CancelFunc {
	mgrCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer GinkgoRecover()

		Expect(mgr.Start(mgrCtx)).To(Succeed())
	}()
	return cancel
}
