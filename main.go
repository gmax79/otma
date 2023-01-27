package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type Result struct {
	Status string `json:"status"`
}

func root(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Microservice Architecture course example"))
}

func health(w http.ResponseWriter, r *http.Request) {
	var result Result
	result.Status = "OK"
	content, _ := json.Marshal(result)
	w.Write(content)
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		fmt.Println("Listen 8000")
		http.HandleFunc("/", root)
		http.HandleFunc("/health", health)
		err := http.ListenAndServe(":8000", nil)
		if err != nil {
			fmt.Println(err)
			close(sigChan)
			return
		}
	}()

	_, ok := <-sigChan
	if ok {
		close(sigChan)
	}

	fmt.Println("Stopped")
}
