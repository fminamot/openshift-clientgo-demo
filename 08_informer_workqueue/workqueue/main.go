package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"
	projectlisters "github.com/openshift/client-go/project/listers/project/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
)

func getProjectClientSet() (*projectclientset.Clientset, error) {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := projectclientset.NewForConfig(config)

	return clientset, err
}

func printProject(workerIndex int, p *projectv1.Project) {
	dn := p.Annotations["openshift.io/display-name"]
	des := p.Annotations["openshift.io/description"]
	status := p.Status.Phase
	log.Printf("[worker:%d] name=%s, displayName=%s, description=%s, status=%s\n", workerIndex, p.Name, dn, des, status)
}

func enqueue(obj interface{}, queue workqueue.TypedRateLimitingInterface[string]) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Println("enqueue error")
		return
	}
	queue.Add(key)
}

func checkProject(workerIndex int, p *projectv1.Project) bool {
	printProject(workerIndex, p)
	return true
}

func processNextItem(workerIndex int, lister projectlisters.ProjectLister, queue workqueue.TypedRateLimitingInterface[string]) bool {
	key, shutdown := queue.Get()
	if shutdown {
		return false
	}

	defer queue.Done(key)

	p, err := lister.Get(key)
	if errors.IsNotFound(err) {
		log.Printf("%s not found in the cache\n", key)
		queue.Forget(key)
		return true
	}
	if err != nil {
		log.Printf("Error getting the project: %s: %v\n", key, err)
		queue.Forget(key)
		return false
	}

	if ok := checkProject(workerIndex, p); ok {
		queue.Forget(key)
	} else {
		queue.AddRateLimited(key)
	}
	return true
}

func worker(ctx context.Context, workerIndex int, lister projectlisters.ProjectLister, queue workqueue.TypedRateLimitingInterface[string]) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker %d done for cancellation\n", workerIndex)
			return
		default:
			if !processNextItem(workerIndex, lister, queue) {
				log.Printf("worker %d done for queue shutdown\n", workerIndex)
				return
			}
		}
	}
}

func main() {

	signalCtx := signals.SetupSignalHandler()
	ctx, cancel := context.WithCancel(signalCtx)

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	log.Println("Creating informer from informer factory")
	factory := projectinformers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Project().V1().Projects().Informer()
	lister := factory.Project().V1().Projects().Lister()

	queue := workqueue.NewTypedRateLimitingQueue[string](
		workqueue.DefaultTypedControllerRateLimiter[string](),
	)

	defer func() {
		log.Println("Defer func called")
		cancel()
		queue.ShutDown()
		factory.Shutdown()
		log.Println("Informer stopped")
	}()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueue(obj, queue)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			enqueue(newObj, queue)
		},
		DeleteFunc: func(obj interface{}) {
			enqueue(obj, queue)
		},
	})

	log.Println("Starting informers")
	go factory.Start(ctx.Done())

	log.Printf("Waiting for cache synced")
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Println("Failed to sync cache")
	}

	workers := 3
	log.Println("Ctrl-C will stop this program")
	for i := 0; i < workers; i++ {
		go worker(ctx, i, lister, queue)
	}

	<-ctx.Done()
	log.Println("Application will shut down")

}
