package blog

import (
	"appengine/aetest"
	"appengine/user"
	. "launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestServing(t *testing.T) { TestingT(t) }

type ServingTest struct {
	ctx aetest.Context
}

func (s *ServingTest) SetUpTest(c *C) {
	ctx, err := aetest.NewContext(nil)
	c.Assert(err, IsNil)
	s.ctx = ctx

	// Calling LoginURL crashes the tests :-(
	s.ctx.Login(&user.User{
		Email: "test@example.com",
		Admin: true,
	})
}

func (s *ServingTest) TearDownTest(c *C) {
	s.ctx.Close()
}

var _ = Suite(&ServingTest{})

func makeRequest() *http.Request {
	return &http.Request{
		Method: "POST",
		URL:    &url.URL{Path: "/blog/new"},
		PostForm: url.Values{
			"Title": {"Hello"},
			"Text":  {"Test Body Text"},
		},
	}
}

func (s *ServingTest) TestEditPost_Render(c *C) {
	rw := httptest.NewRecorder()
	r := &http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "/blog/new"},
	}
	editPost(s.ctx, rw, r)
	c.Check(rw.Code, Equals, http.StatusOK)
}

func (s *ServingTest) TestEditPost_Preview(c *C) {
	rw := httptest.NewRecorder()
	r := makeRequest()
	editPost(s.ctx, rw, r)

	str := rw.Body.String()
	c.Check(strings.Contains(str, "Test Body Text"), Equals, true,
		Commentf("Should echo form body in page"))
}

func (s *ServingTest) TestEditPost_Create(c *C) {
	t := time.Now()
	rw := httptest.NewRecorder()
	r := makeRequest()
	r.PostForm.Set("action", "Post")
	editPost(s.ctx, rw, r)

	c.Check(rw.Code, Equals, http.StatusSeeOther)
	l := rw.Header().Get("Location")
	c.Check(l, Matches, "/blog/.*/hello/")

	time.Sleep(100) // datastore catch up

	p, _ := loadPost(s.ctx, "hello")
	c.Check(p.Text, Equals, "Test Body Text")
	c.Check(p.Created.After(t), Equals, true,
		Commentf("Should be created after start: %s > %s", p.Created, t))
	c.Check(p.Updated.After(t), Equals, true,
		Commentf("Should be created after start: %s > %s", p.Updated, t))
}
