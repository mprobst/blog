package blog

import (
	"fmt"
	"testing"
	"time"

	"github.com/luci/gae/impl/memory"
	"github.com/luci/gae/service/datastore"
	"golang.org/x/net/context"
	. "launchpad.net/gocheck"
)

func TestModels(t *testing.T) { TestingT(t) }

type ModelsTest struct {
	ctx context.Context
}

func setUpTestingDatastore(ctx context.Context) {
	indices, err := datastore.FindAndParseIndexYAML("")
	if err != nil {
		panic(err)
	}
	t := datastore.GetTestable(ctx)
	t.Consistent(true)
	t.AddIndexes(indices...)
}

func (m *ModelsTest) SetUpTest(c *C) {
	// Strong consistency is needed for the eventually consistent page count to work.
	ctx := memory.Use(context.Background())
	m.ctx = ctx
	setUpTestingDatastore(ctx)
}

func (m *ModelsTest) TearDownTest(c *C) {
	// m.ctx.Close()
}

var _ = Suite(&ModelsTest{})

func (m *ModelsTest) TestSlugCreation(c *C) {
	c.Check(titleToSlug("Hello World"), Equals, "hello-world")
	c.Check(titleToSlug("Hello World     123 -- omg"), Equals, "hello-world-123-omg")
}

func (m *ModelsTest) TestPageCount(c *C) {
	c.Check(getPageCount(m.ctx), Equals, 1)
	fmt.Printf("Checking memcached version")
	c.Check(getPageCount(m.ctx), Equals, 1)

	for i := 0; i < 11; i++ {
		p := Post{Title: fmt.Sprintf("t%d", i)}
		storePost(m.ctx, &p)
	}

	c.Check(getPageCount(m.ctx), Equals, 2)
}

func (m *ModelsTest) TestLoadStorePost(c *C) {
	posts := loadPosts(m.ctx, 1)
	c.Check(len(posts), Equals, 0)

	p, _ := testPost()
	storePost(m.ctx, p)
	c.Check(p.Slug, Not(IsNil))
	c.Check(p.Slug.StringID(), Equals, "hello-world")

	posts = loadPosts(m.ctx, 1)
	c.Check(len(posts), Equals, 1)

	p, comments := loadPost(m.ctx, "hello-world")
	c.Check(p.Title, Equals, "Hello World")
	c.Check(p.Slug, NotNil)
	c.Check(len(comments), Equals, 0)
}

func (m *ModelsTest) TestPageLastUpdated(c *C) {
	p, _ := testPost()
	storePost(m.ctx, p)
	lastUpdated := pageLastUpdated(m.ctx)
	c.Check(lastUpdated, Equals, updated)
}

func (m *ModelsTest) TestPageLoadFixesCommentCount(c *C) {
	p, comments := testPost()
	comments = append(comments, Comment{
		Key:    datastore.NewKey(m.ctx, CommentEntity, "", 0, p.Slug),
		Author: "testAuthor3",
		Text:   "textText3",
	})
	c.Check(p.NumComments, Equals, int32(2))
	storePost(m.ctx, p)

	datastore.Put(m.ctx, comments)

	loaded, comments := loadPost(m.ctx, p.Slug.StringID())
	c.Check(loaded.NumComments, Equals, int32(3))
	c.Check(len(comments), Equals, 3)
}

var updated time.Time = time.Now().UTC().Truncate(1 * time.Second)
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
