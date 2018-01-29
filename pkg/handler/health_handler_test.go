package handler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/projectriff/http-gateway/pkg/handler"
	"net/http/httptest"
	"net/http"
)

var _ = Describe("HealthHandler", func() {
	var (
		mockResponseWriter *httptest.ResponseRecorder
		req                *http.Request
	)

	BeforeEach(func() {
		req = httptest.NewRequest("GET", "http://example.com", nil)
		req.URL.Path = "/health"
		mockResponseWriter = httptest.NewRecorder()

	})

	JustBeforeEach(func() {
		handler.HealthHandler()(mockResponseWriter, req)
	})

	It("should return Ok", func() {
		resp := mockResponseWriter.Result()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
