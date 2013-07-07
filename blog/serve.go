package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"net/http"
	"time"
)

var router = mux.NewRouter()

const postPrefix = "/{ymd:\\d{4}/\\d{2}/\\d{2}}/{slug}/"

func init() {
	router.Handle("/", http.RedirectHandler("/blog/", http.StatusSeeOther))

	s := router.PathPrefix("/blog/").Subrouter()
	s.StrictSlash(true)
	s.HandleFunc("/", indexPage).Name("index")
	s.HandleFunc("/edit", editPost).Name("newPost")

	s.HandleFunc("/auth_check", func(rw http.ResponseWriter, req *http.Request) {
		c := appengine.NewContext(req)
		if user.IsAdmin(c) {
			http.Error(rw, "OK", http.StatusOK)
		} else {
			http.Error(rw, "Unauthorized", http.StatusForbidden)
		}
	})
	s.HandleFunc(postPrefix, showPost).Name("showPost")
	s.HandleFunc(postPrefix+"edit", editPost).Name("editPost")

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

var decoder = schema.NewDecoder()

func editPost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	p := Post{}
	r.ParseForm()

	c.Infof("Form data: %s", r.Form)

	decoder.Decode(&p, r.Form)
	p.Created = time.Now()
	p.Updated = time.Now()

	if r.Method == "POST" {

		if err := storePost(c, &p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	}

	if err := renderEditPost(w, &p); err != nil {
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
