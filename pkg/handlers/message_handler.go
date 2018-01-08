package handlers

import (
	"io/ioutil"
	"net/http"

	"github.com/Shopify/sarama"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"github.com/projectriff/function-sidecar/pkg/wireformat"
)

// Creates an http handler that posts the http body as a message to Kafka, replying
// immediately with a successful http response
func MessageHandler(producer sarama.AsyncProducer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Path[len("/messages/"):]
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg := dispatcher.NewMessage(b, make(map[string][]string))
		PropagateIncomingHeaders(r, msg)

		kafkaMsg, err := wireformat.ToKafka(msg)
		kafkaMsg.Topic = topic
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		select {
		case producer.Input() <- kafkaMsg:
			w.Write([]byte("message published to topic: " + topic + "\n"))
		}

	}
}
