package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Config struct {
	Protocol string
	URL      string
	Port     string
}

var config Config
var client http.Client

func forwardCallToNodeos(w http.ResponseWriter, r *http.Request) {
	log.Println("forward calls to nodeos")

	nodeosHost := fmt.Sprintf("%s://%s:%s", config.Protocol, config.URL, config.Port)
	url := nodeosHost + r.URL.String()
	method := r.Method
	body, _ := ioutil.ReadAll(r.Body)

	request, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	if err != nil {
		log.Printf("Error in creating request %s", err)
		return
	}

	res, err := client.Do(request)

	if err != nil {
		log.Printf("Error in executing request %s", err)
		return
	}

	body, _ = ioutil.ReadAll(res.Body)
	log.Printf("Nodeos response: %s - %s", res.Status, body)
	w.Write(body)
}

func main() {
	// TODO: make configurable from file/env
	config = Config{
		Protocol: "http",
		URL:      "localhost",
		Port:     "8888",
	}

	client = http.Client{}

	log.Println("Proxying and filtering nodeos requests...")
	http.HandleFunc("/", forwardCallToNodeos)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
