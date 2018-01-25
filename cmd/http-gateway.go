/*
 * Copyright 2017 the original author or authors.
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

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/satori/go.uuid"
	"strings"
	"github.com/projectriff/http-gateway/transport"
	"fmt"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"io/ioutil"
	"github.com/projectriff/http-gateway/transport/kafka"
)

const ContentType = "Content-Type"
const Accept = "Accept"
const CorrelationId = "correlationId"

var incomingHeadersToPropagate = [...]string{ContentType, Accept}
var outgoingHeadersToPropagate = [...]string{ContentType}

// Function messageHandler creates an http handler that sends the http body to the producer, replying
// immediately with a successful http response.
func messageHandler(producer transport.Producer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Path[len("/messages/"):]

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = producer.Send(topic, dispatcher.NewMessage(b, propagateIncomingHeaders(r)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "message published to topic: %s\n", topic)
	}
}

// Function replyHandler creates an http handler that sends the http body to the producer, then waits
// for a message on a go channel it creates for a reply (this is expected to be set by the main thread) and sends
// that as an http response.
func replyHandler(producer transport.Producer, replies *repliesMap) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Path[len("/requests/"):]

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		correlationId := uuid.NewV4().String() // entropy bottleneck?
		replyChan := make(chan dispatcher.Message)
		replies.put(correlationId, replyChan)

		headers := propagateIncomingHeaders(r)
		headers[CorrelationId] = []string{correlationId}

		err = producer.Send(topic, dispatcher.NewMessage(b, headers))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		select {
		case reply := <-replyChan:
			replies.delete(correlationId)
			propagateOutgoingHeaders(reply, w)
			w.Write(reply.Payload())
		case <-time.After(time.Second * 60):
			replies.delete(correlationId)
			w.WriteHeader(404)
		}
	}
}

func healthHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"UP"}`))
	}
}

func startHttpServer(producer transport.Producer, replies *repliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/messages/", messageHandler(producer))
	http.HandleFunc("/requests/", replyHandler(producer, replies))
	http.HandleFunc("/application/status", healthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}

func propagateIncomingHeaders(request *http.Request) dispatcher.Headers {
	header := make(dispatcher.Headers)
	for _, h := range incomingHeadersToPropagate {
		if vs, ok := request.Header[h]; ok {
			header[h] = vs
		}
	}
	return header
}

func propagateOutgoingHeaders(message dispatcher.Message, response http.ResponseWriter) {
	for _, h := range outgoingHeadersToPropagate {
		if vs, ok := message.Headers()[h]; ok {
			response.Header()[h] = vs
		}
	}
}

func main() {
	// Trap signals to trigger a proper shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, os.Kill)

	// Key is correlationId, value is channel used to pass message received from main Kafka consumer loop
	replies := newRepliesMap()

	brokers := brokers()
	producer, err := kafka.NewKafkaProducer(brokers)
	if err != nil {
		panic(err)
	}
	defer producer.Close()

	consumer, err := kafka.NewKafkaConsumer(brokers, "gateway", []string{"replies"})
	if err != nil {
		panic(err)
	}
	defer consumer.Close()

	srv := startHttpServer(producer, replies)

	for {
		select {
		case <-signals:
			log.Println("Shutting Down...")
			timeout, c := context.WithTimeout(context.Background(), 1*time.Second)
			defer c()
			if err := srv.Shutdown(timeout); err != nil {
				panic(err) // failure/timeout shutting down the server gracefully
			}
			return
		case msg, ok := <-consumer.Messages():
			if ok {
				correlationId, ok := msg.Headers()[CorrelationId]
				if ok {
					c := replies.get(correlationId[0])
					if c != nil {
						log.Printf("Sending reply %v\n", msg)
						c <- msg
					} else {
						log.Printf("Did not find communication channel for correlationId %v. Timed out?", correlationId)
					}
				}
			}
		case err := <-producer.Errors():
			log.Println("Failed to send message ", err)
		}
	}
}

func brokers() []string {
	return strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
}

// Type repliesMap implements a concurrent safe map of channels to send replies to, keyed by message correlationIds
type repliesMap struct {
	m    map[string]chan<- dispatcher.Message
	lock sync.RWMutex
}

func (replies *repliesMap) delete(key string) {
	replies.lock.Lock()
	defer replies.lock.Unlock()
	delete(replies.m, key)
}

func (replies *repliesMap) get(key string) chan<- dispatcher.Message {
	replies.lock.RLock()
	defer replies.lock.RUnlock()
	return replies.m[key]
}

func (replies *repliesMap) put(key string, value chan<- dispatcher.Message) {
	replies.lock.Lock()
	defer replies.lock.Unlock()
	replies.m[key] = value
}

func newRepliesMap() *repliesMap {
	return &repliesMap{make(map[string]chan<- dispatcher.Message), sync.RWMutex{}}
}
