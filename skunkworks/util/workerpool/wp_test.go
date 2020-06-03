package workerpool

import (
	"encoding/json"
	"fmt"
	"github.com/finogeeks/ligase/model"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

type payloadCollection struct {
	WindowsVersion string          `json:"version"`
	Token          string          `json:"token"`
	Payloads       []model.Payload `json:"data"`
}

var wp *WorkerPool

func Test_WorkerPool(t *testing.T) {
	wp = NewWorkerPool(10)
	defer wp.Stop()
	wp.SetHandler(processSometime).Run()
	http.HandleFunc("/payload/", payloadHandler)

	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			fmt.Println("starting listening for payload messages")
		} else {
			fmt.Printf("an error occured while starting payload server %s", err.Error())
		}
	}()

	select {}
}

func processSometime(payload model.Payload) error {
	dur := time.Duration(rand.Intn(10)) * time.Millisecond
	time.Sleep(dur)
	return nil
}

func payloadHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read the body into a string for json decoding
	var content = &payloadCollection{}
	err := json.NewDecoder(io.LimitReader(r.Body, 2048)).Decode(&content)
	if err != nil {
		fmt.Println("an error occured while deserializing message")
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fmt.Println(content)

	// Go through each payload and queue items individually to be feeded
	for _, payload := range content.Payloads {
		wp.FeedPayload(payload)
		fmt.Println("for payload ")
	}
	fmt.Println("ok")

	w.WriteHeader(http.StatusOK)
}
