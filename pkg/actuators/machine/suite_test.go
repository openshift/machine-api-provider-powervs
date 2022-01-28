package machine

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	configv1 "github.com/openshift/api/config/v1"
)

const (
	timeout = 10 * time.Second
)

var (
	k8sClient     client.Client
	eventRecorder record.EventRecorder
)

func TestMain(m *testing.M) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crds"),
		},
	}

	configv1.AddToScheme(scheme.Scheme)

	cfg, err := testEnv.Start()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := testEnv.Stop(); err != nil {
			log.Fatal(err)
		}
	}()

	mgr, err := manager.New(cfg, manager.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	if err != nil {
		log.Fatal(err)
	}

	mgrCtx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := mgr.Start(mgrCtx); err != nil {
			log.Fatal(err)
		}
	}()
	defer cancel()

	k8sClient = mgr.GetClient()
	eventRecorder = mgr.GetEventRecorderFor("powervscontroller")

	code := m.Run()
	os.Exit(code)
}
