/*
 * Copyright 2018 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package handler

import (
	"io/ioutil"
	"log"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"time"
	"github.com/projectriff/http-gateway/transport"
	"net/http"
	"github.com/satori/go.uuid"
	"github.com/projectriff/http-gateway/pkg/replies_map"
	"os"
)

const (
	CorrelationId = "correlationId"
	requestPath   = "/requests/"
)

var outgoingHeadersToPropagate = [...]string{ContentType}

// Function RequestHandler creates an http handler that sends the http body to the producer, then waits
// for a message on a go channel it creates for a reply (this is expected to be set by the main thread) and sends
// that as an http response.
func RequestHandler(producer transport.Producer, replies *replies_map.RepliesMap, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topic, err := parseTopic(r, requestPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		correlationId := uuid.NewV4().String() // entropy bottleneck?
		replyChan := make(chan dispatcher.Message)
		replies.Put(correlationId, replyChan)

		headers := propagateIncomingHeaders(r)
		headers[CorrelationId] = []string{correlationId}

		err = producer.Send(topic, dispatcher.NewMessage(b, headers))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		select {
		case reply := <-replyChan:
			replies.Delete(correlationId)
			propagateOutgoingHeaders(reply, w)
			w.Write(reply.Payload())
		case <-time.After(timeout):
			replies.Delete(correlationId)
			w.WriteHeader(404)
		}
	}
}

func propagateOutgoingHeaders(message dispatcher.Message, response http.ResponseWriter) {
	for _, h := range outgoingHeadersToPropagate {
		if vs, ok := message.Headers()[h]; ok {
			response.Header()[h] = vs
		}
	}
}

func ReplyHandler(signals chan os.Signal, consumer transport.Consumer, replies *replies_map.RepliesMap, producer transport.Producer) {
	consumerMessages := consumer.Messages()
	producerErrors := producer.Errors()
	for {
		select {
		case <-signals:
			return
		case msg, ok := <-consumerMessages:
			if ok {
				correlationId, ok := msg.Headers()[CorrelationId]
				if ok {
					c := replies.Get(correlationId[0])
					if c != nil {
						log.Printf("Sending reply %v\n", msg)
						c <- msg
					} else {
						log.Printf("Did not find communication channel for correlationId %v. Timed out?", correlationId)
					}
				}
			}
		case err := <-producerErrors:
			log.Println("Failed to send message ", err)
		}
	}
}
