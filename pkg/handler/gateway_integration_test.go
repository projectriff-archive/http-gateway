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

package handler_test

import (
	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
	"github.com/projectriff/http-gateway/pkg/handler"
	"github.com/projectriff/http-gateway/transport/mocktransport"
	"time"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"math/rand"
	"bytes"
	"log"
	"net/http"
	"fmt"
)

var _ = FDescribe("HTTP Gateway", func() {
	var (
		gw           handler.Gateway
		mockProducer *mocktransport.Producer
		mockConsumer *mocktransport.Consumer
		port         int
		timeout      time.Duration
		done chan struct{}
		consumerMessages   chan dispatcher.Message
		producerErrors     chan error
	)

	BeforeEach(func() {
		mockProducer = new(mocktransport.Producer)
		mockConsumer = new(mocktransport.Consumer)

		consumerMessages = make(chan dispatcher.Message, 1)
		var cMsg <-chan dispatcher.Message = consumerMessages
		mockConsumer.On("Messages").Return(cMsg)

		producerErrors = make(chan error, 0)
		var pErr <-chan error = producerErrors
		mockProducer.On("Errors").Return(pErr)

		timeout = 60 * time.Second
		done = make(chan struct{})
	})

	JustBeforeEach(func() {
		port = 1024 + rand.Intn(32768 - 1024)
		gw = handler.New(port, mockProducer, mockConsumer, timeout)
	})

	AfterEach(func() {
		done <- struct{}{}
	})

	It("should return Ok", func() {
		gw.Run(done)
//		resp := mockResponseWriter.Result()
//		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		client := http.Client{
			Timeout: time.Duration(60 * time.Second),
		}
		req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%v", port), bytes.NewBufferString("hello"))
		resp, err := client.Do(req)
		defer resp.Body.Close()

	})
})
