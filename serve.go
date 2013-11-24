package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
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
	routeIndexAt = s.HandleFunc("/{page:\\d*}", appEngineHandler(indexPage))
	routeShowPost = s.HandleFunc(postPrefix, appEngineHandler(showPost))
	routeNewPost = s.HandleFunc("/new", appEngineHandler(editPost))
	routeEditPost = s.HandleFunc(postPrefix+"edit", appEngineHandler(editPost))

	http.Handle("/", router)
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
	posts, err := getPosts(c, page)
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
	post, comments, err := getPost(c, getSlug(c, slug))
	if err != nil {
		return err
	}
	return renderPost(w, post, comments)
}

func getSlug(c appengine.Context, slug string) *datastore.Key {
	return datastore.NewKey(c, PostEntity, slug, 0, nil)
}

var decoder = schema.NewDecoder()

func editPost(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	p := Post{}

	vars := mux.Vars(r)
	if slug, ok := vars["slug"]; ok {
		p.Slug = getSlug(c, slug)
		var err error
		p, _, err = getPost(c, p.Slug)
		if err != nil {
			return err
		}
	}
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			return err
		}
		if err := decoder.Decode(&p, r.Form); err != nil {
			return err
		}
		p.Created = time.Now()
	}
	p.Updated = time.Now()

	if r.Method == "POST" {
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
