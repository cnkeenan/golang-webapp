package main

import (
	"errors"
	"github.com/boltdb/bolt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
)

type Page struct {
	Title   string
	Body    []byte
	Editing bool
}

var validPath = regexp.MustCompile("^/(edit|save|view)/([a-zA-Z0-9]+)$")
var templates = template.Must(template.ParseFiles("templates/view.html", "templates/edit.html", "templates/base.layout.html"))
var database *bolt.DB
var dberr error

func (p *Page) save() error {
	filename := "data/" + p.Title + ".txt"
	return ioutil.WriteFile(filename, p.Body, 0600)
}

func getTitle(w http.ResponseWriter, r *http.Request) (string, error) {
	m := validPath.FindStringSubmatch(r.URL.Path)
	if m == nil {
		log.Println("invalid Page Title")
		http.NotFound(w, r)
		return "", errors.New("invalid Page Title")
	}
	return m[2], nil // the title will be the second subexpression
}

func loadPage(title string, editing bool) (Page, error) {
	var body []byte
	database.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Posts"))
		body = b.Get([]byte(title))
		return nil
	})

	return Page{Title: title, Body: body, Editing: editing}, nil
}

func createHandler(fn func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			log.Println("invalid Page Title")
			http.NotFound(w, r)
			return
		}
		fn(w, r, m[2])
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title, false)
	if err != nil || allzeros(p.Body) {
		http.Redirect(w, r, "/edit/"+title, http.StatusFound)
		return
	}

	var pages []Page

	database.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Posts"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			log.Printf("key =%s, value=%s\n", k, v)
			page := Page{Title: string(k), Body: v, Editing: false}
			pages = append(pages, page)
		}

		return nil
	})

	renderTemplate(w, "view", pages)
}

func editHandler(w http.ResponseWriter, r *http.Request, title string) {
	p, err := loadPage(title, true)
	var pages []Page

	if err != nil {
		p = Page{Title: title, Editing: true}
	}
	pages = append(pages, p)
	renderTemplate(w, "edit", pages)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	var pages []Page

	database.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Posts"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			log.Printf("key =%s, value=%s\n", k, v)
			page := Page{Title: string(k), Body: v, Editing: false}
			pages = append(pages, page)
		}

		return nil
	})

	renderTemplate(w, "main", pages)
}

func saveHandler(w http.ResponseWriter, r *http.Request, title string) {
	body := r.FormValue("body")

	database.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("Posts"))
		if err != nil {
			log.Fatalf("create bucket error: %s", err)
			return err
		}
		err = b.Put([]byte(title), []byte(body))
		return err
	})

	http.Redirect(w, r, "/view/"+title, http.StatusFound)
}

func renderTemplate(w http.ResponseWriter, tmpl string, p []Page) {
	log.Printf("Executing template: %s.html", tmpl)
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func allzeros(s []byte) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}

func main() {
	fileserver := http.FileServer(http.Dir("./static/"))
	http.HandleFunc("/view/", createHandler(viewHandler))
	http.HandleFunc("/edit/", createHandler(editHandler))
	http.HandleFunc("/save/", createHandler(saveHandler))
	http.Handle("/static/", http.StripPrefix("/static", fileserver))

	database, dberr = bolt.Open("my.db", 0600, nil)
	if dberr != nil {
		log.Fatal(dberr)
	}

	defer database.Close()

	log.Println("Starting server on port :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
