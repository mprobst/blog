package blog

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/memcache"
	"github.com/luci/gae/service/user"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

type Timestamps struct {
	Created time.Time `gae:"created"`
	Updated time.Time `gae:"updated"`
}

type Post struct {
	Slug        *datastore.Key `gae:"$key"`
	Title       string         `gae:"title,noindex"`
	Text        string         `gae:"text,noindex"`
	NumComments int32          `gae:"numComments,noindex"`
	Draft       bool           `gae:"draft"`
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
	Key         *datastore.Key `gae:"$key"`
	Author      string         `gae:"author,noindex"`
	AuthorEmail string         `gae:"authorEmail,noindex"`
	AuthorUrl   string         `gae:"authorUrl,noindex"`
	Kind        string         `gae:"kind,noindex"`
	Text        string         `gae:"text,noindex"`
	Approved    bool           `gae:"approved,noindex"`
	Timestamps
}

const (
	PostEntity          = "blog_post"
	CommentEntity       = "blog_comment"
	postsPerPage        = 10
	postCountCacheKey   = "blog_post_count"
	lastUpdatedCacheKey = "blog_last_updated"
)

func memcacheGet(c context.Context, key string, value interface{}) error {
	it, err := memcache.GetKey(c, key)
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(it.Value())).Decode(value)
}

func memcacheSet(c context.Context, key string, value interface{}, expiration time.Duration) error {
	it := memcache.NewItem(c, key)
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(value); err != nil {
		return err
	}
	it.SetValue(buf.Bytes())
	it.SetExpiration(expiration)
	return memcache.Set(c, it)
}

// loadPosts loads the given page of posts (1-based).
func loadPosts(c context.Context, page int) []Post {
	posts := make([]Post, 0, postsPerPage)

	cacheKey := pageCacheKey(page - 1)
	if !user.IsAdmin(c) {
		err := memcacheGet(c, cacheKey, &posts)
		if err == nil {
			logging.Infof(c, "Serving cached posts page")
			return posts
		}
		if err != memcache.ErrCacheMiss {
			logging.Errorf(c, "Error trying to read page cache: %s, proceeding.", err)
		}
	}

	q := datastore.NewQuery(PostEntity).
		Order("-created").
		Offset(int32((page - 1) * postsPerPage)).
		Limit(postsPerPage)
	q = filterDraft(c, q)
	err := datastore.GetAll(c, q, &posts)
	if err != nil {
		panic(err)
	}

	memcacheSet(c, cacheKey, posts, 0)

	return posts
}

func pageCacheKey(page int) string {
	return fmt.Sprintf("blog_post_page-%d", page)
}

func pageLastUpdated(c context.Context) time.Time {
	var lastUpdated time.Time
	err := memcacheGet(c, lastUpdatedCacheKey, &lastUpdated)
	if err == nil {
		return lastUpdated
	}

	q := datastore.NewQuery(PostEntity).
		Order("-updated").
		Limit(1)
	q = filterDraft(c, q)
	posts := make([]Post, 0, 1)
	if err := datastore.GetAll(c, q, &posts); err != nil {
		panic(err)
	}
	if len(posts) < 1 {
		return time.Unix(0, 0)
	}
	lastUpdated = posts[0].Updated
	// Ok to fail.
	memcacheSet(c, lastUpdatedCacheKey, lastUpdated, 0)
	logging.Infof(c, "Last Updated %s", lastUpdated)
	return lastUpdated
}

func filterDraft(c context.Context, q *datastore.Query) *datastore.Query {
	if !user.IsAdmin(c) {
		return q.Eq("draft", false)
	}
	return q
}

func createSlug(c context.Context, slugString string) *datastore.Key {
	return datastore.NewKey(c, PostEntity, slugString, 0, nil)
}

func loadPost(c context.Context, slugString string) (*Post, []Comment) {
	slug := createSlug(c, slugString)
	p := &Post{Slug: slug}

	err := datastore.Get(c, p)
	if err != nil {
		panic(err)
	}
	if p.Draft && !user.IsAdmin(c) {
		// Drafts 404 for non-admin users
		panic(datastore.ErrNoSuchEntity)
	}

	comments := make([]Comment, 0, p.NumComments)
	q := datastore.NewQuery(CommentEntity).
		Ancestor(slug).
		Order("created")
	if err := datastore.GetAll(c, q, &comments); err != nil {
		panic(err)
	}
	if actualCount := int32(len(comments)); p.NumComments != actualCount {
		// Somehow comment count got out of sync with post.NumComments,
		// fix the situation by storing post again.
		logging.Warningf(c, "Post with incorrect comment count %s: %d != %d",
			p.Url(), p.NumComments, actualCount)
		p.NumComments = actualCount
		storePost(c, p)
	}
	return p, comments
}

// Counts posts and caches the result.
func getPageCount(c context.Context) int {
	var count int64
	err := memcacheGet(c, postCountCacheKey, &count)
	if err == nil {
		return int(count/postsPerPage) + 1
	}

	// Cache misses, but also memcache not available etc.
	if err != memcache.ErrCacheMiss {
		logging.Errorf(c, "Error trying to read page count: %s, proceeding.", err)
	}

	count, err = datastore.Count(c, datastore.NewQuery(PostEntity))
	if err != nil {
		panic(err)
	}
	logging.Infof(c, "Counted %v posts", count)
	// Ignore potential error
	memcacheSet(c, postCountCacheKey, count, 1*time.Hour)

	return int(count/postsPerPage) + 1
}

func storePost(c context.Context, p *Post) {
	newPost := p.Slug == nil

	err := datastore.RunInTransaction(c, func(c context.Context) error {
		if newPost {
			slug := slugify(c, p)
			p.Slug = slug
		}
		if err := datastore.Put(c, p); err != nil {
			return err
		}
		return nil
	}, &datastore.TransactionOptions{XG: true})

	if err != nil {
		panic(err)
	}

	if newPost {
		logging.Infof(c, "Resetting blog_page_count")
		memcache.Delete(c, postCountCacheKey)

		pages := getPageCount(c)
		pageCacheKeys := make([]string, pages)
		for i := 0; i < pages; i++ {
			pageCacheKeys[i] = pageCacheKey(i)
		}
		logging.Infof(c, "Deleting page caches %s", pageCacheKeys)
		memcache.Delete(c, pageCacheKeys...)
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

func slugify(c context.Context, p *Post) *datastore.Key {
	if p.Slug != nil {
		return p.Slug
	}
	slug := titleToSlug(p.Title)
	newSlug := slug
	var lastErr error
	for i := 1; i <= 5; i++ {
		key := datastore.NewKey(c, PostEntity, newSlug, 0, nil)
		var ex *datastore.ExistsResult
		if ex, lastErr = datastore.Exists(c, key); lastErr == nil && !ex.Get(0) {
			p.Slug = key
			return key // Found a free one
		}
		newSlug = fmt.Sprint(slug, "-", i)
	}
	panic(fmt.Errorf("no free slug for post with title: %s - %v", p.Title, lastErr))
}
