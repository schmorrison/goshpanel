// +build !wasm

package server

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

func serve() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4674"
	}

	r := mux.NewRouter()

	//r.HandleFunc("/files/", homePageRender)
	//r.HandleFunc("/database/", homePageRender)
	//r.HandleFunc("/domain/", homePageRender)
	//r.HandleFunc("/email/", homePageRender)
	//r.HandleFunc("/logging/", homePageRender)
	//r.HandleFunc("/security/", homePageRender)
	//r.HandleFunc("/advanced/", homePageRender)
	//r.HandleFunc("/", homePageRender)

	err := http.ListenAndServe(fmt.Sprintf(":%s", port), r)

	if err != nil {
		log.Fatalf("server crashed: %s", err)
		os.Exit(1)
	}
}
