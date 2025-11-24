package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/watch"
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

func pointerInt64(i int64) *int64 {
	return &i
}

func printProject(p *projectv1.Project) {
	displayName := p.Annotations["openshift.io/display-name"]
	description := p.Annotations["openshift.io/description"]
	fmt.Printf("NAME\tDISPLAY NAME\tDESCRIPTION\n")
	fmt.Println("----------------------------------------------")
	fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
}

func stopProjectWatch(w watch.Interface) {
	fmt.Println("Stopping the project watch")
	w.Stop()
}

func main() {
	var err error

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Creating a project watch")
	w, err := clientset.ProjectV1().Projects().Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: pointerInt64(600), // 600 sec
	})
	if err != nil {
		panic(err)
	}

	defer stopProjectWatch(w)

	fmt.Println("Catching signals to close watch connection")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals
		fmt.Println("Signal caught")
		stopProjectWatch(w)
		os.Exit(0)
	}()

	fmt.Println("Waiting for project events")
	for event := range w.ResultChan() {
		proj, ok := event.Object.(*projectv1.Project)
		if !ok {
			continue
		}

		fmt.Println()

		switch event.Type {
		case watch.Added:
			fmt.Printf("[Event] %s is added\n", proj.Name)
		case watch.Modified:
			fmt.Printf("[Event] %s is modified\n", proj.Name)
		case watch.Deleted:
			fmt.Printf("[Event] %s is deleted\n", proj.Name)
		}
		printProject(proj)
	}
	fmt.Println("Timeout")
}
