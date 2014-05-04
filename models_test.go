package blog

import (
	"appengine/aetest"
	"appengine/memcache"
	"fmt"
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

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
	time.Sleep(100 * time.Millisecond)

	// Invalidate cache.
	memcache.Delete(m.ctx, postCountCacheKey)
	c.Check(getPageCount(m.ctx), Equals, 2)
}

func (m *ModelsTest) TestLoadStorePost(c *C) {
	p := Post{
		Title: "Hello World",
		Text:  "Test content",
		Timestamps: Timestamps{
			Created: time.Now(),
			Updated: time.Now(),
		},
	}
	storePost(m.ctx, &p)
	c.Check(p.Slug, Not(IsNil))
	c.Check(p.Slug.StringID(), Equals, "hello-world")

	p, comments := loadPost(m.ctx, "hello-world")
	c.Check(p.Title, Equals, "Hello World")
	c.Check(len(comments), Equals, 0)
}

func (m *ModelsTest) TestPageLastUpdated(c *C) {
	createdTime := time.Now().Truncate(1 * time.Second)
	p := Post{Timestamps: Timestamps{Created: createdTime}}
	storePost(m.ctx, &p)
	// Wait for writes to apply - no way to actually flush datastore for test.
	time.Sleep(100 * time.Millisecond)
	lastUpdated := pageLastUpdated(m.ctx)
	c.Check(lastUpdated, Equals, createdTime)
}
