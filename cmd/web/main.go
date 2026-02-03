package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	port := flag.String("port", "3000", "HTTP server port")
	flag.Parse()

	// Serve static files from web directory
	fs := http.FileServer(http.Dir("web"))
	http.Handle("/", fs)

	log.Printf("Web server starting on port %s", *port)
	log.Printf("Open http://localhost:%s in your browser", *port)
	
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
