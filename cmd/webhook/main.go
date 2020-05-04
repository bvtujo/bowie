package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

var webhookUrl = os.Getenv("CHIME_URL")
var ddosUrl = "https://ddos.dog"

type DDOSMessage struct {
	S3Url string `json:"s3url"`
}

func handler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log.Println("POST request received")
	msg := DDOSMessage{}
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&msg)
	if err != nil {
		log.Printf("error decoding request body: %s\n", err)
	}

	content := map[string]string{"Content": fmt.Sprintf("/md @Present A new photo of %s has been posted on [🐶DDOS](%s)! ![%s!!!](%s)", ps.ByName("name"), ddosUrl, ps.ByName("name"), msg.S3Url)}
	jsonContent, _ := json.Marshal(content)
	log.Printf("marshaled json content %s\n", jsonContent)
	resp, err := http.Post(webhookUrl, "application/json", bytes.NewBuffer(jsonContent))
	if err != nil {
		log.Printf("error sending message to chime room: %s\n", err)
		http.Error(w, err.Error(), resp.StatusCode)
		return
	}
	w.WriteHeader(http.StatusCreated)
	log.Printf("sent message announcing url %s to chime room\n", msg.S3Url)

}

func main() {
	r := httprouter.New()
	r.POST("/new/:name", handler)
	log.Fatal(http.ListenAndServe(":80", r))
}
