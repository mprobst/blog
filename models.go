package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Timestamps struct {
	Created time.Time `datastore:"created"`
	Updated time.Time `datastore:"updated"`
}

type Post struct {
	Slug        *datastore.Key `datastore:"-"`
	Title       string         `datastore:"title,noindex"`
	Text        string         `datastore:"text,noindex"`
	NumComments int32          `datastore:"numComments,noindex"`
	Draft       bool           `datastore:"draft"`
	Timestamps
}

func (p *Post) Url() template.URL {
	return p.TemplateRoute(routeShowPost)
}

func (p *Post) EditUrl() template.URL {
	return p.TemplateRoute(routeEditPost)
}

func (p *Post) Route(route *mux.Route) *url.URL {
	u, err := route.URL(
		"ymd", p.Created.Format("2006/01/02"),
		"slug", p.Slug.StringID())
	if err != nil {
		panic(err)
	}
	return u
}

func (p *Post) TemplateRoute(route *mux.Route) template.URL {
	url := p.Route(route)
	return template.URL(url.String())
}

type Comment struct {
	Author      string `datastore:"author,noindex"`
	AuthorEmail string `datastore:"authorEmail,noindex"`
	AuthorUrl   string `datastore:"authorUrl,noindex"`
	Kind        string `datastore:"kind,noindex"`
	Text        string `datastore:"text,noindex"`
	Approved    bool   `datastore:"approved,noindex"`
	Timestamps
}

const (
	PostEntity        = "blog_post"
	CommentEntity     = "blog_comment"
	postsPerPage      = 10
	postCountCacheKey = "blog_post_count"
)

func loadPosts(c appengine.Context, page int) []Post {
	q := datastore.NewQuery(PostEntity).
		Order("-created").
		Offset((page - 1) * postsPerPage).
		Limit(postsPerPage)
	if !user.IsAdmin(c) {
		q = q.Filter("draft =", false)
	}
	posts := make([]Post, 0, postsPerPage)
	keys, err := q.GetAll(c, &posts)
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(keys); i++ {
		posts[i].Slug = keys[i]
	}
	return posts
}

func createSlug(c appengine.Context, slugString string) *datastore.Key {
	return datastore.NewKey(c, PostEntity, slugString, 0, nil)
}

func loadPost(c appengine.Context, slugString string) (Post, []Comment) {
	slug := createSlug(c, slugString)
	p := Post{Slug: slug}
	comments := make([]Comment, 0)

	err := datastore.Get(c, slug, &p)
	if err != nil {
		panic(err)
	}
	if p.Draft && !user.IsAdmin(c) {
		// Drafts 404 for non-admin users
		panic(datastore.ErrNoSuchEntity)
	}

	q := datastore.NewQuery(CommentEntity).
		Ancestor(slug).
		Order("created")
	if _, err := q.GetAll(c, &comments); err != nil {
		panic(err)
	}
	return p, comments
}

// Counts posts and caches the result.
func getPageCount(c appengine.Context) int {
	item, err := memcache.Get(c, postCountCacheKey)
	var count int
	if err == nil {
		if value, cnt := binary.Varint(item.Value); cnt > 0 {
			count = int(value)
		} else {
			panic(errors.New("Cannot decode cached count"))
		}
	} else if err == memcache.ErrCacheMiss {
		count, err = datastore.NewQuery(PostEntity).Count(c)
		if err != nil {
			panic(err)
		}
		c.Infof("Counted %v posts", count)
		buf := make([]byte, binary.MaxVarintLen64)
		binary.PutVarint(buf, int64(count))
		item := &memcache.Item{
			Key:        postCountCacheKey,
			Value:      buf,
			Expiration: 1 * time.Hour,
		}
		memcache.Set(c, item) // ignore err
	} else if err != nil {
		panic(err)
	}
	return (count / postsPerPage) + 1
}

func storePost(c appengine.Context, p *Post) {
	newPost := p.Slug == nil

	err := datastore.RunInTransaction(c, func(c appengine.Context) error {
		if newPost {
			slug := slugify(c, p)
			p.Slug = slug
		}
		if _, err := datastore.Put(c, p.Slug, p); err != nil {
			return err
		}
		return nil
	}, &datastore.TransactionOptions{XG: true})

	if err != nil {
		panic(err)
	}

	if newPost {
		c.Infof("Resetting blog_page_count")
		memcache.Delete(c, postCountCacheKey)
	}
}

var (
	slugRE   = regexp.MustCompile("[^-A-Za-z0-9_]")
	dashesRE = regexp.MustCompile("-{2,}")
)

func titleToSlug(title string) string {
	slug := title
	slug = strings.Replace(slug, " ", "-", -1)
	slug = slugRE.ReplaceAllLiteralString(slug, "")
	slug = dashesRE.ReplaceAllLiteralString(slug, "-")
	return strings.ToLower(slug)
}

func slugify(c appengine.Context, p *Post) *datastore.Key {
	if p.Slug != nil {
		return p.Slug
	}
	slug := titleToSlug(p.Title)
	newSlug := slug
	dummy := Post{}
	for i := 1; i <= 5; i++ {
		key := datastore.NewKey(c, PostEntity, newSlug, 0, nil)
		if datastore.Get(c, key, &dummy) == datastore.ErrNoSuchEntity {
			p.Slug = key
			return key // Found a free one
		}
		newSlug = fmt.Sprint(slug, "-", i)
	}
	panic(fmt.Errorf("no free slug for post with title: %s", p.Title))
}
