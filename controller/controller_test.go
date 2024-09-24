//go:build unit

package controller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	ctrlruntime "sigs.k8s.io/controller-runtime"
	ctrlruntimereconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kuadrant/policy-machinery/machinery"
)

func TestControllerOptions(t *testing.T) {
	opts := &ControllerOptions{
		name:      "controller",
		logger:    logr.Discard(),
		runnables: map[string]RunnableBuilder{},
		reconcile: func(context.Context, []ResourceEvent, *machinery.Topology, error, *sync.Map) error {
			return nil
		},
	}

	WithName("test")(opts)
	if opts.name != "test" {
		t.Errorf("expected name test, got %s", opts.name)
	}

	WithLogger(testLogger)(opts)
	if opts.logger != testLogger {
		t.Errorf("expected logger %v, got %v", testLogger, opts.logger)
	}

	WithClient(testClient)(opts)
	if opts.client != testClient {
		t.Errorf("expected client %v, got %v", testClient, opts.client)
	}

	WithRunnable("service watcher", testServiceWatcher)(opts)
	if len(opts.runnables) != 1 {
		t.Errorf("expected 1 runnable, got %d", len(opts.runnables))
	}
	if opts.runnables["service watcher"] == nil {
		t.Errorf("expected service watcher runnable, got nil")
	}

	WithRunnable("configmap watcher", testConfigMapWatcher)(opts)
	if len(opts.runnables) != 2 {
		t.Errorf("expected 2 runnables, got %d", len(opts.runnables))
	}
	if opts.runnables["service watcher"] == nil {
		t.Errorf("expected service watcher runnable, got nil")
	}
	if opts.runnables["configmap watcher"] == nil {
		t.Errorf("expected configmap watcher runnable, got nil")
	}

	WithPolicyKinds(testPolicyKinds...)(opts)
	if len(opts.policyKinds) != len(testPolicyKinds) {
		t.Errorf("expected %d policy kinds, got %d", len(testPolicyKinds), len(opts.policyKinds))
	}
	if !lo.Every(opts.policyKinds, testPolicyKinds) {
		t.Errorf("expected policy kinds %v, got %v", testPolicyKinds, opts.policyKinds)
	}

	WithObjectKinds(testObjctKinds...)(opts)
	if len(opts.objectKinds) != len(testObjctKinds) {
		t.Errorf("expected %d object kinds, got %d", len(testObjctKinds), len(opts.objectKinds))
	}
	if !lo.Every(opts.objectKinds, testObjctKinds) {
		t.Errorf("expected object kinds %v, got %v", testObjctKinds, opts.objectKinds)
	}

	WithObjectLinks(testLinkFunc)(opts)
	if len(opts.objectLinks) != 1 {
		t.Errorf("expected 1 object link, got %d", len(opts.objectLinks))
	}

	ManagedBy(testManager)(opts)
	if opts.manager != testManager {
		t.Errorf("expected manager %v, got %v", testManager, opts.manager)
	}

	WithReconcile(testReconcileFunc)(opts)
	if opts.reconcile == nil {
		t.Errorf("expected reconcile func, got nil")
	}

	AllowLoops()(opts)
	if opts.allowTopologyLoops == false {
		t.Errorf("expected allowTopologyLoops true, got false")
	}
}

