package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	projectinformers "github.com/openshift/client-go/project/informers/externalversions"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createProjectRequest(name string, displayName string, description string) *projectv1.ProjectRequest {
	projectRequest := projectv1.ProjectRequest{
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		DisplayName: displayName,
		Description: description,
	}
	return &projectRequest
}

func createProjectsFromCSV(clientset *projectclientset.Clientset, csvFile string) (map[string]int, error) {
	pnames := map[string]int{}
	counter := 0

	file, err := os.Open(csvFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	r := csv.NewReader(file)

	if _, err := r.Read(); err != nil {
		if err != io.EOF {
			return nil, err
		}
	}

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}

		ctx := context.Background()

		pr := createProjectRequest(row[0], row[1], row[2])

		newProject, err := clientset.ProjectV1().ProjectRequests().Create(ctx, pr, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}

		pnames[newProject.Name] = counter
		counter++

		log.Printf("%s created\n", newProject.Name)
	}
	return pnames, nil
}

func printProject(p *projectv1.Project) {
	displayName := p.Annotations["openshift.io/display-name"]
	description := p.Annotations["openshift.io/description"]
	fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
}

func main() {
	const csvFile = "../projects.csv"
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
