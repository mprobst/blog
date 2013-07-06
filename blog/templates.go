package blog

import (
	"html/template"
	"io"
	"time"
)

var funcMap = template.FuncMap{
	"dateTime": func(t time.Time) string {
		return t.Format("Monday, January 2, 2006, 15:04")
	},
}

var templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("blog/*.html"))

func renderPosts(wr io.Writer, posts []Post, page int) error {
	return templates.ExecuteTemplate(wr, "page", map[string]interface{}{
		"Posts": posts,
	})
}

func (p *Post) TextHtml() template.HTML {
	return template.HTML(p.Text)
}

var editTemplate = template.Must(template.New("edit").Parse(EDIT_TEMPLATE_HTML))

const EDIT_TEMPLATE_HTML = `
<html>
  <body>
    <form action="/edit/create" method="post">
      Title: <input name="title"><br/>
      Text:<br/>
      <textarea name="text"></textarea>
      <br/>
      <input type="submit">
    </form>
  </body>
</html>
`
