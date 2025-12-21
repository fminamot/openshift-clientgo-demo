package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"

	"k8s.io/apimachinery/pkg/labels"
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
	var err error

	stopCh := make(chan struct{})

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	log.Println("Creating informer from informer factory")
	factory := projectinformers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Project().V1().Projects().Informer()

	log.Println("Starting informers")
	go factory.Start(stopCh)

	defer func() {
		close(stopCh)
		factory.Shutdown()
		log.Println("Informer was stopped")
	}()

	log.Printf("Waiting for cache synced")
	if !cache.WaitForCacheSync(stopCh, informer.HasSynced) {
		log.Println("Failed to sync cache")
	}

	log.Printf("Listing all projects from lister")
	lister := factory.Project().V1().Projects().Lister()

	list, _ := lister.List(labels.Everything())
	for _, p := range list {
		fmt.Println(p.Name)
	}

	log.Println("Done")
}
