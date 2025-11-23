package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
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
	fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
}

func pointerInt64(i int64) *int64 {
	return &i
}

func main() {
	const projectName = "myproject"
	const displayName = "MyProject"
	const description = "This is a description of myproject"

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	ctx := context.Background()
	pr := createProjectRequest(projectName, displayName, description)

	fmt.Println("Watching project event")
	w, err := clientset.ProjectV1().Projects().Watch(ctx, metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("metadata.name=%s", projectName),
		TimeoutSeconds: pointerInt64(20), // 20 sec
	})
	if err != nil {
		panic(err)
	}

	defer w.Stop()

	fmt.Println("Creating project")
	p, err := clientset.ProjectV1().ProjectRequests().Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating project: %v", err)
	}

	for event := range w.ResultChan() {
		proj, ok := event.Object.(*projectv1.Project)
		if !ok {
			continue
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			if proj.Name == p.Name && proj.Status.Phase == corev1.NamespaceActive {
				fmt.Printf("Project %s is created (phase: %s)\n", proj.Name, proj.Status.Phase)
				return
			}
		case watch.Deleted:
			fmt.Printf("%s is deleted unexpectedly\n", proj.Name)
		}
	}

	fmt.Println("Watch ended or timed out")
}
