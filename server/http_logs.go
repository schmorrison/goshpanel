// +build !wasm

package server

import (
	"fmt"
	"log"
	"net/http"
)

func httpError(w http.ResponseWriter, err error, code int) {
	http.Error(w, fmt.Sprintf("http error: %s", err), code)
	log.Printf("http error: %s: code %d", err, code)
}
