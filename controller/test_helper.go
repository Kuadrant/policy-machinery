//go:build unit || integration

package controller

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

	ctrlruntimemanager "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kuadrant/policy-machinery/machinery"
)

var (
	testLogger        logr.Logger
	testClient        *dynamic.DynamicClient
	testPolicyKinds   []schema.GroupKind
	testObjctKinds    []schema.GroupKind
	testLinkFunc      LinkFunc
	testScheme        *runtime.Scheme
	testManager       ctrlruntimemanager.Manager
	testReconcileFunc ReconcileFunc

	testServiceWatcher   RunnableBuilder
	testConfigMapWatcher RunnableBuilder
)

func init() {
	testLogger = CreateAndSetLogger()
	testClient = dynamic.New(&fake.RESTClient{})
	testPolicyKinds = []schema.GroupKind{
		{Group: "test/v1", Kind: "FooPolicy"},
		{Group: "test/v1", Kind: "BarPolicy"},
	}
	testObjctKinds = []schema.GroupKind{
		{Group: "test/v1", Kind: "MyObject"},
	}
	testLinkFunc = func(objs Store) machinery.LinkFunc {
		myObjects := objs.FilterByGroupKind(schema.GroupKind{Group: "test/v1", Kind: "MyObject"})
		return machinery.LinkFunc{
			From: schema.GroupKind{Group: "test/v1", Kind: "MyObject"},
			To:   machinery.GatewayGroupKind,
			Func: func(_ machinery.Object) []machinery.Object { return []machinery.Object{&RuntimeObject{myObjects[0]}} },
		}
	}
	testReconcileFunc = func(_ context.Context, events []ResourceEvent, topology *machinery.Topology, _ *sync.Map, err error) {
		for _, event := range events {
			testLogger.Info("reconcile",
				"kind", event.Kind,
				"event", event.EventType.String(),
				"targetables", len(topology.Targetables().Items()),
				"policies", len(topology.Policies().Items()),
				"objects", len(topology.Objects().Items()),
			)
		}
	}
	testScheme = runtime.NewScheme()
	corev1.AddToScheme(testScheme)
	var err error
	testManager, err = ctrlruntimemanager.New(&rest.Config{}, ctrlruntimemanager.Options{})
	if err != nil {
		panic(err)
	}
	testServiceWatcher = Watch(&corev1.Service{}, ServicesResource, "foo")
	testConfigMapWatcher = Watch(&corev1.ConfigMap{}, ConfigMapsResource, metav1.NamespaceAll, FilterResourcesByLabel[*corev1.ConfigMap]("app=foo"))
}
