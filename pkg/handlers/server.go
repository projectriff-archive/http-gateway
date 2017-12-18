package handlers

import (
	"log"
	"net/http"

	"github.com/projectriff/http-gateway/pkg/replies"
	"gopkg.in/Shopify/sarama.v1"
)

func StartHttpServer(producer sarama.AsyncProducer, repliesMap *replies.RepliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/messages/", MessageHandler(producer))
	http.HandleFunc("/requests/", ReplyHandler(producer, repliesMap))
	http.HandleFunc("/application/status", HealthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}
