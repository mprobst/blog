package blog

import (
	"bytes"
	"github.com/knieriem/markdown"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
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

	partials := make([]string, 0)
	templateFiles := make([]string, 0)
	err := filepath.Walk("tmpl", func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		fileName := filepath.Base(path)
		if !strings.HasSuffix(fileName, ".html") {
			return nil // Not a template
		}
		if strings.HasPrefix(filepath.Base(path), "_") {
			partials = append(partials, path)
		} else {
			templateFiles = append(templateFiles, path)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	log.Printf("Parsing templates %v with partials %v\n", templateFiles, partials)

	for _, tmpl := range templateFiles {
		current := template.New(tmpl).Funcs(funcMap)
		template.Must(current.ParseFiles(tmpl))
		for _, partial := range partials {
			template.Must(current.ParseFiles(partial))
		}
		templates[tmpl] = current
	}
}

func renderPost(wr io.Writer, post Post, comments []Comment) error {
	return renderTemplate(wr, templates["tmpl/post_single.html"], "layout", map[string]interface{}{
		"baseUri":  "/",
		"Post":     &post,
		"Comments": comments,
	})
}

func renderPosts(wr io.Writer, posts []Post, page, pageCount int) error {
	var previous, next int
	if page > 1 {
		previous = page - 1
	} else {
		previous = 0
	}
	if page < pageCount {
		next = page + 1
	} else {
		next = 0
	}

	pages := make([]bool, pageCount+1)
	pages[page] = true

	return renderTemplate(wr, templates["tmpl/post_page.html"], "layout", map[string]interface{}{
		"baseUri": "/",
		"Posts":   posts,
		"Pagination": map[string]interface{}{
			"Previous": previous,
			"Next":     next,
			"Pages":    pages,
		},
	})
}

func renderEditPost(wr io.Writer, post *Post) error {
	return renderTemplate(wr, templates["tmpl/post_edit.html"], "layout", map[string]interface{}{
		"baseUri": "/",
		"Post":    post,
	})
}

func renderTemplate(wr io.Writer, t *template.Template, name string, data map[string]interface{}) error {
	// Buffer the rendered output so that potential errors don't end up mixed with the output
	var buffer bytes.Buffer
	var err error
	if err = t.ExecuteTemplate(&buffer, name, data); err != nil {
		return err
	}
	_, err = buffer.WriteTo(wr)
	return err
}
