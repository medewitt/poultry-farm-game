package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("./docs"))
	http.Handle("/", fs)

	// Add the correct MIME type for WebAssembly files
	http.HandleFunc("/main.wasm", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeFile(w, r, "./docs/main.wasm")
	})

	log.Println("Listening on http://localhost:8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
