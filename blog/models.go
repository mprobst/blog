package blog

import (
	"appengine"
	"appengine/datastore"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

type Post struct {
	Slug        string    `datastore:"-"`
	Title       string    `datastore:"title,noindex"`
	Text        string    `datastore:"text,noindex"`
	Created     time.Time `datastore:"created"`
	Updated     time.Time `datastore:"updated"`
	NumComments int32     `datastore:"numComments,noindex"`
	Draft       bool      `datastore:"draft"`
}

const PostEntity = "blog_post"

func (p *Post) Url() template.URL {
	url, err := router.GetRoute("showPost").URL(
		"ymd", p.Created.Format("2006/01/02"),
		"slug", p.Slug)
	if err != nil {
		return template.URL(err.Error())
	}
	return template.URL(url.String())
}

func getPosts(c appengine.Context, page int) ([]Post, error) {
	q := datastore.NewQuery(PostEntity).Order("-created").Limit(10)
	posts := make([]Post, 0, 10)
	keys, err := q.GetAll(c, &posts)
	for i := 0; i < len(keys); i++ {
		posts[i].Slug = keys[i].StringID()
	}
	return posts, err
}

func getPost(c appengine.Context, slug string) (Post, error) {
	k := datastore.NewKey(c, PostEntity, slug, 0, nil)
	p := Post{}
	err := datastore.Get(c, k, &p)
	return p, err
}

func storePost(c appengine.Context, p *Post) error {
	return datastore.RunInTransaction(c, func(c appengine.Context) error {
		slug, err := slugify(c, p)
		if err != nil {
			return err
		}
		if _, err := datastore.Put(c, slug, p); err != nil {
			return err
		}
		return nil
	}, &datastore.TransactionOptions{XG: true})
}

var slugRE = regexp.MustCompile("[^A-Za-z0-9_-]")
var dashesRE = regexp.MustCompile("-{2,}")

func TitleToSlug(title string) string {
	slug := slugRE.ReplaceAllLiteralString(title, "")
	slug = dashesRE.ReplaceAllLiteralString(slug, "-")
	return strings.ToLower(slug)
}

func slugify(c appengine.Context, p *Post) (*datastore.Key, error) {
	if p.Slug != "" {
		return datastore.NewKey(c, PostEntity, p.Slug, 0, nil), nil
	}
	slug := TitleToSlug(p.Title)
	newSlug := slug
	dummy := Post{}
	for i := 1; i <= 5; i++ {
		key := datastore.NewKey(c, PostEntity, newSlug, 0, nil)
		if datastore.Get(c, key, &dummy) == datastore.ErrNoSuchEntity {
			p.Slug = newSlug
			return key, nil // Found a free one
		}
		newSlug = fmt.Sprint(slug, "-", i)
	}
	return nil, fmt.Errorf("no free slug for post with title: %s", p.Title)
}
