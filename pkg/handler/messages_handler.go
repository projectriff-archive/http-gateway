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
	"github.com/projectriff/function-sidecar/pkg/dispatcher"
	"fmt"
	"net/http"
)

const (
	messagePath = "/messages/"
	ContentType = "Content-Type"
	Accept      = "Accept"
)

var incomingHeadersToPropagate = [...]string{ContentType, Accept}

// Function messageHandler is an http handler that sends the http body to the producer, replying
// immediately with a successful http response.
func(g *gateway) messagesHandler(w http.ResponseWriter, r *http.Request) {
	topic, err := parseTopic(r, messagePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = g.producer.Send(topic, dispatcher.NewMessage(b, propagateIncomingHeaders(r)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Message published to topic: %s\n", topic)
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

