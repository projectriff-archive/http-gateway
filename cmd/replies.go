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
	"sync"

	"github.com/projectriff/function-sidecar/pkg/dispatcher"
)

// Type repliesMap implements a concurrent safe map of channels to send replies to, keyed by message correlationIds
type RepliesMap struct {
	m    map[string]chan<- dispatcher.Message
	lock sync.RWMutex
}

func (replies *RepliesMap) delete(key string) {
	replies.lock.Lock()
	defer replies.lock.Unlock()
	delete(replies.m, key)
}

func (replies *RepliesMap) get(key string) chan<- dispatcher.Message {
	replies.lock.RLock()
	defer replies.lock.RUnlock()
	return replies.m[key]
}

func (replies *RepliesMap) put(key string, value chan<- dispatcher.Message) {
	replies.lock.Lock()
	defer replies.lock.Unlock()
	replies.m[key] = value
}

func newRepliesMap() *RepliesMap {
	return &RepliesMap{make(map[string]chan<- dispatcher.Message), sync.RWMutex{}}
}
