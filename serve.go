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
	"runtime"
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

func appEngineHandler(f func(c appengine.Context, rw http.ResponseWriter, r *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		defer func() {
			if r := recover(); r != nil {
				stack := make([]byte, 4*(2<<10))
				count := runtime.Stack(stack, false)
				stack = stack[:count]
				handleError(c, rw, r, stack)
			}
		}()
		f(c, rw, r)
	}
}

func handleError(c appengine.Context, rw http.ResponseWriter, obj interface{}, stack []byte) {
	code := http.StatusInternalServerError
	msg := "An internal error occurred"
	details := fmt.Sprintf("%s", stack)
	if obj == datastore.ErrNoSuchEntity {
		code = http.StatusNotFound
		msg = "Not found"
	} else if err, ok := obj.(error); ok {
		c.Criticalf("Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %s", err.Error())
	} else {
		c.Criticalf("Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %+v\n%s", obj, details)
	}
	rw.WriteHeader(code)
	renderError(rw, user.IsAdmin(c), msg, details)
}

func indexPage(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	page, err := strconv.Atoi(mux.Vars(r)["page"])
	if err != nil {
		page = 1
	}
	posts := loadPosts(c, page)
	count := getPageCount(c)
	renderPosts(w, posts, page, count)
}

func showPost(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug, ok := vars["slug"]
	if !ok {
		panic(datastore.ErrNoSuchEntity) // hack, hack
	}
	post, comments := loadPost(c, slug)
	renderPost(w, post, comments)
}

var decoder = schema.NewDecoder()

func editPost(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	if !user.IsAdmin(c) {
		panic("Unauthorized")
	}

	p := Post{}

	vars := mux.Vars(r)
	if slug, ok := vars["slug"]; ok {
		p, _ = loadPost(c, slug)
	} else {
		// New post.
		p.Created = time.Now()
	}
	var action string

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			panic(err)
		}
		c.Infof("Form data: %v", r.Form)
		action = r.Form.Get("action")
		r.Form.Del("action") // The button used to post, not of interest below
		p.Draft = false      // Default to false, unless the form contains true
		if err := decoder.Decode(&p, r.Form); err != nil {
			panic(err)
		}
	}
	p.Updated = time.Now()

	if r.Method == "POST" && action == "Post" {
		storePost(c, &p)
		url := p.Route(routeShowPost)
		http.Redirect(w, r, url.String(), http.StatusFound)
		return
	}

	renderEditPost(w, &p)
}
