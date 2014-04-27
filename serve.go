package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"log"
	"net/http"
	"strconv"
	"time"
)

var router = mux.NewRouter()
var routeIndex,
	routeIndexAt,
	routeNewPost,
	routeAuthCheck,
	routeShowPost,
	routeEditPost *mux.Route

const postPrefix = "/{ymd:\\d{4}/\\d{2}/\\d{2}}/{slug}/"

func init() {
	router.Handle("/", http.RedirectHandler("/blog/", http.StatusSeeOther))

	s := router.PathPrefix("/blog").Subrouter()
	s.StrictSlash(true)

	routeAuthCheck = s.HandleFunc("/auth_check", func(rw http.ResponseWriter, req *http.Request) {
		c := appengine.NewContext(req)
		if user.IsAdmin(c) {
			http.Error(rw, "OK", http.StatusOK)
		} else {
			http.Error(rw, "Unauthorized", http.StatusForbidden)
		}
	})

	routeIndex = s.HandleFunc("/", appEngineHandler(indexPage))
	routeIndexAt = s.HandleFunc("/{page:\\d*}/", appEngineHandler(indexPage))
	routeShowPost = s.HandleFunc(postPrefix, appEngineHandler(showPost))
	routeNewPost = s.HandleFunc("/new", appEngineHandler(editPost))
	routeEditPost = s.HandleFunc(postPrefix+"edit", appEngineHandler(editPost))

	http.Handle("/", router)

	log.Println("Routes set up, ready to serve.")
}

func appEngineHandler(f func(c appengine.Context, rw http.ResponseWriter, r *http.Request) error) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		if err := f(c, rw, r); err != nil {
			handleError(rw, err)
		}
	}
}

func handleError(w http.ResponseWriter, err error) {
	if err == datastore.ErrNoSuchEntity {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
}

func indexPage(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	page, err := strconv.Atoi(mux.Vars(r)["page"])
	if err != nil {
		page = 1
	}
	posts, err := loadPosts(c, page)
	if err != nil {
		return err
	}
	count, err := getPageCount(c)
	if err != nil {
		return err
	}
	return renderPosts(w, posts, page, count)
}

func showPost(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	slug, ok := vars["slug"]
	if !ok {
		return datastore.ErrNoSuchEntity // hack, hack
	}
	post, comments, err := loadPost(c, slug)
	if err != nil {
		return err
	}
	return renderPost(w, post, comments)
}

var decoder = schema.NewDecoder()

func editPost(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	if !user.IsAdmin(c) {
		return fmt.Errorf("Unauthorized")
	}

	p := Post{}

	vars := mux.Vars(r)
	if slug, ok := vars["slug"]; ok {
		var err error
		p, _, err = loadPost(c, slug)
		if err != nil {
			return err
		}
	} else {
		// New post.
		p.Created = time.Now()
	}
	var action string

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			return err
		}
		c.Infof("Form data: %v", r.Form)
		action = r.Form.Get("action")
		r.Form.Del("action") // The button used to post, not of interest below
		p.Draft = false      // Default to false, unless the form contains true
		if err := decoder.Decode(&p, r.Form); err != nil {
			return err
		}
	}
	p.Updated = time.Now()

	if r.Method == "POST" && action == "Post" {
		if err := storePost(c, &p); err != nil {
			return err
		}
		url, err := p.Route(routeShowPost)
		if err != nil {
			return err
		}
		http.Redirect(w, r, url.String(), http.StatusFound)
		return nil
	}

	return renderEditPost(w, &p)
}
