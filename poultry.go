package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cznic/ql"
)

const (
	pages = `{{ define "main" -}}
	<html>
		<head>
			<style type='text/css'>
			body
			{
				width:50%;
				margin:8px auto;
				display:flex;
				flex-direction:column;
			}

			#new-post
			{
				display:flex;
				flex-direction:column;
			}

			#new-post input[name='title']
			{
				width:100%;
			}

			#new-post textarea[name='content']
			{
				width:100%;
				height:30em;
			}
			</style>
		</head>
		<body>
			<form id='new-post' method='post' action='/post'>
				<label>Title: <input type='text' name='title' /></label>
				<label>Content: <textarea name='content'></textarea></label>
				<input type='submit' />
			</form>

			<hr />

			<div id='posts'>
				{{- range . }}
					<div class='post'>
						<div class='post-title'>{{ .Title }}</div>
						<div class='post-content'>{{ .Content }}</div>
					</div>
				{{ end -}}
			</div>
	</html>
{{- end }}`
)

var (
	DB   *sql.DB
	Tmpl *template.Template
)

func init() {
	ql.RegisterDriver()

	Tmpl = template.Must(template.New("pages").Parse(pages))
}

func handlePost(rw http.ResponseWriter, req *http.Request) {
	tx, err := DB.BeginTx(req.Context(), nil)
	if err != nil {
		log.Printf("Error: Failed to being transaction: %v", err)
		return
	}

	_, err = tx.Exec(`INSERT INTO posts (title, content) VALUES ($1, $2)`,
		req.PostFormValue("title"),
		req.PostFormValue("content"),
	)
	if err != nil {
		log.Printf("Error: Failed to insert post: %v", err)
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Error: Failed to commit transaction: %v", err)
		return
	}

	http.Redirect(rw, req, "/", http.StatusFound)
}

func handleMain(rw http.ResponseWriter, req *http.Request) {
	type Post struct {
		Time    time.Time
		Title   string
		Content string
	}

	rows, err := DB.QueryContext(req.Context(), `SELECT * FROM posts ORDER BY ts DESC`)
	if err != nil {
		log.Printf("Error: Failed to query table %q: %v", "posts", err)
		return
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		err = rows.Scan(&p.Time, &p.Title, &p.Content)
		if err != nil {
			log.Printf("Error: Failed to scan row: %v", err)
			return
		}

		posts = append(posts, p)
	}
	err = rows.Err()
	if err != nil {
		log.Printf("Error: Failed to iterate rows: %v", err)
		return
	}

	err = Tmpl.ExecuteTemplate(rw, "main", posts)
	if err != nil {
		log.Printf("Error: Failed to execute template %q: %v", "main", err)
		return
	}
}

func initDB(path string) {
	db, err := sql.Open("ql", path)
	if err != nil {
		log.Printf("Error: Failed to open database %q: %v", path, err)
		os.Exit(1)
	}
	DB = db

	tx, err := db.Begin()
	if err != nil {
		log.Printf("Error: Failed to begin transaction: %v", err)
		os.Exit(1)
	}

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS posts (
		ts time DEFAULT now(),
		title string,
		content string,
)`)
	if err != nil {
		log.Printf("Error: Failed to create table %q: %v", "posts", err)
		os.Exit(1)
	}

	err = tx.Commit()
	if err != nil {
		log.Printf("Error: Failed to commit transaction: %v", err)
		os.Exit(1)
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %v [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
	}
	var flags struct {
		addr string
		db   string
	}
	flag.StringVar(&flags.addr, "addr", ":8080", "The address to listen on.")
	flag.StringVar(&flags.db, "db", "db.ql", "The database to use.")
	flag.Parse()

	initDB(flags.db)
	defer DB.Close()

	http.HandleFunc("/post", handlePost)
	http.HandleFunc("/", handleMain)

	log.Printf("Error: Server failed: %v\n", http.ListenAndServe(flags.addr, nil))
	os.Exit(1)
}
