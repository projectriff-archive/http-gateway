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
	"os"
	"os/signal"
	"syscall"
	"strings"
	"github.com/projectriff/http-gateway/transport/kafka"
	"github.com/projectriff/http-gateway/pkg/handler"
	"log"
	"time"
)

func main() {

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

	gw := handler.New(8080, producer, consumer, 60 * time.Second)

	closeCh := make(chan struct{})
	gw.Run(closeCh)

	// Wait for shutdown
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, os.Kill)
	<-signals
	log.Println("Shutting Down...")
	closeCh <- struct{}{}


}

func brokers() []string {
	return strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
}
