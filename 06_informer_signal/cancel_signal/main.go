package main

import (
	"context"
	"log"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {

	signalCtx := signals.SetupSignalHandler()    // 親コンテキスト
	ctx, cancel := context.WithCancel(signalCtx) // 子コンテキスト

	var shutdown sync.WaitGroup

	// プログラム終了処理
	defer func() {
		log.Println("Defer func was called")
		cancel()        // cancel関数呼び出しによってctx.Doneに通知が飛ぶ
		shutdown.Wait() // シャットダウンが終了するまで待つ
		log.Println("Defer func done")

	}()

	// アプリケーションの終了処理
	shutdown.Add(1)
	go func() {
		defer shutdown.Done()

		<-ctx.Done() // シグナルまたはcancel関数が実行されるまで待機する
		log.Println("Shutting down")
	}()

	log.Println("Wait for 10 sec")
	time.Sleep(10 * time.Second)
	log.Println("Done")
}
