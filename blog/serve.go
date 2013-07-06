package blog

import (
	"appengine"
	"appengine/datastore"
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

var router = mux.NewRouter()

func init() {
	router := mux.NewRouter()
	s := router.PathPrefix("/blog/").Subrouter()
	s.HandleFunc("/", indexPage).Name("index")
	s.HandleFunc("/edit", editPost).Name("newPost")
	postPrefix := "/{year:\\d{4}}/{month:\\d{2}}/{day:\\d{2}}/{slug}/"
	s.HandleFunc(postPrefix, showPost).
		Name("showPost")
	s.HandleFunc(
		"/{year:\\d{4}}/{month:\\d{2}}/{day:\\d{2}}/{slug}/edit", editPost).
		Name("editPost")

	http.Handle("/", router)
}

func indexPage(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	posts, err := getPosts(c, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := renderPosts(w, posts, 0); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func editPost(w http.ResponseWriter, r *http.Request) {
	p := Post{}
	if err := editTemplate.Execute(w, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func showPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	c := appengine.NewContext(r)
	post, err := getPost(c, vars["slug"])
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = renderPosts(w, []Post{post}, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func createPage(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	p := Post{
		Title:   r.FormValue("title"),
		Text:    r.FormValue("text"),
		Created: time.Now(),
		Updated: time.Now(),
	}
	if err := storePost(c, &p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
