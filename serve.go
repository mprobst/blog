package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/mail"
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
var routeShowPost,
	routeEditPost *mux.Route

func init() {
	// Use app code to render all 404s.
	router.NotFoundHandler = appEngineHandler(
		func(c appengine.Context, rw http.ResponseWriter, r *http.Request) {
			panic(datastore.ErrNoSuchEntity)
		})

	// Redirects.
	redirect := appEngineHandler(redirectToDomain)
	router.Host("www.martin-probst.com").Handler(redirect)
	router.Host("martin-probst.com").Handler(redirect)
	router.Host("www.probst.io").Handler(redirect)
	router.Handle("/", http.RedirectHandler("/blog/", http.StatusSeeOther))

	router.HandleFunc("/robots.txt", func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte("# All OK.\n"))
	})

	// Blog routes
	s := router.PathPrefix("/blog").Subrouter()
	s.StrictSlash(true)

	s.HandleFunc("/auth_check", func(rw http.ResponseWriter, req *http.Request) {
		c := appengine.NewContext(req)
		rw.Write([]byte(strconv.FormatBool(user.IsAdmin(c))))
	})

	s.HandleFunc("/", appEngineHandler(indexPage))
	s.HandleFunc("/{page:\\d*}/", appEngineHandler(indexPage))

	s.Handle("/feed", http.RedirectHandler("/blog/feed/1", http.StatusMovedPermanently))
	s.HandleFunc("/feed/{page:\\d*}", appEngineHandler(feed))

	s.HandleFunc("/new", appEngineHandler(editPost))
	postPrefix := "/{ymd:\\d{4}/\\d{1,2}/\\d{1,2}}/{slug}/"
	routeShowPost = s.HandleFunc(postPrefix, appEngineHandler(showPost))
	routeEditPost = s.HandleFunc(postPrefix+"edit", appEngineHandler(editPost))

	http.Handle("/", router)

	log.Println("Routes set up, ready to serve.")
}

func appEngineHandler(f func(c appengine.Context, rw http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		defer func() {
			if recovered := recover(); recovered != nil {
				stack := make([]byte, 4*(2<<10))
				count := runtime.Stack(stack, false)
				stack = stack[:count]
				handleError(c, rw, r, recovered, stack)
			}
		}()
		f(c, rw, r)
	}
}

func handleError(c appengine.Context, rw http.ResponseWriter, r *http.Request, obj interface{}, stack []byte) {
	code := http.StatusInternalServerError
	msg := "An internal error occurred"
	details := fmt.Sprintf("%s", stack)
	if obj == datastore.ErrNoSuchEntity {
		code = http.StatusNotFound
		msg = "Not found"
	} else if err, ok := obj.(error); ok {
		c.Errorf("Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %s\n%s", err.Error(), details)
	} else {
		c.Errorf("Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %+v\n%s", obj, details)
	}
	mailMsg := &mail.Message{
		Sender:  "martin@probst.io",
		To:      []string{"martin@probst.io"},
		Subject: fmt.Sprintf("[blog] Server Error - %s", msg),
		Body:    fmt.Sprintf("%s http://probst.io%s\n\n%s", r.Method, r.RequestURI, details),
	}
	if err := mail.Send(c, mailMsg); err != nil {
		c.Errorf("Failed to send error report email: %v", err)
	}
	rw.WriteHeader(code)
	renderError(rw, user.IsAdmin(c), msg, details)
}

func redirectToDomain(c appengine.Context, rw http.ResponseWriter, r *http.Request) {
	host := r.Host
	url := fmt.Sprintf("http://probst.io%s", r.RequestURI)
	c.Infof("Redirecting request to %s to %s", host, url)
	// Safe to echo the user's request as we preped http://probst.io to it and it's
	// properly escaped in Redirect.
	http.Redirect(rw, r, url, http.StatusMovedPermanently)
}

func loadPostsPage(c appengine.Context, r *http.Request) ([]Post, int, int) {
	page, err := strconv.Atoi(mux.Vars(r)["page"])
	if err != nil {
		page = 1
	}
	posts := loadPosts(c, page)
	count := getPageCount(c)
	return posts, page, count
}

func indexPage(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	posts, page, count := loadPostsPage(c, r)
	renderPosts(w, posts, page, count)
}

func feed(c appengine.Context, w http.ResponseWriter, r *http.Request) {
	posts, page, count := loadPostsPage(c, r)
	lastUpdated := pageLastUpdated(c)
	renderPostsFeed(w, posts, lastUpdated, page, count)
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
		url, err := user.LoginURL(c, r.RequestURI)
		if err != nil {
			panic(err)
		}
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return
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
		http.Redirect(w, r, url.String(), http.StatusSeeOther)
		return
	}

	renderEditPost(w, &p)
}
