package blog

import (
	"fmt"
	"testing"
	"time"
	. "launchpad.net/gocheck"
	"appengine/aetest"
	"appengine/datastore"
	"appengine/memcache"
)

const DATASTORE_APPLY_WAIT = 500 * time.Millisecond

func TestModels(t *testing.T) { TestingT(t) }

type ModelsTest struct {
	ctx aetest.Context
}

func (m *ModelsTest) SetUpTest(c *C) {
	ctx, err := aetest.NewContext(nil)
	c.Assert(err, IsNil)
	m.ctx = ctx
}

func (m *ModelsTest) TearDownTest(c *C) {
	m.ctx.Close()
}

var _ = Suite(&ModelsTest{})

func (m *ModelsTest) TestSlugCreation(c *C) {
	c.Check(titleToSlug("Hello World"), Equals, "hello-world")
	c.Check(
		titleToSlug("Hello World     123 -- omg"), Equals, "hello-world-123-omg")
}

func (m *ModelsTest) TestPageCount(c *C) {
	c.Check(getPageCount(m.ctx), Equals, 1)
	m.ctx.Infof("Checking memcached version")
	c.Check(getPageCount(m.ctx), Equals, 1)

	for i := 0; i < 11; i++ {
		p := Post{Title: fmt.Sprintf("t%d", i)}
		storePost(m.ctx, &p)
	}
	// Wait for writes to apply - no way to actually flush datastore for test.
	time.Sleep(DATASTORE_APPLY_WAIT)

	// Invalidate cache.
	memcache.Delete(m.ctx, postCountCacheKey)
	c.Check(getPageCount(m.ctx), Equals, 2)
}

var updated time.Time = time.Now().Truncate(1 * time.Second)
var created time.Time = updated.Add(-20 * time.Minute)

func testPost() (*Post, []Comment) {
	p := Post{
		Title: "Hello World",
		Text:  "Test content",
		Timestamps: Timestamps{
			Created: created,
			Updated: updated,
		},
		NumComments: 2,
	}
	comments := []Comment{Comment{
		Author: "testAuthor1",
		Text:   "textText1",
		Timestamps: Timestamps{
			Created: created.Add(20 * time.Minute),
		},
	}, Comment{
		Author: "testAuthor2",
		Text:   "textText2",
		Timestamps: Timestamps{
			Created: created.Add(40 * time.Minute),
		},
	}}
	return &p, comments
}

func (m *ModelsTest) TestLoadStorePost(c *C) {
	p, _ := testPost()
	storePost(m.ctx, p)
	c.Check(p.Slug, Not(IsNil))
	c.Check(p.Slug.StringID(), Equals, "hello-world")

	p, comments := loadPost(m.ctx, "hello-world")
	c.Check(p.Title, Equals, "Hello World")
	c.Check(len(comments), Equals, 0)
}

func (m *ModelsTest) TestPageLastUpdated(c *C) {
	p, _ := testPost()
	storePost(m.ctx, p)
	// Wait for writes to apply - no way to actually flush datastore for test.
	time.Sleep(DATASTORE_APPLY_WAIT)
	lastUpdated := pageLastUpdated(m.ctx)
	c.Check(lastUpdated, Equals, updated)
}

func (m *ModelsTest) TestPageLoadFixesCommentCount(c *C) {
	p, comments := testPost()
	comments = append(comments, Comment{
		Author: "testAuthor3",
		Text:   "textText3",
	})
	c.Check(p.NumComments, Equals, int32(2))
	storePost(m.ctx, p)

	key := datastore.NewKey(m.ctx, CommentEntity, "", 0, p.Slug)
	datastore.PutMulti(m.ctx, []*datastore.Key{key, key, key}, comments)

	loaded, comments := loadPost(m.ctx, p.Slug.StringID())
	c.Check(loaded.NumComments, Equals, int32(3))
	c.Check(len(comments), Equals, 3)
}
