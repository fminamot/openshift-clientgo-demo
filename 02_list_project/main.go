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
	"time"

	projectv1 "github.com/openshift/api/project/v1"
	projectclientset "github.com/openshift/client-go/project/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func createProjectsFromCSV(clientset *projectclientset.Clientset, csvFile string) error {
	// Project情報を含むCSVファイルを開く
	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()
	r := csv.NewReader(file)

	// CSVヘッダ行をスキップ
	if _, err := r.Read(); err != nil {
		if err != io.EOF {
			return err
		}
	}

	// CSV一行ごとにProjectRequestを作成
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}

		ctx := context.Background()

		pr := createProjectRequest(row[0], row[1], row[2])

		newProject, err := clientset.ProjectV1().ProjectRequests().Create(ctx, pr, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		fmt.Printf("%s created\n", newProject.Name)
	}
	return nil
}

const csvFile = "projects.csv"

func main() {
	var err error

	// 1. Getting OpenShift Project client-set
	clientset, err := getProjectClientSet()
	if err != nil {
		log.Fatalf("Error creating project client: %v", err)
	}

	// 2. Creating projects from CSV file
	err = createProjectsFromCSV(clientset, csvFile)
	if err != nil {
		log.Fatalf("Error creating projects: %v", err)
	}

	time.Sleep(5 * time.Second)

	// 3 Getting Project list
	ctx := context.Background()
	projects, err := clientset.ProjectV1().Projects().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing projects %v", err)
	}

	// 4. Printing Project info
	fmt.Printf("NAME\tDISPLAY NAME\tDESCRIPTION\n")
	fmt.Println("----------------------------------------------")
	for _, p := range projects.Items {
		displayName := p.Annotations["openshift.io/display-name"]
		description := p.Annotations["openshift.io/description"]
		fmt.Printf("%s\t%s\t%s\n", p.Name, displayName, description)
	}
}
