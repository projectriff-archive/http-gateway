package handlers

import (
	"net/http"

	"github.com/projectriff/http-gateway/pkg/message"
)

const ContentType = "Content-Type"
const Accept = "Accept"

var incomingHeadersToPropagate = [...]string{ContentType, Accept}
var outgoingHeadersToPropagate = [...]string{ContentType}

func PropagateIncomingHeaders(request *http.Request, message *message.Message) {
	for _, h := range incomingHeadersToPropagate {
		if v, ok := request.Header[h]; ok {
			message.Headers[h] = v[0]
		}
	}
}

func PropagateOutgoingHeaders(message *message.Message, response http.ResponseWriter) {
	for _, h := range outgoingHeadersToPropagate {
		if v, ok := message.Headers[h]; ok {
			switch value := v.(type) {
			case string:
				response.Header()[h] = []string{value}
			case []string:
				response.Header()[h] = value
			}
		}
	}
}
