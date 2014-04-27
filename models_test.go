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

type ModelsTest struct{}

var _ = Suite(&ModelsTest{})

func (m *ModelsTest) TestSlugCreation(c *C) {
	c.Check(titleToSlug("Hello World"), Equals, "hello-world")
	c.Check(
		titleToSlug("Hello World     123 -- omg"), Equals, "hello-world-123-omg")
}

func assertNewContext(c *C) aetest.Context {
	ctx, err := aetest.NewContext(nil)
	c.Assert(err, IsNil)
	return ctx
}

func (m *ModelsTest) TestPageCount(c *C) {
	ctx := assertNewContext(c)
	defer ctx.Close()

	c.Check(getPageCount(ctx), Equals, 1)
	ctx.Infof("Checking memcached version")
	c.Check(getPageCount(ctx), Equals, 1)

	for i := 0; i < 11; i++ {
		p := Post{Title: fmt.Sprintf("t%d", i)}
		storePost(ctx, &p)
	}
	// Wait for writes to apply - no way to actually flush datastore for test.
	time.Sleep(100 * time.Millisecond)

	// Invalidate cache.
	memcache.Delete(ctx, postCountCacheKey)
	c.Check(getPageCount(ctx), Equals, 2)
}

func (m *ModelsTest) TestLoadStorePost(c *C) {
	ctx := assertNewContext(c)
	defer ctx.Close()

	p := Post{
		Title: "Hello World",
		Text:  "Test content",
		Timestamps: Timestamps{
			Created: time.Now(),
			Updated: time.Now(),
		},
	}
	storePost(ctx, &p)
	c.Check(p.Slug, Not(IsNil))
	c.Check(p.Slug.StringID(), Equals, "hello-world")

	p, comments := loadPost(ctx, "hello-world")
	c.Check(p.Title, Equals, "Hello World")
	c.Check(len(comments), Equals, 0)
}
