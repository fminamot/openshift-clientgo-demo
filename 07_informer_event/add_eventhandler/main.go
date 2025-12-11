package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
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

	defer func() {
		log.Println("Defer func called")
		cancel()
		factory.Shutdown()
		log.Println("Informer was stopped")
	}()

	projEvent := make(chan struct{})

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			p := obj.(*projectv1.Project)
			log.Printf("[Event] %s is added, => %s\n", p.Name, p.Status.Phase)
			projEvent <- struct{}{}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			p := newObj.(*projectv1.Project)
			log.Printf("[Event] %s is modified, => %s\n", p.Name, p.Status.Phase)
			projEvent <- struct{}{}
		},
		DeleteFunc: func(obj interface{}) {
			p := obj.(*projectv1.Project)
			log.Printf("[Event] %s is deleted\n", p.Name)
			projEvent <- struct{}{}
		},
	})

	log.Println("Starting informers")
	go factory.Start(ctx.Done())

	log.Printf("Waiting for cache synced")
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		log.Println("Failed to sync cache")
	}

	log.Println("Timeout in 1 min. Ctrl-C will stop this program")
	for {
		select {
		case <-time.After(1 * time.Minute):
			log.Println("Timeout")
			return
		case <-ctx.Done():
			log.Println("Application will shut down")
			return
		case <-projEvent:
			continue
		}
	}
}
