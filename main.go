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
	path := r.RequestURI
	w.Write([]byte(path))
}

func health(w http.ResponseWriter, r *http.Request) {
	var result Result
	result.Status = "OK"
	content, _ := json.Marshal(result)
	w.Write(content)
}

func readness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("true"))
}

func liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("true"))
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
		http.HandleFunc("/readness", readness)
		http.HandleFunc("/liveness", liveness)
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
