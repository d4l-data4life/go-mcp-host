package handlers

import (
	"net/http"
)

//WriteHTTPErrorCode writes given HTTP Code to the HTTP response and provides explanation - for errors
func WriteHTTPErrorCode(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}

//WriteHTTPCode writes given HTTP Code to the HTTP response
func WriteHTTPCode(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}
