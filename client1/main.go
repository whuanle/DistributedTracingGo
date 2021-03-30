package main

import (
	"bufio"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"io"
	"net/http"
	"os"
)

func CreateTracer(servieName string) (opentracing.Tracer, io.Closer, error) {
	var cfg = jaegercfg.Configuration{
		ServiceName: servieName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
			// 按实际情况替换你的 ip
			CollectorEndpoint: "http://180.102.130.181:14268/api/traces",
		},
	}

	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)
	return tracer, closer, err
}

func main() {
	tracer, closer, _ := CreateTracer("UserinfoService")
	// 创建第一个 span A
	parentSpan := tracer.StartSpan("A")
	// 调用其它服务
	GetUserInfo(tracer, parentSpan)
	// 结束 A
	parentSpan.Finish()
	// 结束当前 tracer
	closer.Close()

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadByte()
}

// 请求远程服务，获得用户信息
func GetUserInfo(tracer opentracing.Tracer, parentSpan opentracing.Span) {
	// 继承上下文关系，创建子 span
	childSpan := tracer.StartSpan(
		"B",
		opentracing.ChildOf(parentSpan.Context()),
	)

	url := "http://127.0.0.1:8081/Get?username=痴者工良"
	req, _ := http.NewRequest("GET", url, nil)
	// 设置 tag，这个 tag 我们后面讲
	ext.SpanKindRPCClient.Set(childSpan)
	ext.HTTPUrl.Set(childSpan, url)
	ext.HTTPMethod.Set(childSpan, "GET")
	tracer.Inject(childSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	resp, _ := http.DefaultClient.Do(req)
	_ = resp // 丢掉
	defer childSpan.Finish()
}

//func x(ctx context.Context) {
//	span, ctx := opentracing.StartSpanFromContext(ctx, "mytest")
//	ext.SamplingPriority.Set(span, 1)
//	defer span.Finish()
//
//	span.LogFields(
//		log.String("event", "soft error"),
//		log.String("type", "cache timeout"),
//		log.Int("waited.millis", 1500),
//	)
//}