func TestNewController(t *testing.T) {
	type expected struct {
		name          string
		logger        logr.Logger
		client        *dynamic.DynamicClient
		manager       ctrlruntime.Manager
		policyKinds   []schema.GroupKind
		objectKinds   []schema.GroupKind
		objectLinks   []LinkFunc
		runnableNames []string
	}

	testCases := []struct {
		name     string
		options  []ControllerOption
		expected expected
	}{
		{
			name: "defaults",
			expected: expected{
				name:          "controller",
				logger:        logr.Discard(),
				client:        nil,
				manager:       nil,
				policyKinds:   []schema.GroupKind{},
				objectKinds:   []schema.GroupKind{},
				objectLinks:   []LinkFunc{},
				runnableNames: []string{},
			},
		},
		{
			name: "with options",
			options: []ControllerOption{
				WithName("test"),
				WithLogger(testLogger),
				WithClient(testClient),
				WithRunnable("service watcher", testServiceWatcher),
				WithRunnable("configmap watcher", testConfigMapWatcher),
				WithPolicyKinds(testPolicyKinds...),
				WithObjectKinds(testObjctKinds...),
				WithObjectLinks(testLinkFunc),
				ManagedBy(testManager),
			},
			expected: expected{
				name:          "test",
				logger:        testLogger,
				client:        testClient,
				manager:       testManager,
				policyKinds:   testPolicyKinds,
				objectKinds:   testObjctKinds,
				objectLinks:   []LinkFunc{testLinkFunc},
				runnableNames: []string{"service watcher", "configmap watcher"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewController(tc.options...)
			if c.name != tc.expected.name {
				t.Errorf("expected name %s, got %s", tc.expected.name, c.name)
			}
			if c.logger != tc.expected.logger {
				t.Errorf("expected logger %v, got %v", tc.expected.logger, c.logger)
			}
			if c.client != tc.expected.client {
				t.Errorf("expected client %v, got %v", tc.expected.client, c.client)
			}
			if c.manager != tc.expected.manager {
				t.Errorf("expected manager %v, got %v", tc.expected.manager, c.manager)
			}
			switch c.cache.(type) {
			case *watchableCacheStore:
			default:
				t.Errorf("expected cache type *watchableCacheStore, got %T", c.cache)
			}
			if len(c.topology.policyKinds) != len(tc.expected.policyKinds) || !lo.Every(c.topology.policyKinds, tc.expected.policyKinds) {
				t.Errorf("expected policyKinds %v, got %v", tc.expected.policyKinds, c.topology.policyKinds)
			}
			if len(c.topology.objectKinds) != len(tc.expected.objectKinds) || !lo.Every(c.topology.objectKinds, tc.expected.objectKinds) {
				t.Errorf("expected objectKinds %v, got %v", tc.expected.objectKinds, c.topology.objectKinds)
			}
			if len(c.topology.objectLinks) != len(tc.expected.objectLinks) {
				t.Errorf("expected %d objectLinks, got %d", len(tc.expected.objectLinks), len(c.topology.objectLinks))
			}
			if len(c.runnables) != len(tc.expected.runnableNames) || !lo.Every(lo.Keys(c.runnables), tc.expected.runnableNames) {
				t.Errorf("expected objectKinds %v, got %v", tc.expected.objectKinds, c.topology.objectKinds)
			}
		})
	}
}

func TestControllerReconcile(t *testing.T) {
	objs := []Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-service", UID: "7ed703a2-635d-4002-a825-5624823760a5"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-configmap", UID: "aed148b1-285a-48ab-8839-fe99475bc6fc"}},
	}
	objUIDs := lo.Map(objs, func(o Object, _ int) string { return string(o.GetUID()) })
	cache := &cacheStore{store: make(Store)}
	controller := &Controller{
		logger: testLogger,
		cache:  cache,
		listFuncs: []ListFunc{
			func() []Object { return objs },
		},
	}
	controller.Reconcile(context.TODO(), ctrlruntimereconcile.Request{})
	cachedObjs := lo.Keys(cache.List())
	if len(cachedObjs) != 2 {
		t.Errorf("expected 2 objects, got %d", len(cachedObjs))
	}
	if !lo.Every(cachedObjs, objUIDs) {
		t.Errorf("expected %v object UIDs in the cache, got %v", objUIDs, cachedObjs)
	}
}

func TestStartControllerUnmanaged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := NewController()
	go func() {
		if err := c.Start(ctx); err != nil {
			t.Errorf("expected no error when starting manager, got %s", err.Error())
			cancel()
		}
	}()
	time.Sleep(3 * time.Second)
}
