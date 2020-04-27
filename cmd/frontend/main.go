package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/bvtujo/bowie/m/v2/pkg/database"
	"github.com/bvtujo/bowie/m/v2/pkg/s3"
	"github.com/bvtujo/bowie/m/v2/pkg/web"
)

const (
	formFileKey = "myFile"
)

const (
	errNoDogSpecified = "no dog specified :("
	errBadFile        = "bad file o_O"
)

var s3Bucket = os.Getenv("BUCKET_NAME")
var dynamoTable = os.Getenv("MY_TABLE_NAME")

// Index returns the homepage, or all dog gifs.
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	t, err := web.LoadTemplate("cmd/frontend/index.html")
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot read template: %w", err) {
		return
	}
	p := web.PageData{
		Stylesheet: "",
	}
	d := web.IndexData{
		PageData: p,
		Items:    nil,
	}
	err = t.Execute(w, d)
	if checkHTTPErrorf(w, http.StatusInternalServerError, "execute index: %w", err) {
		return
	}
	return
}

func checkHTTPErrorf(w http.ResponseWriter, code int, message string, e error) bool {
	if !strings.Contains(message, `%w`) {
		panic("bad error message")
	}
	if e != nil {
		err := fmt.Sprintf(message, e)
		log.Error(err)
		http.Error(w, err, code)
		return true
	}
	return false
}

// Add adds a new gif to the specified dog's feed.
func AddPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	name := ps.ByName("dogName")
	if name == "" {
		http.Error(w, errNoDogSpecified, http.StatusBadRequest)
		return
	}

	p := web.PageData{
		Stylesheet: "",
	}
	d := web.AddData{
		PageData: p,
		DogName:  name,
	}
	t, err := web.LoadTemplate("cmd/frontend/add.go.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	t.Execute(w, d)

	return
}

func AddNewDogPic(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	err := r.ParseForm()
	if err != nil {
		log.Warn("error parsing form: %w", err)
		http.Error(w, errBadFile, http.StatusBadRequest)
		return
	}
	file, handler, err := r.FormFile(formFileKey)
	if err != nil {
		log.Warn("error getting file: %w", err)
		return
	}
	defer file.Close()
	log.Infof("Uploaded File: %+v\n", handler.Filename)
	log.Infof("File Size: %+v\n", handler.Size)
	log.Infof("MIME Header: %+v\n", handler.Header)

	sess, err := session.NewSession()
	if err != nil {
		log.Warn("error create session: %w", err)
		http.Error(w, "can't create s3 client", http.StatusInternalServerError)
	}
	s3client := s3.NewS3Uploader(s3Bucket, sess)
	s3Key := fmt.Sprintf("%s/%d.gif", ps.ByName("dogName"), time.Now().Unix())
	res, err := s3client.Upload(file, s3Key)
	if err != nil {
		log.Warnf("error upload file to s3: %w", err)
		http.Error(w, "can't upload file to s3", http.StatusInternalServerError)
		return
	}
	url := res.Location
	dogpic := database.DogPic{
		Dog:       aws.String(ps.ByName("dogName")),
		Key:       aws.String(s3Key),
		Timestamp: getTimestamp(),
		Tags:      parseTags(r.FormValue("tags")),
		URL:       aws.String(url),
	}
	dogsvc := database.NewDogService(sess, dynamoTable)
	_, err = dogsvc.Add(dogpic)
	if err != nil {
		log.Warnf("error add pic to ddb: %w", err)
		http.Error(w, "can't add file to ddb", http.StatusInternalServerError)
		return
	}
	rdURL := fmt.Sprintf("/dogs/%s", ps.ByName("dogName"))
	http.Redirect(w, r, rdURL, http.StatusSeeOther)
}

func ShowDog(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	fmt.Fprintf(w, "showing %s", ps.ByName("dogName"))
}

func getTimestamp() int64 {
	t := time.Now()
	return t.Unix()
}

func parseTags(t string) []*string {
	// TODO
	tags := strings.Split(t, ",")
	var out []*string
	for _, t := range tags {
		out = append(out, &t)
	}
	return out
}

func Healthcheck(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	return
}

func main() {
	r := httprouter.New()

	r.GET("/", Index)
	r.GET("/add/:dogName", AddPage)
	r.POST("/add/:dogName", AddNewDogPic)
	r.GET("/dogs/:dogName", ShowDog)

	r.GET("/healthcheck", Healthcheck)

	log.Fatal(http.ListenAndServe(":80", r))

}
