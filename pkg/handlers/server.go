package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/projectriff/http-gateway/pkg/replies"
	"gopkg.in/Shopify/sarama.v1"
)

type SimpleCarrier struct {
	values map[string]string
}

// Set conforms to the TextMapWriter interface.
func (c SimpleCarrier) Set(key, val string) {
	c.values[key] = val
}

// ForeachKey conforms to the TextMapReader interface.
func (c SimpleCarrier) ForeachKey(handler func(key, val string) error) error {
	for k, vals := range c.values {
		for _, v := range vals {
			if err := handler(k, string(v)); err != nil {
				return err
			}
		}
	}
	return nil
}

type TraceContext interface {
	Close() error
	InitSpan() (FinishableSpan, error)
}

type FinishableSpan interface {
	Finish()
}

type HTTPTraceContext struct {
	Collector zipkintracer.Collector
	Tracer    opentracing.Tracer
	Span      opentracing.Span
}

type NoOpTraceContext struct{}
type NoOpSpan struct{}

func (c *HTTPTraceContext) Close() error {
	return c.Collector.Close()
}

func (c *HTTPTraceContext) InitSpan() (FinishableSpan, error) {
	span := c.Tracer.StartSpan("http-span") //TODO: improve naming

	return span, c.Tracer.Inject(span.Context(), opentracing.TextMap, SimpleCarrier{values: make(map[string]string)})
}

func (c *NoOpTraceContext) Close() error {
	return nil
}

func (c *NoOpTraceContext) InitSpan() (FinishableSpan, error) {
	return &NoOpSpan{}, nil
}

func (c *NoOpSpan) Finish() {}

func StartHttpServer(producer sarama.AsyncProducer, repliesMap *replies.RepliesMap) *http.Server {
	srv := &http.Server{Addr: ":8080"}

	traceContext, traceErr := buildTraceContext()
	if traceErr != nil {
		panic(traceErr)
	}
	http.HandleFunc("/messages/", MessageHandler(producer, traceContext))
	http.HandleFunc("/requests/", ReplyHandler(producer, repliesMap, traceContext))
	http.HandleFunc("/application/status", HealthHandler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("Httpserver: ListenAndServe() error: %s", err)
		}
	}()

	log.Printf("Listening on %v", srv.Addr)
	return srv
}

func buildTraceContext() (TraceContext, error) {

	gwConfigStr := os.Getenv("GATEWAY_CONFIG")
	if gwConfigStr == "" {
		return &NoOpTraceContext{}, nil
	}

	var gwConfig map[string]string
	err := json.Unmarshal([]byte(gwConfigStr), &gwConfig)
	if err != nil {
		panic(err)
	}

	zipkinUrl := gwConfig["gw.trace.zipkin.url"]
	if zipkinUrl != "" {
		serviceName := gwConfig["gw.trace.servicename"]
		if serviceName == "" {
			serviceName = "default-service"
		}
		return buildHTTPTraceContext(zipkinUrl, serviceName)
	}

	return &NoOpTraceContext{}, nil
}

func buildHTTPTraceContext(zipkinUrl string, serviceName string) (TraceContext, error) {
	zipkinCollector, zipkinInitErr := zipkintracer.NewHTTPCollector(zipkinUrl + "/api/v1/spans")
	if zipkinInitErr != nil {
		panic(zipkinInitErr)
	}

	zipkinRecorder := zipkintracer.NewRecorder(zipkinCollector, true, "0.0.0.0:0", serviceName)
	zipkinTracer, tracerErr := zipkintracer.NewTracer(zipkinRecorder, zipkintracer.ClientServerSameSpan(true), zipkintracer.TraceID128Bit(true))
	if tracerErr != nil {
		panic(tracerErr)
	}

	opentracing.InitGlobalTracer(zipkinTracer)
	return &HTTPTraceContext{Collector: zipkinCollector, Tracer: zipkinTracer}, nil
}
