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

package server_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/projectriff/http-gateway/pkg/server"
	"github.com/projectriff/message-transport/pkg/message"
	"github.com/projectriff/message-transport/pkg/transport/mocktransport"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("HTTP Gateway", func() {
	var (
		gw               server.Gateway
		mockProducer     *mocktransport.Producer
		mockConsumer     *mocktransport.Consumer
		port             int
		timeout          time.Duration
		done             chan struct{}
		consumerMessages chan message.Message
		producerErrors   chan error
	)

	BeforeEach(func() {
		mockProducer = new(mocktransport.Producer)
		mockConsumer = new(mocktransport.Consumer)

		consumerMessages = make(chan message.Message, 1)
		var cMsg <-chan message.Message = consumerMessages
		mockConsumer.On("Messages").Return(cMsg)

		producerErrors = make(chan error, 0)
		var pErr <-chan error = producerErrors
		mockProducer.On("Errors").Return(pErr)

		timeout = 6 * time.Second
		done = make(chan struct{})
	})

	JustBeforeEach(func() {
		port = 1024 + rand.Intn(32768-1024)
		gw = server.New(port, mockProducer, mockConsumer, timeout)
	})

	AfterEach(func() {
		done <- struct{}{}
	})

	It("should request/reply OK", func() {
		gw.Run(done)

		mockProducer.On("Send", "foo", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			defer GinkgoRecover()
			msg := args[1].(message.Message)
			consumerMessages <- message.NewMessage([]byte("hello "+string(msg.Payload())),
				message.Headers{server.CorrelationId: msg.Headers()[server.CorrelationId],
					"Content-Type": []string{"bag/plastic"},
				})
			Expect(msg.Headers()["Content-Type"]).To(Equal([]string{"text/solid"}))
			Expect(msg.Headers()["Not-Propagated-Header"]).To(BeNil())
		})

		resp := doRequest(port, "foo", bytes.NewBufferString("world"), "Content-Type", "text/solid", "Not-Propagated-Header", "secret")

		b := make([]byte, 11)
		resp.Body.Read(b)

		Expect(b).To(Equal([]byte("hello world")))
		Expect(resp.Header.Get(server.CorrelationId)).To(BeZero())
		Expect(resp.Header.Get("Content-Type")).To(Equal("bag/plastic"))

		defer resp.Body.Close()

	})

	It("should accept messages and fire&forget", func() {
		gw.Run(done)

		mockProducer.On("Send", "bar", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			defer GinkgoRecover()
			msg := args[1].(message.Message)
			Expect(msg.Payload()).To(Equal([]byte("world")))
			Expect(msg.Headers()["Content-Type"]).To(Equal([]string{"text/solid"}))
			Expect(msg.Headers()["Not-Propagated-Header"]).To(BeNil())
		})

		resp := doMessage(port, "bar", bytes.NewBufferString("world"), "Content-Type", "text/solid", "Not-Propagated-Header", "secret")

		Expect(resp.StatusCode).To(Equal(200))

		defer resp.Body.Close()

	})
})

func doRequest(port int, topic string, body io.Reader, headerKV ...string) *http.Response {
	return post(port, "/requests/"+topic, body, headerKV...)
}

func doMessage(port int, topic string, body io.Reader, headerKV ...string) *http.Response {
	return post(port, "/messages/"+topic, body, headerKV...)
}

func post(port int, path string, body io.Reader, headerKV ...string) *http.Response {
	client := http.Client{}
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%v%v", port, path), body)
	for i := 0; i < len(headerKV); i += 2 {
		req.Header.Add(headerKV[i], headerKV[i+1])
	}
	resp, _ := client.Do(req)
	return resp
}
