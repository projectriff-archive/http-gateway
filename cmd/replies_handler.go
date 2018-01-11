package main

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"github.com/projectriff/function-sidecar/pkg/wireformat"
	uuid "github.com/satori/go.uuid"
)

const CorrelationId = "correlationId"

// Creates an http handler that posts the http body as a message to Kafka, then waits
// for a message on a go channel it creates for a reply (this is expected to be set by the main thread) and sends
// that as an http response.
func replyHandler(producer sarama.AsyncProducer, repliesMap *RepliesMap) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Path[len("/requests/"):]
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		correlationId := uuid.NewV4().String()
		replyChan := make(chan dispatcher.Message)
		repliesMap.put(correlationId, replyChan)

		msg := dispatcher.NewMessage(b, make(map[string][]string))
		propagateIncomingHeaders(r, msg)
		msg.Headers()[CorrelationId] = []string{correlationId}

		kafkaMsg, err := wireformat.ToKafka(msg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		kafkaMsg.Topic = topic

		select {
		case producer.Input() <- kafkaMsg:
			select {
			case reply := <-replyChan:
				repliesMap.delete(correlationId)
				propagateOutgoingHeaders(reply, w)
				w.Write(reply.Payload())
			case <-time.After(time.Second * 60):
				repliesMap.delete(correlationId)
				w.WriteHeader(404)
			}
		}
	}
}
