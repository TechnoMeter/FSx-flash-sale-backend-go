package handler

import (
    "embed"
    "net/http"
)

//go:embed index.html
var indexHTML embed.FS

func IndexPage(w http.ResponseWriter, r *http.Request) {
    content, _ := indexHTML.ReadFile("index.html")
    w.Header().Set("Content-Type", "text/html")
    w.Write(content)
}