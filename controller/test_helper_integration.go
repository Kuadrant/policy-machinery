//go:build integration

package controller

import (
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	ctrlruntimeenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrlruntimemanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	testConfig     *rest.Config
	testEnv        *ctrlruntimeenvtest.Environment
	testEnvStarted bool
)

// startTestEnv starts the test environment if it hasn't been started yet
// It requires a Kubernetes cluster running in the context
func startTestEnv() {
	if testEnvStarted {
		return
	}
	testEnv = &ctrlruntimeenvtest.Environment{
		Scheme:             testScheme,
		UseExistingCluster: ptr.To(true),
	}
	var err error
	testConfig, err = testEnv.Start()
	if err != nil {
		panic(err)
	}
	testEnvStarted = true
}

// buildStartableTestManager builds a startable manager for testing
func buildStartableTestManager() (ctrlruntimemanager.Manager, error) {
	startTestEnv() // lazy loading the environment and config

	return ctrlruntimemanager.New(testConfig, ctrlruntimemanager.Options{
		Logger: testLogger,
		Scheme: testScheme,
	})
}
