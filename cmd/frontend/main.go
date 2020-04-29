package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
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

var publicDogPicBucket = os.Getenv("BUCKET_NAME")
var dynamoTable = os.Getenv("MY_TABLE_NAME")
var publicAssetsBucket = os.Getenv("ASSETS_BUCKET_NAME")

var mimeImageTypeRegex = regexp.MustCompile("image/.*")

// Index returns the homepage, or all dog gifs.
func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	t, err := web.LoadTemplate("cmd/frontend/index.html")
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot read template: %w", err) {
		return
	}
	sess, err := session.NewSession()
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot get dynamo connection: %w", err) {
		return
	}
	ddb := database.NewDogService(sess, dynamoTable)
	allPics, err := ddb.GetAll()
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot scan ddb: %w", err) {
		return
	}

	p := web.PageData{
		Stylesheet: fmt.Sprintf("https://%s.s3.amazonaws.com/main.css", publicAssetsBucket),
	}
	d := web.IndexData{
		PageData:    p,
		Items:       convertAll(allPics),
		TitleFlavor: "dog pics",
	}
	err = t.Execute(w, d)
	if checkHTTPErrorf(w, http.StatusInternalServerError, "execute index: %w", err) {
		log.Infof("error execute index: %s", err.Error())
		return
	}
	return
}

func convertAll(in []database.DogPic) []web.DogPic {
	var o []web.DogPic
	for _, p := range in {
		o = append(o, convert(p))
	}
	return o
}
func convert(in database.DogPic) web.DogPic {
	return web.DogPic{
		Dog:                *in.Dog,
		URL:                *in.URL,
		FriendlyUploadDate: friendlify(in.Timestamp),
	}
}

func friendlify(timestamp int64) string {
	now := time.Now()

	hourDiff := now.Sub(time.Unix(timestamp, 0)).Hours()

	crossesMidnight := (float64(now.Hour())-hourDiff < 0)

	switch {
	case hourDiff < 24:
		if crossesMidnight {
			return "yesterday"
		} else {
			return "today"
		}
	case hourDiff/24 < 7:
		return fmt.Sprintf("%d days ago", int(hourDiff/24))
	}
	return fmt.Sprintf("%d weeks ago", int(hourDiff/24/7))
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
		Stylesheet: fmt.Sprintf("https://%s.s3.amazonaws.com/main.css", publicAssetsBucket),
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
	if !(mimeImageTypeRegex.MatchString(handler.Header["Content-Type"][0])) {
		log.Infof("file %s is not an image", handler.Filename)
		http.Redirect(w, r, fmt.Sprintf("/sweet/%s/add", ps.ByName("dogName")), http.StatusSeeOther)
	}
	log.Infof("Uploaded File: %+v\n", handler.Filename)
	log.Infof("File Size: %+v\n", handler.Size)
	log.Infof("MIME Header: %+v\n", handler.Header)

	sess, err := session.NewSession()
	if err != nil {
		log.Warn("error create session: %w", err)
		http.Error(w, "can't create s3 client", http.StatusInternalServerError)
	}
	s3client := s3.NewS3Uploader(publicDogPicBucket, sess)
	s3Key := fmt.Sprintf("%s/%d.gif", ps.ByName("dogName"), time.Now().Unix())
	res, err := s3client.PublicUpload(file, s3Key)
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
		URL:       aws.String(url),
	}
	dogsvc := database.NewDogService(sess, dynamoTable)
	_, err = dogsvc.Add(dogpic)
	if err != nil {
		log.Warnf("error add pic to ddb: %s", err.Error())
		http.Error(w, "can't add file to ddb", http.StatusInternalServerError)
		deleteOut, err2 := s3client.Delete(s3Key)
		if err2 != nil {
			// TODO add reaper
			log.Warnf("delete pic from s3: %s", err2.Error())
			return
		}
		log.Infof("deleted object from s3: %v", deleteOut)
		return
	}
	rdURL := fmt.Sprintf("/sweet/%s", ps.ByName("dogName"))
	http.Redirect(w, r, rdURL, http.StatusSeeOther)
}

func ShowDog(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t, err := web.LoadTemplate("cmd/frontend/index.html")
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot read template: %w", err) {
		return
	}
	sess, err := session.NewSession()
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot get dynamo connection: %w", err) {
		return
	}
	ddb := database.NewDogService(sess, dynamoTable)
	dogPics, err := ddb.GetDog(ps.ByName("dogName"))
	if checkHTTPErrorf(w, http.StatusInternalServerError, "cannot scan ddb: %w", err) {
		return
	}

	p := web.PageData{
		Stylesheet: fmt.Sprintf("https://%s.s3.amazonaws.com/main.css", publicAssetsBucket),
	}
	d := web.IndexData{
		PageData:    p,
		Items:       convertAll(dogPics),
		TitleFlavor: ps.ByName("dogName"),
	}
	err = t.Execute(w, d)
	if checkHTTPErrorf(w, http.StatusInternalServerError, "execute index: %w", err) {
		log.Infof("error execute index: %s", err.Error())
		return
	}
	return
}

func getTimestamp() int64 {
	t := time.Now()
	return t.Unix()
}

func Healthcheck(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	w.WriteHeader(http.StatusOK)
	return
}

func main() {

	log.Infof("started new task")
	r := httprouter.New()

	r.GET("/", Index)
	r.GET("/sweet/:dogName/add", AddPage)
	r.POST("/sweet/:dogName/add", AddNewDogPic)
	r.GET("/sweet/:dogName", ShowDog)

	r.GET("/healthcheck", Healthcheck)

	log.Fatal(http.ListenAndServe(":80", r))

}
