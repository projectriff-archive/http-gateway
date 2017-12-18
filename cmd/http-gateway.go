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

	"github.com/bsm/sarama-cluster"
	"github.com/projectriff/http-gateway/pkg/handlers"
	"github.com/projectriff/http-gateway/pkg/message"
	"github.com/projectriff/http-gateway/pkg/replies"
	"gopkg.in/Shopify/sarama.v1"
)

func startHttpServer(producer sarama.AsyncProducer, repliesMap *replies.RepliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	http.HandleFunc("/messages/", handlers.MessageHandler(producer))
	http.HandleFunc("/requests/", handlers.ReplyHandler(producer, repliesMap))
	http.HandleFunc("/application/status", handlers.HealthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}

func main() {
	// Trap signals to trigger a proper shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, os.Kill)

	// Key is correlationId, value is channel used to pass message received from main Kafka consumer loop
	repliesMap := replies.NewRepliesMap()

	brokers := []string{os.Getenv("SPRING_CLOUD_STREAM_KAFKA_BINDER_BROKERS")}
	producer, err := sarama.NewAsyncProducer(brokers, nil)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	consumerConfig := makeConsumerConfig()
	consumer, err := cluster.NewConsumer(brokers, "gateway", []string{"replies"}, consumerConfig)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()
	if consumerConfig.Consumer.Return.Errors {
		go consumeErrors(consumer)
	}
	if consumerConfig.Group.Return.Notifications {
		go consumeNotifications(consumer)
	}

	srv := startHttpServer(producer, repliesMap)

MainLoop:
	for {
		select {
		case <-signals:
			log.Println("Shutting Down...")
			timeout, c := context.WithTimeout(context.Background(), 1*time.Second)
			defer c()
			if err := srv.Shutdown(timeout); err != nil {
				panic(err) // failure/timeout shutting down the server gracefully
			}
			break MainLoop
		case msg, ok := <-consumer.Messages():
			if ok {
				messageWithHeaders, err := message.ExtractMessage(msg.Value)
				if err != nil {
					log.Println("Failed to extract message ", err)
					break
				}
				correlationId, ok := messageWithHeaders.Headers["correlationId"].(string)
				if ok {
					c := repliesMap.Get(correlationId)
					if c != nil {
						log.Printf("Sending %v\n", messageWithHeaders)
						c <- messageWithHeaders
						consumer.MarkOffset(msg, "") // mark message as processed
					} else {
						log.Printf("Did not find communication channel for correlationId %v. Timed out?", correlationId)
						consumer.MarkOffset(msg, "") // mark message as processed
					}
				}
			}

		case err := <-producer.Errors():
			log.Println("Failed to produce kafka message ", err)
		}
	}
}

func makeConsumerConfig() *cluster.Config {
	consumerConfig := cluster.NewConfig()
	consumerConfig.Consumer.Return.Errors = true
	consumerConfig.Group.Return.Notifications = true
	return consumerConfig
}

func consumeNotifications(consumer *cluster.Consumer) {
	for ntf := range consumer.Notifications() {
		log.Printf("Rebalanced: %+v\n", ntf)
	}
}
func consumeErrors(consumer *cluster.Consumer) {
	for err := range consumer.Errors() {
		log.Printf("Error: %s\n", err.Error())
	}
}
