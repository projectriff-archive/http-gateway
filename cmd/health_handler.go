package main

import (
	"net/http"
)

func healthHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"UP"}`))
	}
}
