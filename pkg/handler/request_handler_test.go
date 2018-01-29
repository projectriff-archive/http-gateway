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

package handler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectriff/http-gateway/pkg/handler"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"github.com/projectriff/http-gateway/transport/mocktransport"
	"net/http/httptest"
	"strings"
	"net/http"
	"errors"
	"github.com/projectriff/http-gateway/pkg/replies_map"
	"os"
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"time"
)

type testSignal struct{}

func (*testSignal) String() string {
	return "test signal"
}

func (*testSignal) Signal() {}

var _ = Describe("RequestHandler", func() {

	const errorMessage = "doh!"

	var (
		mockProducer       *mocktransport.Producer
		mockConsumer       *mocktransport.Consumer
		mockResponseWriter *httptest.ResponseRecorder
		req                *http.Request
		testError          error
		repliesMap         *replies_map.RepliesMap
		signals            chan os.Signal
		testSig            *testSignal
		consumerMessages   chan dispatcher.Message
		producerErrors     chan error
		timeout            time.Duration
	)

	BeforeEach(func() {
		mockProducer = new(mocktransport.Producer)
		mockConsumer = new(mocktransport.Consumer)
		req = httptest.NewRequest("GET", "http://example.com", nil)
		req.URL.Path = "/requests/testtopic"
		mockResponseWriter = httptest.NewRecorder()
		testError = errors.New(errorMessage)
		repliesMap = replies_map.NewRepliesMap()
		timeout = time.Second * 60

		testSig = &testSignal{}
		signals = make(chan os.Signal, 1)

		consumerMessages = make(chan dispatcher.Message, 1)
		var cMsg <-chan dispatcher.Message = consumerMessages
		mockConsumer.On("Messages").Return(cMsg)

		producerErrors = make(chan error, 0)
		var pErr <-chan error = producerErrors
		mockProducer.On("Errors").Return(pErr)
	})

	AfterEach(func() {
		signals <- testSig
	})

	JustBeforeEach(func() {
		go handler.ReplyHandler(signals, mockConsumer, repliesMap, mockProducer)
		handler.RequestHandler(mockProducer, repliesMap, timeout)(mockResponseWriter, req)
	})

	Context("when the request URL is unexpected", func() {
		BeforeEach(func() {
			req.URL.Path = "/short"
		})

		It("should return a 404", func() {
			resp := mockResponseWriter.Result()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Context("when the request body cannot be read", func() {
		BeforeEach(func() {
			req.Body = &badReader{testError}
		})

		It("should reply with a suitable error response", func() {
			resp := mockResponseWriter.Result()
			body, _ := ioutil.ReadAll(resp.Body)

			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
			Expect(string(body)).To(HavePrefix(errorMessage))
		})
	})

	Context("when the request body can be read", func() {
		BeforeEach(func() {
			req.Body = ioutil.NopCloser(strings.NewReader("body"))
		})

		Context("when sending succeeds", func() {
			BeforeEach(func() {
				mockProducer.On("Send", mock.AnythingOfType("string"), mock.Anything).Run(func(args mock.Arguments) {
					msg, ok := args[1].(dispatcher.Message)
					Expect(ok).To(BeTrue())
					consumerMessages <- dispatcher.NewMessage([]byte(""), msg.Headers())
				}).Return(nil)
			})

			It("should send one message to the producer", func() {
				Expect(sendIndex(mockProducer)).To(BeNumerically(">", -1))
			})

			It("should send to the correct topic", func() {
				Expect(mockProducer.Calls[sendIndex(mockProducer)].Arguments[0]).To(Equal("testtopic"))
			})

			It("should send a message containing the correct body", func() {
				Expect(string(sentMessage(mockProducer).Payload())).To(Equal("body"))
			})

			It("should send a message with a correlation id header", func() {
				Expect(sentMessage(mockProducer).Headers()).To(HaveKey("correlationId"))
			})

			Context("when the request contains some headers that should be propagated and some that should not", func() {
				BeforeEach(func() {
					req.Header.Add("Content-Type", "text/plain")
					req.Header.Add("Accept", "application/json")
					req.Header.Add("Accept", "text/plain")
					req.Header.Add("Accept-Charset", "utf-8")
				})

				It("should send a message containing the correct headers", func() {
					headers := sentMessage(mockProducer).Headers()
					Expect(headers).To(HaveKeyWithValue("Content-Type", ConsistOf("text/plain")))
					Expect(headers).To(HaveKeyWithValue("Accept", ConsistOf("application/json", "text/plain")))
					Expect(headers).NotTo(HaveKey("Accept-Charset"))
				})
			})

			It("should reply with an OK response", func() {
				resp := mockResponseWriter.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when sending fails", func() {
			BeforeEach(func() {
				mockProducer.On("Send", mock.AnythingOfType("string"), mock.Anything).Return(testError)
			})

			It("should reply with a suitable error response", func() {
				resp := mockResponseWriter.Result()
				body, _ := ioutil.ReadAll(resp.Body)

				Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
				Expect(string(body)).To(HavePrefix(errorMessage))
			})
		})

		Context("when sending succeeds but the reply takes too long", func() {
			BeforeEach(func() {
				timeout = time.Nanosecond
				mockProducer.On("Send", mock.AnythingOfType("string"), mock.Anything).Run(func(args mock.Arguments) {
				}).Return(nil)
			})

			It("should time out with a 404", func() {
				resp := mockResponseWriter.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when sending succeeds but the producer reports an error before the reply comes back", func() {
			BeforeEach(func() {
				mockProducer.On("Send", mock.AnythingOfType("string"), mock.Anything).Run(func(args mock.Arguments) {
					msg, ok := args[1].(dispatcher.Message)
					Expect(ok).To(BeTrue())

					producerErrors <- testError

					consumerMessages <- dispatcher.NewMessage([]byte(""), msg.Headers())
				}).Return(nil)
			})

			It("should reply with OK", func() {
				resp := mockResponseWriter.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("when sending succeeds but the reply comes back with the wrong correlation id", func() {
			BeforeEach(func() {
				timeout = 10 * time.Millisecond
				mockProducer.On("Send", mock.AnythingOfType("string"), mock.Anything).Run(func(args mock.Arguments) {
					headers := make(dispatcher.Headers)
					headers["correlationId"] = []string{""}
					consumerMessages <- dispatcher.NewMessage([]byte(""), headers)
				}).Return(nil)
			})

			It("should time out with a 404", func() {
				resp := mockResponseWriter.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})

func sendIndex(mockProducer *mocktransport.Producer) int {
	index := -1
	for i := 0; i < len(mockProducer.Calls); i++ {
		if mockProducer.Calls[i].Method == "Send" {
			Expect(index).To(Equal(-1))
			index = i
		}
	}
	Expect(index).NotTo(Equal(-1))
	return index
}