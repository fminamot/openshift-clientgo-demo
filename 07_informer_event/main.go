package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createProjectRequest(name string, displayName string, description string) *projectv1.ProjectRequest {
	projectRequest := projectv1.ProjectRequest{
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		DisplayName: displayName,
		Description: description,
	}
	return &projectRequest
}

func printProject(p *projectv1.Project) {
	displayName := p.Annotations["openshift.io/display-name"]
	description := p.Annotations["openshift.io/description"]
	fmt.Printf("NAME\tDISPLAY NAME\tDESCRIPTION\n")
	fmt.Println("----------------------------------------------")
	fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
}

func main() {
	const (
		projectName = "myproject"
		displayName = "MyProject"
		description = "This is a description of myproject"
	)

	// Wrap the signal context in a cancel context
	signalCtx := signals.SetupSignalHandler()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel() // to stop informer

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	fmt.Println("Creating informer from informer factory")
	factory := projectinformers.NewSharedInformerFactory(clientset, 0)
	informer := factory.Project().V1().Projects().Informer()

	done := make(chan string) // project name

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			proj := obj.(*projectv1.Project)
			fmt.Printf("[Event] %s is added\n", proj.Name)

			if proj.Name == projectName &&
				proj.Status.Phase == corev1.NamespaceActive {
				done <- proj.Name // sending project name to done channel
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldP := oldObj.(*projectv1.Project)
			newP := newObj.(*projectv1.Project)
			fmt.Printf("[Event] %s is modified\n", newP.Name)

			if newP.Name != projectName {
				return
			}

			if oldP.Status.Phase != newP.Status.Phase &&
				newP.Status.Phase == corev1.NamespaceActive {
				done <- newP.Name // sending project name to done channel
			}
		},
	})

	fmt.Println("Starting informers")
	go factory.Start(ctx.Done()) // arg is stop channel

	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	fmt.Println("Cache is synced")

	fmt.Println("Creating project")
	pr := createProjectRequest(projectName, displayName, description)
	newProject, err := clientset.ProjectV1().ProjectRequests().Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating project: %v", err)
	}
	fmt.Printf("%s created\n", newProject.Name)

	name := <-done // wait for done from event handlers

	lister := factory.Project().V1().Projects().Lister()
	p, err := lister.Get(name) // get project from informer cache
	if err != nil {
		log.Fatalf("Error getting project %v", err)
	}
	printProject(p)
}
