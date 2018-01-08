package handlers

import (
	"log"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/projectriff/http-gateway/pkg/replies"
)

type SimpleCarrier struct {
	values map[string]string
}

// Set conforms to the TextMapWriter interface.
func (c SimpleCarrier) Set(key, val string) {
	c.values[key] = val
}

// ForeachKey conforms to the TextMapReader interface.
func (c SimpleCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range c.values {
		for _, v := range vals {
			if err := handler(k, string(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

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
