package blog

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"

	"github.com/luci/gae/impl/prod"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/mail"
	"github.com/luci/gae/service/info"
	"github.com/luci/gae/service/user"

	"google.golang.org/appengine"

	"github.com/luci/luci-go/common/logging"
)

var router = mux.NewRouter()
var routeShowPost,
	routeEditPost *mux.Route

func init() {
	// Use app code to render all 404s.
	router.NotFoundHandler = appEngineHandler(
		func(c context.Context, rw http.ResponseWriter, r *http.Request) {
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
		ctx := prod.Use(appengine.NewContext(req), req)
		rw.Write([]byte(strconv.FormatBool(user.IsAdmin(ctx))))
	})

	s.Handle("/", appEngineHandler(indexPage))
	s.Handle("/{page:\\d*}/", appEngineHandler(indexPage))

	s.Handle("/feed", http.RedirectHandler("/blog/feed/1", http.StatusMovedPermanently))
	s.Handle("/feed/{page:\\d*}", appEngineHandler(feed))

	s.Handle("/new", appEngineHandler(editPost))
	postPrefix := "/{ymd:\\d{4}/\\d{1,2}/\\d{1,2}}/{slug}/"
	routeShowPost = s.Handle(postPrefix, appEngineHandler(showPost))
	routeEditPost = s.Handle(postPrefix+"edit", appEngineHandler(editPost))

	router.HandleFunc("/.well-known/acme-challenge/{challenge}", func(rw http.ResponseWriter, req *http.Request) {
		c := mux.Vars(req)["challenge"]
		if c == "challenge" {
			rw.Write([]byte("response"))
		} else if c == "challenge" {
			rw.Write([]byte("response"))
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			rw.Write([]byte("Unexpected challenge"))
		}
	})

	router.Handle("/_/setup_fixture", appEngineHandler(func(ctx context.Context, rw http.ResponseWriter, r *http.Request) {
		if !info.IsDevAppServer(ctx) {
			panic(datastore.ErrNoSuchEntity)
		}
		storeDevelopmentFixture(ctx)
		rw.WriteHeader(http.StatusOK)
	}))

	http.Handle("/", router)
}

type appEngineHandlerFunc func(c context.Context, rw http.ResponseWriter, r *http.Request)

func appEngineHandler(f appEngineHandlerFunc) http.Handler {
	recovering := func(rw http.ResponseWriter, r *http.Request) {
		ctx := prod.Use(appengine.NewContext(r), r)
		defer func() {
			if recovered := recover(); recovered != nil {
				stack := make([]byte, 4*(2<<10))
				count := runtime.Stack(stack, false)
				stack = stack[:count]
				handleError(ctx, rw, r, recovered, stack)
			}
		}()
		f(ctx, rw, r)
	}
	return http.HandlerFunc(recovering)
}

func handleError(c context.Context, rw http.ResponseWriter, r *http.Request, obj interface{}, stack []byte) {
	code := http.StatusInternalServerError
	msg := "An internal error occurred"
	details := fmt.Sprintf("%s", stack)
	if obj == datastore.ErrNoSuchEntity {
		code = http.StatusNotFound
		msg = "Not found"
	} else if err, ok := obj.(error); ok {
		logging.Errorf(c, "Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %s\n%s", err.Error(), details)
	} else {
		logging.Errorf(c, "Error: %+v\n%s", obj, stack)
		details = fmt.Sprintf("Error: %+v\n%s", obj, details)
	}
	if code >= 500 {
		mailMsg := &mail.Message{
			Sender:  "martin@probst.io",
			To:      []string{"martin@probst.io"},
			Subject: fmt.Sprintf("[blog] Server Error - %s", msg),
			Body:    fmt.Sprintf("%s http://probst.io%s\n\n%s", r.Method, r.RequestURI, details),
		}
		if err := mail.Send(c, mailMsg); err != nil {
			logging.Errorf(c, "Failed to send error report email: %v", err)
		}
	}
	rw.WriteHeader(code)
	renderError(rw, user.IsAdmin(c), msg, details)
}

func redirectToDomain(c context.Context, rw http.ResponseWriter, r *http.Request) {
	host := r.Host
	url := fmt.Sprintf("http://probst.io%s", r.RequestURI)
	logging.Infof(c, "Redirecting request to %s to %s", host, url)
	// Safe to echo the user's request as we preped http://probst.io to it and it's
	// properly escaped in Redirect.
	http.Redirect(rw, r, url, http.StatusMovedPermanently)
}

func loadPostsPage(c context.Context, r *http.Request) ([]Post, int, int) {
	page, err := strconv.Atoi(mux.Vars(r)["page"])
	if err != nil {
		page = 1
	}
	posts := loadPosts(c, page)
	count := getPageCount(c)
	if page > count {
		panic(datastore.ErrNoSuchEntity)
	}
	return posts, page, count
}

func indexPage(c context.Context, w http.ResponseWriter, r *http.Request) {
	posts, page, count := loadPostsPage(c, r)
	renderPosts(w, posts, page, count)
}

func feed(c context.Context, w http.ResponseWriter, r *http.Request) {
	posts, page, count := loadPostsPage(c, r)
	lastUpdated := pageLastUpdated(c)
	renderPostsFeed(w, posts, lastUpdated, page, count)
}

func showPost(c context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	slug, ok := vars["slug"]
	if !ok {
		panic(datastore.ErrNoSuchEntity) // hack, hack
	}
	post, comments := loadPost(c, slug)
	renderPost(w, post, comments)
}

var decoder = schema.NewDecoder()

func editPost(c context.Context, w http.ResponseWriter, r *http.Request) {
	if !user.IsAdmin(c) {
		// info.Get(c).
		url, err := user.LoginURL(c, r.RequestURI)
		if err != nil {
			panic(err)
		}
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
		return
	}

	var p *Post

	vars := mux.Vars(r)
	if slug, ok := vars["slug"]; ok {
		p, _ = loadPost(c, slug)
	} else {
		// New post.
		p = &Post{}
		p.Created = time.Now().UTC()
	}
	var action string

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			panic(err)
		}
		logging.Infof(c, "Form data: %v", r.Form)
		action = r.Form.Get("action")
		r.Form.Del("action") // The button used to post, not of interest below
		p.Draft = false      // Default to false, unless the form contains true
		if err := decoder.Decode(p, r.Form); err != nil {
			panic(err)
		}
	}
	p.Updated = time.Now().UTC()

	if r.Method == "POST" && action == "Post" {
		storePost(c, p)
		url := p.Route(routeShowPost)
		http.Redirect(w, r, url.String(), http.StatusSeeOther)
		return
	}

	renderEditPost(w, p)
}
