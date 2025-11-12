package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func getProjectClienSet() (*projectclientset.Clientset, error) {
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

func main() {
	// 1. Getting OpenShift Project client-set
	clientset, err := getProjectClienSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	// 2. Creating ProjectRequest structure
	ctx := context.Background()
	pr := createProjectRequest("myproject", "MyProject", "This is a escription of myproject")

	// 3. Creating ProjectRequest
	p, err := clientset.ProjectV1().ProjectRequests().Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating project: %v", err)
	}

	fmt.Printf("%s created\n", p.Name)
}
