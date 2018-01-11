package main

import (
	"log"
	"net/http"

	"github.com/Shopify/sarama"
)

func startHttpServer(producer sarama.AsyncProducer, repliesMap *RepliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/messages/", messageHandler(producer))
	http.HandleFunc("/requests/", replyHandler(producer, repliesMap))
	http.HandleFunc("/application/status", healthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}
