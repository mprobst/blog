package blog

import (
	"appengine"
	"appengine/datastore"
	"fmt"
	"regexp"
	"time"
)

type Post struct {
	Slug        string    `datastore:"slug"`
	Title       string    `datastore:"title"`
	Text        string    `datastore:"text"`
	Created     time.Time `datastore:"created"`
	Updated     time.Time `datastore:"updated"`
	NumComments int32     `datastore:"numComments"`
	Draft       bool      `datastore:"draft"`
}

func (p *Post) Url() string {
	return fmt.Sprintf("%d/%d/%d/%s", p.Created.Year(), p.Created.Month(),
		p.Created.Day(), p.Slug)
}

func getPosts(c appengine.Context, page int) ([]Post, error) {
	q := datastore.NewQuery("Post").Order("-created").Limit(10)
	posts := make([]Post, 0, 10)
	_, err := q.GetAll(c, &posts)
	return posts, err
}

func getPost(c appengine.Context, slug string) (Post, error) {
	k := datastore.NewKey(c, "Post", slug, 0, nil)
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
var dashesRE = regexp.MustCompile("-+")

func slugify(c appengine.Context, p *Post) (*datastore.Key, error) {
	slug := p.Title
	slug = slugRE.ReplaceAllLiteralString(slug, "")
	slug = dashesRE.ReplaceAllLiteralString(slug, "-")
	newSlug := slug
	dummy := Post{}
	for i := 1; i <= 5; i++ {
		key := datastore.NewKey(c, "Post", newSlug, 0, nil)
		if datastore.Get(c, key, &dummy) == datastore.ErrNoSuchEntity {
			p.Slug = newSlug
			return key, nil // Found a free one
		}
		newSlug = fmt.Sprint(slug, "-", i)
	}
	return nil, fmt.Errorf("no free slug for post with title: %s", p.Title)
}
