package blog

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
	"encoding/binary"
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

type Timestamps struct {
	Created time.Time `datastore:"created"`
	Updated time.Time `datastore:"updated"`
}

type Post struct {
	Slug        string `datastore:"-"`
	Title       string `datastore:"title,noindex"`
	Text        string `datastore:"text,noindex"`
	NumComments int32  `datastore:"numComments,noindex"`
	Draft       bool   `datastore:"draft"`
	Timestamps
}

func (p *Post) Url() (template.URL, error) {
	return p.getRoute("showPost")
}

func (p *Post) EditUrl() (template.URL, error) {
	return p.getRoute("editPost")
}

type Comment struct {
	Author      string `datastore:"author,noindex"`
	AuthorEmail string `datastore:"authorEmail,noindex"`
	AuthorUrl   string `datastore:"authorUrl,noindex"`
	Kind        string `datastore:"kind,noindex"`
	Text        string `datastore:"text,noindex"`
	Timestamps
}

const PostEntity = "blog_post"
const CommentEntity = "blog_comment"
const postsPerPage = 10
const postCountCacheKey = "blog_post_count"

func (p *Post) getRoute(name string) (template.URL, error) {
	url, err := router.GetRoute(name).URL(
		"ymd", p.Created.Format("2006/01/02"),
		"slug", p.Slug)
	if err != nil {
		return "", err
	}
	return template.URL(url.String()), nil
}

func getPosts(c appengine.Context, page int) ([]Post, error) {
	q := datastore.NewQuery(PostEntity).
		Order("-created").
		Offset((page - 1) * postsPerPage).
		Limit(postsPerPage)
	if !user.IsAdmin(c) {
		q = q.Filter("draft =", false)
	}
	posts := make([]Post, 0, postsPerPage)
	keys, err := q.GetAll(c, &posts)
	for i := 0; i < len(keys); i++ {
		posts[i].Slug = keys[i].StringID()
	}
	return posts, err
}

func getPost(c appengine.Context, slug string) (Post, []Comment, error) {
	p := Post{}
	comments := make([]Comment, 0)

	k := datastore.NewKey(c, PostEntity, slug, 0, nil)
	err := datastore.Get(c, k, &p)
	if err != nil {
		return p, comments, err
	}
	if p.Draft && !user.IsAdmin(c) {
		// Drafts 404 for non-admin users
		return p, comments, datastore.ErrNoSuchEntity
	}

	p.Slug = k.StringID()
	_, error := datastore.NewQuery(CommentEntity).
		Ancestor(k).
		Order("created").
		GetAll(c, &comments)
	if error != nil {
		return p, comments, err
	}
	return p, comments, err
}

func getPageCount(c appengine.Context) (int, error) {
	item, err := memcache.Get(c, postCountCacheKey)
	if err == memcache.ErrCacheMiss {
		c.Infof("Counting posts")
		count, err := datastore.NewQuery(PostEntity).Count(c)
		if err != nil {
			return -1, err
		}
		c.Infof("Got %v posts", count)
		buf := make([]byte, binary.MaxVarintLen64)
		binary.PutVarint(buf, int64(count))
		item := &memcache.Item{
			Key:   postCountCacheKey,
			Value: buf,
		}
		memcache.Set(c, item) // ignore err
		return count / postsPerPage, nil
	} else if err != nil {
		return -1, err
	} else {
		if value, cnt := binary.Varint(item.Value); cnt > 0 {
			return int(value) / postsPerPage, nil
		} else {
			return -1, errors.New("Cannot decode cached count")
		}
	}
}

func storePost(c appengine.Context, p *Post) error {
	return datastore.RunInTransaction(c, func(c appengine.Context) error {
		needRecount := p.Slug == ""
		slug, err := slugify(c, p)
		if err != nil {
			return err
		}
		if _, err := datastore.Put(c, slug, p); err != nil {
			return err
		}
		if needRecount {
			c.Infof("Resetting blog_page_count")
			memcache.Delete(c, postCountCacheKey)
			getPageCount(c)
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
