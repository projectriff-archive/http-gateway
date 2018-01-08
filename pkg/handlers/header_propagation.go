package handlers

import (
	"net/http"

	"github.com/projectriff/function-sidecar/pkg/dispatcher"
)

const ContentType = "Content-Type"
const Accept = "Accept"

var incomingHeadersToPropagate = [...]string{ContentType, Accept}
var outgoingHeadersToPropagate = [...]string{ContentType}

func PropagateIncomingHeaders(request *http.Request, message dispatcher.Message) {
	for _, h := range incomingHeadersToPropagate {
		if v, ok := request.Header[h]; ok {
			(message.Headers())[h] = v
		}
	}
}

func PropagateOutgoingHeaders(message dispatcher.Message, response http.ResponseWriter) {
	for _, h := range outgoingHeadersToPropagate {
		if v, ok := message.Headers()[h]; ok {
			response.Header()[h] = v
		}
	}
}
