package main

import (
	"anahita-libs/site"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"webshell/config"
	"webshell/shell"

	"anahita-libs/data/services"
)

const webPort = "80"

func main() {
	config := config.NewConfig()
	if err := config.Validate(); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// Without the app there is nothing to send: it does not start.
	page, err := os.ReadFile(filepath.Join(config.WebBuildDir, "index.html"))
	if err != nil {
		log.Fatalf("no web app at %s: %v", config.WebBuildDir, err)
	}
	template, err := shell.NewTemplate(string(page))
	if err != nil {
		log.Fatalf("%s/index.html: %v", config.WebBuildDir, err)
	}

	server := &shell.Server{
		Template: template,
		Reader: &shell.GraphReader{
			Nodes:  services.NewGraphNodeGRPC(config),
			Actors: services.NewActorsGRPC(config),
			Media:  services.NewMediaGRPC(config),
		},
		Signer:    &shell.FileSigner{Files: services.NewFilesGRPC(config)},
		Dir:       config.WebBuildDir,
		PublicURL: config.PublicWebURL,
		APIURL:    site.FromEnv().APIURL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	mux.Handle("/", server)

	log.Printf("web shell: sending %s on :%s", config.WebBuildDir, webPort)
	httpServer := &http.Server{
		Addr:              ":" + webPort,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(httpServer.ListenAndServe())
}
