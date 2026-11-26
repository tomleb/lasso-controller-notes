package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/kubeconfig"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func main() {
	if err := mainErr(); err != nil {
		slog.Error("Exiting", "err", err)
		os.Exit(1)
	}
}

const (
	crtNamespace = "default"
	crtName      = "my-token"
	clusterName  = "my-cluster"

	// How long we wait before creating the Cluster that the
	// ClusterRegistrationToken depends on.
	clusterCreationDelay = 12 * time.Second
	// How long we keep running after the Cluster was created.
	runAfterCreation = 15 * time.Second

	crtByClusterIndex = "crtByCluster"
)

// This example shows how to handle a dependency on another object that may not
// exist (yet).
//
// We have two types, loosely modelled after Rancher's:
//   - Cluster (cluster-scoped)
//   - ClusterRegistrationToken (CRT, namespaced), which points to a Cluster
//     with .spec.clusterName
//
// The CRT controller needs the Cluster to exist to do its job (here: fill in
// .status.token). The CRT can be created before its Cluster: the order in
// which objects are created in k8s is not guaranteed, and even if it was,
// the CRT and Cluster caches are fed by different watches, so the CRT
// controller can see the CRT before the Cluster cache has the Cluster.
//
// The example creates the CRT first, starts the controller, then creates the
// Cluster 12 seconds later. Run it with either:
//
//	go run . -mode=bad
//	go run . -mode=good
//
// and compare the logs.
func mainErr() error {
	mode := flag.String("mode", "bad", "which CRT handler to run: bad or good")
	flag.Parse()
	if *mode != "bad" && *mode != "good" {
		return fmt.Errorf("unknown mode %q, must be bad or good", *mode)
	}

	ctx := context.Background()
	start := time.Now()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Show time elapsed since start, easier to read than timestamps.
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.String("t", time.Since(start).Truncate(time.Millisecond).String())
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	scheme := runtime.NewScheme()
	if err := addToScheme(scheme); err != nil {
		return err
	}

	clientConfig := kubeconfig.GetNonInteractiveClientConfig(os.Getenv("KUBECONFIG"))
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}

	if err := createCRDs(ctx, restConfig); err != nil {
		return fmt.Errorf("creating CRDs: %w", err)
	}

	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(restConfig, scheme, &controller.SharedControllerFactoryOptions{})
	if err != nil {
		return err
	}

	// These are the same generic controllers that wrangler's codegen
	// (pkg/generated/controllers/...) wraps. We use them directly to avoid
	// having to run codegen for this example.
	crts := generic.NewController[*ClusterRegistrationToken, *ClusterRegistrationTokenList](ClusterRegistrationTokenGVK, clusterRegistrationTokenResource, true, controllerFactory)
	clusters := generic.NewNonNamespacedController[*Cluster, *ClusterList](ClusterGVK, clusterResource, controllerFactory)

	if err := resetObjects(crts, clusters); err != nil {
		return err
	}

	var handlerRuns atomic.Int64
	switch *mode {
	case "bad":
		registerBad(ctx, crts, clusters, &handlerRuns)
	case "good":
		registerGood(ctx, crts, clusters, &handlerRuns)
	}

	// Create the CRT *before* the Cluster exists.
	_, err = crts.Create(&ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Name: crtName, Namespace: crtNamespace},
		Spec:       ClusterRegistrationTokenSpec{ClusterName: clusterName},
	})
	if err != nil {
		return fmt.Errorf("creating CRT: %w", err)
	}
	slog.Info("Created CRT", "crt", crtNamespace+"/"+crtName, "clusterName", clusterName)

	if err := controllerFactory.Start(ctx, 1); err != nil {
		return err
	}
	slog.Info("Started controller factory", "mode", *mode)

	time.Sleep(clusterCreationDelay)

	_, err = clusters.Create(&Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}})
	if err != nil {
		return fmt.Errorf("creating Cluster: %w", err)
	}
	clusterCreatedAt := time.Since(start)
	slog.Info("Created Cluster", "cluster", clusterName)

	// Wait to see how long it takes for the CRT to be reconciled.
	deadline := time.Now().Add(runAfterCreation)
	for time.Now().Before(deadline) {
		crt, err := crts.Cache().Get(crtNamespace, crtName)
		if err == nil && crt.Status.Token != "" {
			slog.Info("CRT is ready",
				"token", crt.Status.Token,
				"latency_after_cluster_created", (time.Since(start) - clusterCreatedAt).Truncate(time.Millisecond).String(),
				"handler_runs", handlerRuns.Load(),
			)
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	slog.Warn("CRT still not ready", "waited_after_cluster_created", runAfterCreation.String(), "handler_runs", handlerRuns.Load())
	return nil
}

// The bad way.
//
// The handler fetches the Cluster from the cache. When the Cluster doesn't
// exist, the cache returns a NotFound error, which the handler returns as-is.
//
// Returning an error tells lasso to re-enqueue the key with the rate limiter
// (see the errors example). Lasso's default rate limiter is
// exponential: 5ms, 10ms, 20ms, ..., up to 30s per item, then 2 minutes after
// 30 failures.
//
// When running this example, you should see something like:
//
//	ERROR: error syncing 'default/my-token': handler crt-handler: clusters.example.com "my-cluster" not found, requeuing
//
// repeated ~12 times in the 12 seconds before the Cluster is created, with
// the delay between each doubling. Then, once the Cluster is created,
// NOTHING happens until the next retry fires, ~8 seconds later in this
// example (the retries are at ~5s, ~10s, ~20s, ~41s, ...). The delay keeps
// doubling up to 30s, then 2 minutes after 30 failures, so the longer the
// Cluster takes to appear, the worse it gets. The Cluster appearing is not
// what triggers the CRT, the timer is.
//
// Problems with this approach:
//   - Logs are spammed with errors for something that is an expected,
//     transient state, not a failure. This hides actual errors.
//   - The handler (and every OTHER handler registered for CRTs, since they
//     share the key in the workqueue) runs again and again for nothing.
//   - Latency: the CRT is reconciled whenever the backoff expires, not when
//     the Cluster appears. The longer the Cluster takes to appear, the
//     longer the delay.
//   - If the Cluster never appears (eg: the CRT is orphaned), we keep
//     retrying, every 2 minutes, forever.
func registerBad(ctx context.Context, crts *generic.Controller[*ClusterRegistrationToken, *ClusterRegistrationTokenList], clusters *generic.NonNamespacedController[*Cluster, *ClusterList], runs *atomic.Int64) {
	// Side note: informers are created lazily, the first time .Informer() or
	// .Cache() is called, and only informers that exist when the factory is
	// started are started. If the first .Cache() call happened inside the
	// handler (ie: after Start), the Cluster cache would never be started
	// and would stay empty forever. We create it here, before Start. In the
	// good way, relatedresource.Watch does it for us.
	clusters.Informer()

	crts.OnChange(ctx, "crt-handler", func(key string, crt *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
		if crt == nil || crt.Status.Token != "" {
			return crt, nil
		}
		runs.Add(1)
		slog.Info("CRT handler running", "key", key)

		cluster, err := clusters.Cache().Get(crt.Spec.ClusterName)
		if err != nil {
			// BAD: NotFound is expected here, it is not a failure.
			return crt, err
		}

		return setToken(crts, crt, cluster)
	})
}

// The good way.
//
// Same handler, except a NotFound Cluster is not an error: we just return
// and wait. Nothing will re-enqueue the CRT on its own, so we also register a
// relatedresource watch: whenever a Cluster changes (created, updated,
// deleted), the resolver returns the keys of the CRTs that point to that
// Cluster and they are enqueued in the CRT controller's workqueue.
//
// When running this example, you should see the CRT handler run exactly once
// before the Cluster is created (no errors), then once more right after the
// Cluster is created, with a latency in the milliseconds.
func registerGood(ctx context.Context, crts *generic.Controller[*ClusterRegistrationToken, *ClusterRegistrationTokenList], clusters *generic.NonNamespacedController[*Cluster, *ClusterList], runs *atomic.Int64) {
	// An indexer lets the resolver find the CRTs for a given Cluster without
	// listing all CRTs. Indexers must be added before the cache is started.
	crts.Cache().AddIndexer(crtByClusterIndex, func(crt *ClusterRegistrationToken) ([]string, error) {
		return []string{crt.Spec.ClusterName}, nil
	})

	crts.OnChange(ctx, "crt-handler", func(key string, crt *ClusterRegistrationToken) (*ClusterRegistrationToken, error) {
		if crt == nil || crt.Status.Token != "" {
			return crt, nil
		}
		runs.Add(1)
		slog.Info("CRT handler running", "key", key)

		cluster, err := clusters.Cache().Get(crt.Spec.ClusterName)
		if apierrors.IsNotFound(err) {
			// GOOD: The Cluster doesn't exist yet. This is not an error,
			// the relatedresource watch below will enqueue this CRT once
			// the Cluster appears.
			slog.Info("Cluster not found, waiting for it to appear", "key", key, "cluster", crt.Spec.ClusterName)
			return crt, nil
		}
		if err != nil {
			// Any other error is still a real error.
			return crt, err
		}

		return setToken(crts, crt, cluster)
	})

	// relatedresource.Watch registers a handler on the Cluster controller
	// (the last argument(s)). For every Cluster key, it calls the resolver
	// and enqueues the returned keys into the CRT controller (the enqueuer,
	// crts here).
	//
	// Note that the Cluster may be nil (eg: deleted), so we resolve using
	// the name only.
	relatedresource.Watch(ctx, "crt-watch-cluster", func(_, name string, _ runtime.Object) ([]relatedresource.Key, error) {
		crtsForCluster, err := crts.Cache().GetByIndex(crtByClusterIndex, name)
		if err != nil {
			return nil, err
		}
		keys := make([]relatedresource.Key, 0, len(crtsForCluster))
		for _, crt := range crtsForCluster {
			slog.Info("Cluster changed, enqueuing CRT", "cluster", name, "crt", crt.Namespace+"/"+crt.Name)
			keys = append(keys, relatedresource.NewKey(crt.Namespace, crt.Name))
		}
		return keys, nil
	}, crts, clusters)
}

func setToken(crts *generic.Controller[*ClusterRegistrationToken, *ClusterRegistrationTokenList], crt *ClusterRegistrationToken, cluster *Cluster) (*ClusterRegistrationToken, error) {
	crt = crt.DeepCopy()
	crt.Status.Token = "token-for-" + cluster.Name
	slog.Info("Cluster found, setting token", "crt", crt.Namespace+"/"+crt.Name, "cluster", cluster.Name)
	return crts.UpdateStatus(crt)
}

func createCRDs(ctx context.Context, restConfig *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(restConfig)
	if err != nil {
		return err
	}
	clusterCRD := crd.FromGV(SchemeGroupVersion, ClusterGVK.Kind)
	clusterCRD.PluralName = clusterResource
	clusterCRD.NonNamespace = true

	crtCRD := crd.FromGV(SchemeGroupVersion, ClusterRegistrationTokenGVK.Kind).WithStatus()
	crtCRD.PluralName = clusterRegistrationTokenResource

	_, err = factory.CreateCRDs(ctx, clusterCRD, crtCRD)
	return err
}

// resetObjects deletes the objects from a previous run, so that the example
// can be run multiple times.
func resetObjects(crts *generic.Controller[*ClusterRegistrationToken, *ClusterRegistrationTokenList], clusters *generic.NonNamespacedController[*Cluster, *ClusterList]) error {
	if err := crts.Delete(crtNamespace, crtName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err := clusters.Delete(clusterName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	// Deletion is asynchronous, wait until both are gone.
	for range 100 {
		_, crtErr := crts.Get(crtNamespace, crtName, metav1.GetOptions{})
		_, clusterErr := clusters.Get(clusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(crtErr) && apierrors.IsNotFound(clusterErr) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timed out waiting for objects from previous run to be deleted")
}
