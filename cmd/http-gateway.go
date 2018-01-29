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
	"syscall"
	"time"
	"strings"
	"github.com/projectriff/http-gateway/transport"
	"github.com/projectriff/http-gateway/transport/kafka"
	"github.com/projectriff/http-gateway/pkg/handler"
	"github.com/projectriff/http-gateway/pkg/replies_map"
)

func startHttpServer(producer transport.Producer, replies *replies_map.RepliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/messages/", handler.MessageHandler(producer))
	http.HandleFunc("/requests/", handler.RequestHandler(producer, replies, time.Second * 60))
	http.HandleFunc("/application/status", handler.HealthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}

const CorrelationId = "correlationId"

func main() {
	// Trap signals to trigger a proper shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, os.Kill)

	// Key is correlationId, value is channel used to pass message received from main Kafka consumer loop
	replies := replies_map.NewRepliesMap()

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

	handler.ReplyHandler(signals, consumer, replies, producer)
	log.Println("Shutting Down...")
	timeout, c := context.WithTimeout(context.Background(), 1*time.Second)
	defer c()
	if err := srv.Shutdown(timeout); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}
}

func brokers() []string {
	return strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
}
