package blog

import (
	. "launchpad.net/gocheck"
	"testing"
)

func TestModels(t *testing.T) { TestingT(t) }

type ModelsTest struct{}

var _ = Suite(&ModelsTest{})

func (m *ModelsTest) TestSlugCreation(c *C) {
	c.Check(TitleToSlug("Hello World"), Equals, "hello-world")
	c.Check(TitleToSlug("Hello World     123 -- omg"), Equals, "hello-world-123-omg")
}
