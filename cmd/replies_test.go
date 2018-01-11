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
	"testing"

	"github.com/projectriff/function-sidecar/pkg/dispatcher"
)

func TestPutAndGet(t *testing.T) {
	testChannel := make(chan dispatcher.Message)
	repliesMap := newRepliesMap()
	repliesMap.put("testkey1", testChannel)
	returnedChannel := repliesMap.get("testkey1")
	if testChannel != returnedChannel {
		t.Fatal("Expected identical channels: ", testChannel, returnedChannel)
	}
}

func TestDelete(t *testing.T) {
	testChannel := make(chan dispatcher.Message)
	repliesMap := newRepliesMap()
	repliesMap.put("testkey2", testChannel)
	repliesMap.delete("testkey2")

	returnedChannel := repliesMap.get("testkey2")
	if returnedChannel != nil {
		t.Fatal("Expected nil but got a channel: ", returnedChannel)
	}
}
