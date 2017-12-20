package handlers

import (
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/projectriff/http-gateway/pkg/message"
	"github.com/projectriff/http-gateway/pkg/replies"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/Shopify/sarama.v1"
)

const CorrelationId = "correlationId"

// Creates an http handler that posts the http body as a message to Kafka, then waits
// for a message on a go channel it creates for a reply (this is expected to be set by the main thread) and sends
// that as an http response.
func ReplyHandler(producer sarama.AsyncProducer, repliesMap *replies.RepliesMap, traceContext TraceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		span, spanErr := traceContext.InitSpan()
		if spanErr != nil {
			log.Printf("Error initializing tracing span: %v", spanErr)
			return
		}
		defer span.Finish()

		topic := r.URL.Path[len("/requests/"):]
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		correlationId := uuid.NewV4().String()
		replyChan := make(chan message.Message)
		repliesMap.Put(correlationId, replyChan)

		msg := message.Message{Payload: b, Headers: make(map[string]interface{})}
		PropagateIncomingHeaders(r, &msg)
		msg.Headers[CorrelationId] = correlationId

		bytesOut, err := message.EncodeMessage(msg)
		if err != nil {
			log.Printf("Error encoding message: %v", err)
			return
		}
		kafkaMsg := &sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(bytesOut)}

		select {
		case producer.Input() <- kafkaMsg:
			select {
			case reply := <-replyChan:
				repliesMap.Delete(correlationId)
				p := reply.Payload
				PropagateOutgoingHeaders(&reply, w)
				w.Write(p.([]byte)) // TODO equivalent of Spring's HttpMessageConverter handling
			case <-time.After(time.Second * 60):
				repliesMap.Delete(correlationId)
				w.WriteHeader(404)
			}
		}
	}
}
