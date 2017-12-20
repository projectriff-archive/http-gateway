package handlers

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/projectriff/http-gateway/pkg/message"
	"gopkg.in/Shopify/sarama.v1"
)

// Creates an http handler that posts the http body as a message to Kafka, replying
// immediately with a successful http response
func MessageHandler(producer sarama.AsyncProducer, traceContext TraceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span, spanErr := traceContext.InitSpan()
		if spanErr != nil {
			log.Printf("Error initializing tracing span: %v", spanErr)
			return
		}
		defer span.Finish()

		topic := r.URL.Path[len("/messages/"):]
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		msg := message.Message{Payload: b, Headers: make(map[string]interface{})}
		PropagateIncomingHeaders(r, &msg)

		bytesOut, err := message.EncodeMessage(msg)
		if err != nil {
			log.Printf("Error encoding message: %v", err)
			return
		}
		kafkaMsg := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(bytesOut)}

		select {
		case producer.Input() <- kafkaMsg:
			w.Write([]byte("message published to topic: " + topic + "\n"))
		}

	}
}
