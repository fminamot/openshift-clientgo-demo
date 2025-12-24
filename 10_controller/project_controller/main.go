package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	apiprojectv1 "github.com/openshift/api/project/v1"

	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"
	projectinformersv1 "github.com/openshift/client-go/project/informers/externalversions/project/v1"
	projectv1 "github.com/openshift/client-go/project/listers/project/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
)

type ProjectController struct {
	client       projectclientset.Interface
	projInformer cache.SharedIndexInformer
	projLister   projectv1.ProjectLister
	projSynched  cache.InformerSynced
	queue        workqueue.TypedRateLimitingInterface[string]
}

func NewProjectController(cl projectclientset.Interface, informer projectinformersv1.ProjectInformer) *ProjectController {
	controller := &ProjectController{
		client:       cl,
		projInformer: informer.Informer(),
		projLister:   informer.Lister(),
		projSynched:  informer.Informer().HasSynced,
		queue: workqueue.NewTypedRateLimitingQueue[string](
			workqueue.DefaultTypedControllerRateLimiter[string](),
		),
	}

	controller.projInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.projectAdded,
		UpdateFunc: controller.projectUpdated,
	})
	return controller
}

func (c *ProjectController) enqueueProject(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Println("enqueue error")
		return
	}
	c.queue.Add(key)
}

func (c *ProjectController) projectAdded(obj interface{}) {
	c.enqueueProject(obj)
}

func (c *ProjectController) projectUpdated(oldObj interface{}, newObj interface{}) {
	c.enqueueProject(newObj)
}

func printProject(p *apiprojectv1.Project) {
	dn := p.Annotations["openshift.io/display-name"]
	status := p.Status.Phase
	log.Printf("name=%s, displayName=%s, status=%s\n", p.Name, dn, status)
}

func (c *ProjectController) syncHandler(ctx context.Context, key string) bool {
	p, err := c.projLister.Get(key)

	if errors.IsNotFound(err) {
		log.Printf("%s not found in the cache\n", key)
		c.queue.Forget(key)
		return true
	}
	if err != nil {
		log.Printf("Error getting the project: %s: %v\n", key, err)
		c.queue.Forget(key)
		return false
	}

	// display-nameが空であれば、"<requester>'s <project>"という文字列を設定
	dn := p.Annotations["openshift.io/display-name"]

	if dn == "" {
		newObj := p.DeepCopy()
		req := p.Annotations["openshift.io/requester"]
		newObj.Annotations["openshift.io/display-name"] = req + "'s " + newObj.Name
		updated, _ := c.client.ProjectV1().Projects().Update(ctx, newObj, metav1.UpdateOptions{})
		printProject(updated)
		return true
	}

	printProject(p)
	return true
}

func (c *ProjectController) processNextItem(ctx context.Context) bool {
	key, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Done(key)

	if ok := c.syncHandler(ctx, key); ok {
		c.queue.Forget(key)
	} else {
		c.queue.AddRateLimited(key)
	}

	return true
}

func (c *ProjectController) runWorker(ctx context.Context) {
	for c.processNextItem(ctx) {
	}
}

func (c *ProjectController) Run(ctx context.Context, workers int) error {
	defer c.queue.ShutDown()

	if !cache.WaitForCacheSync(ctx.Done(), c.projSynched) {
		return fmt.Errorf("Failed to sync cache")
	}

	log.Println("Ctrl-C will stop this controller")
	for i := 0; i < workers; i++ {
		go c.runWorker(ctx)
	}

	<-ctx.Done()

	log.Println("Controller done")
	return nil
}

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

func main() {
	signalCtx := signals.SetupSignalHandler()
	ctx, cancel := context.WithCancel(signalCtx)

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	factory := projectinformers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Project().V1().Projects()
	controller := NewProjectController(clientset, informer)

	defer func() {
		cancel()
		factory.Shutdown()
	}()

	go factory.Start(ctx.Done())

	err = controller.Run(ctx, 1)
	if err != nil {
		log.Fatalf("Error running controller: %v", err)
	}
}
