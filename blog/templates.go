package blog

import (
	"bytes"
	"github.com/knieriem/markdown"
	"html/template"
	"io"
	"strings"
	"time"
)

var parser = markdown.NewParser(&markdown.Extensions{})

var funcMap = template.FuncMap{
	"dateTime": func(t time.Time) string {
		return t.Format("Monday, January 2, 2006, 15:04")
	},
	"markdown": func(s string) template.HTML {
		var buffer bytes.Buffer
		reader := strings.NewReader(s)
		parser.Markdown(reader, markdown.ToHTML(&buffer))
		return template.HTML(buffer.String())
	},
}

var templates map[string]*template.Template

func init() {
	templates = make(map[string]*template.Template)
	for _, tmpl := range []string{"blog/posts.html", "blog/edit.html"} {
		templates[tmpl] = template.Must(
			template.New(tmpl).Funcs(funcMap).ParseFiles("blog/layout.html", "blog/post.html", tmpl))
	}
}

func renderPosts(wr io.Writer, posts []Post, page int) error {
	return templates["blog/posts.html"].ExecuteTemplate(wr, "layout", map[string]interface{}{
		"baseUri": "/",
		"Posts":   posts,
	})
}

func renderEditPost(wr io.Writer, post *Post) error {
	return templates["blog/edit.html"].ExecuteTemplate(wr, "layout", map[string]interface{}{
		"baseUri": "/",
		"Post":    post,
	})
}
