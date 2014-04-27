package blog

import (
	"bytes"
	"github.com/mprobst/blackfriday"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// var parser = markdown.NewParser(&markdown.Extensions{})

var funcMap = template.FuncMap{
	"dateTime": func(t time.Time) string {
		return t.Format("Monday, January 2, 2006, 15:04")
	},
	"markdown": func(s string) template.HTML {
		// MarkdownCommon sanitizes HTML.
		formatted := string(blackfriday.MarkdownCommon([]byte(s)))
		return template.HTML(formatted)
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

func renderPost(wr io.Writer, post Post, comments []Comment) {
	renderTemplate(wr, templates["tmpl/post_single.html"], map[string]interface{}{
		"Post":     &post,
		"Comments": comments,
	})
}

func renderPosts(wr io.Writer, posts []Post, page, pageCount int) {
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

	renderTemplate(wr, templates["tmpl/post_page.html"], map[string]interface{}{
		"Posts": posts,
		"Pagination": map[string]interface{}{
			"Previous": previous,
			"Next":     next,
			"Pages":    pages,
		},
	})
}

func renderEditPost(wr io.Writer, post *Post) {
	renderTemplate(wr, templates["tmpl/post_edit.html"], map[string]interface{}{
		"Post": post,
	})
}

func renderError(wr io.Writer, withDetail bool, msg string, details string) {
	renderTemplate(wr, templates["tmpl/error.html"], map[string]interface{}{
		"Message":        msg,
		"IncludeDetails": withDetail,
		"Details":        details,
	})
}

func renderTemplate(wr io.Writer, t *template.Template, data map[string]interface{}) {
	data["baseUri"] = "/blog/"
	// Buffer the rendered output so that potential errors don't end up mixed with the output
	var buffer bytes.Buffer
	if err := t.ExecuteTemplate(&buffer, "layout", data); err != nil {
		panic(err)
	}
	if _, err := buffer.WriteTo(wr); err != nil {
		panic(err)
	}
}
