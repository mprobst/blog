package blog

import (
	"fmt"
	"time"

	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

func storeDevelopmentFixture(c context.Context) {
	logging.Infof(c, "Storing development fixture...")

	now := time.Now().UTC().Add(10 * 24 * time.Hour)

	for i := 0; i < 20; i++ {
		now = now.Add(1 * time.Hour)
		storePost(c, &Post{
			Title: fmt.Sprintf("My post #%d", i),
			Text:  fmt.Sprintf("This is the text of post #%d", i),
			Timestamps: Timestamps{
				Created: now.Add(-1 * time.Hour),
				Updated: now,
			},
		})
	}

	p := &Post{
		Title: "Post with comments",
		Text:  "This is the text of a post with comments",
		Timestamps: Timestamps{
			Created: now,
			Updated: now,
		},
	}

	storePost(c, p)

	comment := &Comment{
		Author:      "icke",
		AuthorEmail: "icke@hier.com",
		AuthorUrl:   "http://icke.com",
		Text:        "icke war hier",
		Approved:    true,
	}
	if err := storeComment(c, p, comment); err != nil {
		panic(err)
	}
	comment = &Comment{
		Author:      "other",
		AuthorEmail: "other@example.com",
		Text:        "other comment",
		Approved:    false,
	}
	if err := storeComment(c, p, comment); err != nil {
		panic(err)
	}
	logging.Infof(c, "Storing development fixture... done.")
}
