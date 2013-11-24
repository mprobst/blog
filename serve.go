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
	post, comments, err := getPost(c, vars["slug"])
	if err != nil {
		return err
	}
	return renderPost(w, post, comments)
}

var decoder = schema.NewDecoder()

func editPost(c appengine.Context, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	p := Post{}
	if slug, ok := vars["slug"]; ok && r.Method != "POST" {
		var err error
		p, _, err = getPost(c, slug)
		if err != nil {
			return err
		}
		c.Infof("Post %v", p)
	} else {
		r.ParseForm()
		c.Infof("Form data: %s", r.Form)

		decoder.Decode(&p, r.Form)
		p.Created = time.Now()
	}
	p.Updated = time.Now()

	if r.Method == "POST" {
		if err := storePost(c, &p); err != nil {
			return err
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return nil
	}

	return renderEditPost(w, &p)
}
