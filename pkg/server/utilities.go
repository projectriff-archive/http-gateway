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

package server

import (
	"fmt"
	"net/http"
)

func parseTopic(r *http.Request, path string) (string, error) {
	if len(r.URL.Path) < len(path) || r.URL.Path[:len(path)] != path {
		return "", fmt.Errorf("Unknown path: %s", r.URL.Path)
	}
	return r.URL.Path[len(path):], nil
}