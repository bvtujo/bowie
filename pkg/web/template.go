package web

import (
	"fmt"
	"html/template"
	"net/http"
)

func LoadTemplate(f string) (*template.Template, error) {
	temp, err := template.ParseFiles(f)
	if err != nil {
		return nil, err
	}
	return temp, nil
}

type PageData struct {
	Stylesheet string
}

type IndexData struct {
	PageData
	Items []DogPic
}

type DogPic struct {
	URL     string
	DogName string
}

func executeIndex(w http.ResponseWriter, t *template.Template, d IndexData) error {
	err := t.Execute(w, d)
	if err != nil {
		return fmt.Errorf("execute index template: %w", err)
	}
	return nil
}

type AddData struct {
	PageData
	DogName string
}
