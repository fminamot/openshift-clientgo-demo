package main

import (
	"fmt"
	"log"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	signalCtx := signals.SetupSignalHandler()

	// シャットダウン処理
	go func() {
		<-signalCtx.Done() // ここでシグナルを受信するまで待機
		log.Println("Shutting down for 3 sec")
		time.Sleep(3 * time.Second)
		log.Println("Second Ctrl-C will terminate this program")
	}()

	fmt.Println("Wait for 20 sec. Ctrl-C will start shutdown process")
	time.Sleep(20 * time.Second)
	fmt.Println("Done")
}
