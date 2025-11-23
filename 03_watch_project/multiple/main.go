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

		fmt.Printf("%s created\n", newProject.Name)
	}
	return pnames, nil
}

func pointerInt64(i int64) *int64 {
	return &i
}

/*
func printProject(p *projectv1.Project) {
	displayName := p.Annotations["openshift.io/display-name"]
	description := p.Annotations["openshift.io/description"]
	fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
}
*/

func main() {
	const csvFile = "../projects.csv"
	var err error

	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Creating a project watch")
	w, err := clientset.ProjectV1().Projects().Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: pointerInt64(20), // 20 sec
	})
	if err != nil {
		panic(err)
	}

	defer w.Stop()

	fmt.Println("Creating projects")
	pnames, err := createProjectsFromCSV(clientset, csvFile)
	if err != nil {
		log.Fatalf("Error creating projects: %v", err)
	}

	fmt.Println("Waiting for project events")
	for event := range w.ResultChan() {
		proj, ok := event.Object.(*projectv1.Project)
		if !ok {
			continue
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			_, found := pnames[proj.Name]
			if found && proj.Status.Phase == corev1.NamespaceActive {
				fmt.Printf("%s is ready (phase: %s)\n", proj.Name, proj.Status.Phase)
				delete(pnames, proj.Name)
				if len(pnames) == 0 {
					return
				}
			}
		case watch.Deleted:
			fmt.Printf("%s is deleted unexpectedly\n", proj.Name)
		}
	}
	fmt.Println("Timeout")
}
