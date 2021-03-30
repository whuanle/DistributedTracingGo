### 从何说起

之前参加柠檬大佬的训练营(免费白嫖)，在大佬的指导下我们技术蒸蒸日上，然后作业我们需要实现一个 Jaeger 后端，笔者采用 .NET + MongoDB 来实现(大佬说用C#写的扣10分，呜呜呜...)，C# 版本的实现项目地址[https://github.com/whuanle/DistributedTracing](https://github.com/whuanle/DistributedTracing)，项目支持 Jaeger Collector、Query 等。

现在笔者开始转 Go 语言，所以开始 Go 重新实现一次，下一篇文章将完整介绍如何实现一个 Jaeger Collector。在这篇文章，我们可以先学习 Jaeger client Go 的使用方法，以及 Jaeger Go 的一些概念。

在此之前，建议读者稍微看一下 [分布式链路追踪框架的基本实现原理](https://www.cnblogs.com/whuanle/p/14321107.html) 这篇文章，需要了解 Dapper 论文和一些 Jaeger 的概念。

接下来我们将一步步学习 Go 中的一些技术，后面慢慢展开 Jaeger Client。



### Jaeger

OpenTracing 是开放式分布式追踪规范，OpenTracing API 是一致，可表达，与供应商无关的API，用于分布式跟踪和上下文传播。

OpenTracing 的客户端库以及规范，可以到 Github 中查看：https://github.com/opentracing/

Jaeger 是 Uber 开源的分布式跟踪系统，详细的介绍可以自行查阅资料。



### 部署 Jaeger

这里我们需要部署一个 Jaeger 实例，以供微服务以及后面学习需要。

使用 Docker 部署很简单，只需要执行下面一条命令即可：

```shell
docker run -d -p 5775:5775/udp -p 16686:16686 -p 14250:14250 -p 14268:14268 jaegertracing/all-in-one:latest
```

访问 16686 端口，即可看到 UI 界面。

后面我们生成的链路追踪信息会推送到此服务，而且可以通过 Jaeger UI 查询这些追踪信息。

![JaegerUI](https://img2020.cnblogs.com/blog/1315495/202101/1315495-20210109224307467-2054331430.png)

![JaegerUI](https://www.whuanle.cn/wp-content/uploads/2021/01/image-1610203390774.png)



### 从示例了解 Jaeger Client Go

这里，我们主要了解一些 Jaeger Client 的接口和结构体，了解一些代码的使用。

为了让读者方便了解 Trace、Span 等，可以看一下这个 Json 的大概结构：

```json
        {
            "traceID": "2da97aa33839442e",
            "spans": [
                {
                    "traceID": "2da97aa33839442e",
                    "spanID": "ccb83780e27f016c",
                    "flags": 1,
                    "operationName": "format-string",
                    "references": [...],
                    "tags": [...],
                    "logs": [...],
                    "processID": "p1",
                    "warnings": null
                },
                ... ...
            ],
            "processes": {
                "p1": {
                    "serviceName": "hello-world",
                    "tags": [...]
                },
                "p2": ...,
            "warnings": null
        }
```



创建一个 client1 的项目，然后引入 Jaeger client  包。

```shell
go get -u github.com/uber/jaeger-client-go/
```

然后引入包

```
import (
	"github.com/uber/jaeger-client-go"
)
```



### 了解 trace、span

链路追踪中的一个进程使用一个 trace 实例标识，每个服务或函数使用一个 span 标识，jaeger 包中有个函数可以创建空的 trace：

```go
tracer := opentracing.GlobalTracer()	// 生产中不要使用
```

然后就是调用链中，生成父子关系的 Span：

```go
func main() {
	tracer := opentracing.GlobalTracer()
	// 创建第一个 span A
	parentSpan := tracer.StartSpan("A")
    defer parentSpan.Finish()		// 可手动调用 Finish()

}
func B(tracer opentracing.Tracer,parentSpan opentracing.Span){
	// 继承上下文关系，创建子 span
	childSpan := tracer.StartSpan(
		"B",
		opentracing.ChildOf(parentSpan.Context()),
		)
	defer childSpan.Finish()	// 可手动调用 Finish()
}
```

每个 span 表示调用链中的一个结点，每个结点都需要明确父 span。

现在，我们知道了，如何生成 `trace{span1,span2}`，且 `span1 -> span2` 即 span1 调用 span2，或 span1 依赖于 span2。 



### tracer 配置

由于服务之间的调用是跨进程的，每个进程都有一些特点的标记，为了标识这些进程，我们需要在上下文间、span 携带一些信息。

例如，我们在发起请求的第一个进程中，配置 trace，配置服务名称等。

```go
// 引入 jaegercfg "github.com/uber/jaeger-client-go/config"
	cfg := jaegercfg.Configuration{
		ServiceName: "client test", // 对其发起请求的的调用链，叫什么服务
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

```

Sampler 是客户端采样率配置，可以通过 `sampler.type` 和 `sampler.param` 属性选择采样类型，后面详细聊一下。

Reporter 可以配置如何上报，后面独立小节聊一下这个配置。



传递上下文的时候，我们可以打印一些日志：

```go
	jLogger := jaegerlog.StdLogger
```

配置完毕后就可以创建 tracer 对象了：

```go
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)
	
	defer closer.Close()
	if err != nil {
	}
```

完整代码如下：

```go
import (
    "github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
)

func main() {

	cfg := jaegercfg.Configuration{
		ServiceName: "client test", // 对其发起请求的的调用链，叫什么服务
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}

	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)

	defer closer.Close()
	if err != nil {
	}

	// 创建第一个 span A
	parentSpan := tracer.StartSpan("A")
	defer parentSpan.Finish()

	B(tracer,parentSpan)
}

func B(tracer opentracing.Tracer, parentSpan opentracing.Span) {
	// 继承上下文关系，创建子 span
	childSpan := tracer.StartSpan(
		"B",
		opentracing.ChildOf(parentSpan.Context()),
	)
	defer childSpan.Finish()
}
```

启动后：

```
2021/03/30 11:14:38 Initializing logging reporter
2021/03/30 11:14:38 Reporting span 689df7e83255d05d:75668e8ed5ec61da:689df7e83255d05d:1
2021/03/30 11:14:38 Reporting span 689df7e83255d05d:689df7e83255d05d:0000000000000000:1
2021/03/30 11:14:38 DEBUG: closing tracer
2021/03/30 11:14:38 DEBUG: closing reporter
```



### Sampler 配置

sampler 配置代码示例：

```go
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		}
```

这个 sampler 可以使用 `jaegercfg.SamplerConfig`，通过 `type`、`param` 两个字段来配置采样器。

为什么要配置采样器？因为服务中的请求千千万万，如果每个请求都要记录追踪信息并发送到 Jaeger 后端，那么面对高并发时，记录链路追踪以及推送追踪信息消耗的性能就不可忽视，会对系统带来较大的影响。当我们配置 sampler 后，jaeger 会根据当前配置的采样策略做出采样行为。

详细可以参考：[https://www.jaegertracing.io/docs/1.22/sampling/](https://www.jaegertracing.io/docs/1.22/sampling/)

jaegercfg.SamplerConfig 结构体中的字段 Param 是设置采样率或速率，要根据 Type 而定。

下面对其关系进行说明：

| Type            | Param   | 说明                                                         |
| --------------- | ------- | ------------------------------------------------------------ |
| "const"         | 0或1    | 采样器始终对所有 tracer 做出相同的决定；要么全部采样，要么全部不采样 |
| "probabilistic" | 0.0~1.0 | 采样器做出随机采样决策，Param 为采样概率                     |
| "ratelimiting"  | N       | 采样器一定的恒定速率对tracer进行采样，Param=2.0，则限制每秒采集2条 |
| "remote"        | 无      | 采样器请咨询Jaeger代理以获取在当前服务中使用的适当采样策略。 |

`sampler.Type="remote"`/`sampler.Type=jaeger.SamplerTypeRemote` 是采样器的默认值，当我们不做配置时，会从 Jaeger 后端中央配置甚至动态地控制服务中的采样策略。



### Reporter 配置

看一下 ReporterConfig 的定义。

```go
type ReporterConfig struct {
    QueueSize                  int `yaml:"queueSize"`
    BufferFlushInterval        time.Duration
    LogSpans                   bool   `yaml:"logSpans"`
    LocalAgentHostPort         string `yaml:"localAgentHostPort"`
    DisableAttemptReconnecting bool   `yaml:"disableAttemptReconnecting"`
    AttemptReconnectInterval   time.Duration
    CollectorEndpoint          string            `yaml:"collectorEndpoint"`
    User                       string            `yaml:"user"`
    Password                   string            `yaml:"password"`
    HTTPHeaders                map[string]string `yaml:"http_headers"`
}
```

Reporter 配置客户端如何上报追踪信息的，所有字段都是可选的。

这里我们介绍几个常用的配置字段。

* QUEUESIZE，设置队列大小，存储采样的 span 信息，队列满了后一次性发送到 jaeger 后端；defaultQueueSize 默认为 100；
* BufferFlushInterval 强制清空、推送队列时间，对于流量不高的程序，队列可能长时间不能满，那么设置这个时间，超时可以自动推送一次。对于高并发的情况，一般队列很快就会满的，满了后也会自动推送。默认为1秒。
* LogSpans   是否把 Log 也推送，span 中可以携带一些日志信息。

* LocalAgentHostPort   要推送到的 Jaeger agent，默认端口 6831，是 Jaeger 接收压缩格式的 thrift 协议的数据端口。
* CollectorEndpoint       要推送到的 Jaeger Collector，用 Collector 就不用 agent 了。



例如通过 http 上传 trace：

```
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:           true,
			CollectorEndpoint: "http://127.0.0.1:14268/api/traces",
		},
```

据黑洞大佬的提示，HTTP 走的就是 thrift，而 gRPC 是 .NET 特供，所以 reporter 格式只有一种，而且填写 CollectorEndpoint，我们注意要填写完整的信息。

完整代码测试：

```go
import (
	"bufio"
    "github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"os"
)

func main() {

	var cfg = jaegercfg.Configuration{
		ServiceName: "client test", // 对其发起请求的的调用链，叫什么服务
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:           true,
			CollectorEndpoint: "http://127.0.0.1:14268/api/traces",
		},
	}

	jLogger := jaegerlog.StdLogger
	tracer, closer, _ := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)

	// 创建第一个 span A
	parentSpan := tracer.StartSpan("A")
	// 调用其它服务
	B(tracer, parentSpan)
	// 结束 A
	parentSpan.Finish()
	// 结束当前 tracer
	closer.Close()

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadByte()
}
func B(tracer opentracing.Tracer, parentSpan opentracing.Span) {
	// 继承上下文关系，创建子 span
	childSpan := tracer.StartSpan(
		"B",
		opentracing.ChildOf(parentSpan.Context()),
	)
	defer childSpan.Finish()
}
```

运行后输出结果：

```
2021/03/30 15:04:15 Initializing logging reporter
2021/03/30 15:04:15 Reporting span 715e0af47c7d9acb:7dc9a6b568951e4f:715e0af47c7d9acb:1
2021/03/30 15:04:15 Reporting span 715e0af47c7d9acb:715e0af47c7d9acb:0000000000000000:1
2021/03/30 15:04:15 DEBUG: closing tracer
2021/03/30 15:04:15 DEBUG: closing reporter
2021/03/30 15:04:15 DEBUG: flushed 1 spans
2021/03/30 15:04:15 DEBUG: flushed 1 spans
```

打开 Jaeger UI，可以看到已经推送完毕(http://127.0.0.1:16686)。

![上传的trace](./images/上传的trace.jpg)

这时，我们可以抽象代码代码示例：

```go
func CreateTracer(servieName string) (opentracing.Tracer, io.Closer, error) {
	var cfg = jaegercfg.Configuration{
		ServiceName: servieName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans:          true,
			// 按实际情况替换你的 ip
			CollectorEndpoint: "http://127.0.0.1:14268/api/traces",
		},
	}

	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)
	return tracer, closer, err
}
```

这样可以复用代码，调用函数创建一个新的 tracer。这个记下来，后面要用。



### 分布式系统与span

前面介绍了如何配置 tracer 、推送数据到 Jaeger Collector，接下来我们聊一下 Span。请看图。

下图是一个由用户 X 请求发起的，穿过多个服务的分布式系统，A、B、C、D、E 表示不同的子系统或处理过程。

在这个图中， A 是前端，B、C 是中间层、D、E 是 C 的后端。这些子系统通过 rpc 协议连接，例如 gRPC。

一个简单实用的分布式链路追踪系统的实现，就是对服务器上每一次请求以及响应收集跟踪标识符(message identifiers)和时间戳(timestamped events)。

这里，我们只需要记住，从 A 开始，A 需要依赖多个服务才能完成任务，每个服务可能是一个进程，也可能是一个进程中的另一个函数。这个要看你代码是怎么写的。后面会详细说一下如何定义这种关系，现在大概了解一下即可。

![span调用链](https://img2020.cnblogs.com/blog/1315495/202101/1315495-20210124092812074-1348764625.png)

![span调用链](https://www.whuanle.cn/wp-content/uploads/2021/01/image-1611451697259.png)

### 怎么调、怎么传

如果有了解过 Jaeger 或读过  [分布式链路追踪框架的基本实现原理](https://www.cnblogs.com/whuanle/p/14321107.html)  ，那么已经大概了解的 Jaeger 的工作原理。

jaeger 是分布式链路追踪工具，如果不用在跨进程上，那么 Jaeger 就失去了意义。而微服务中跨进程调用，一般有 HTTP 和 gRPC 两种，下面将来讲解如何在 HTTP、gPRC 调用中传递 Jaeger 的 上下文。



### HTTP，跨进程追踪

A、B 两个进程，A 通过 HTTP 调用 B 时，通过 Http Header 携带 trace 信息(称为上下文)，然后 B 进程接收后，解析出来，在创建 trace 时跟传递而来的 上下文关联起来。

一般使用中间件来处理别的进程传递而来的上下文。`inject` 函数打包上下文到 Header 中，而 `extract` 函数则将其解析出来。

![](https://img2020.cnblogs.com/blog/1315495/202101/1315495-20210124151204778-2127450807.png)

![https://www.whuanle.cn/wp-content/uploads/2021/01/image-1611472330444.png](https://www.whuanle.cn/wp-content/uploads/2021/01/image-1611472330444.png)



这里我们分为两步，第一步从 A 进程中传递上下文信息到 B 进程，为了方便演示已经实践，我们使用 client-webserver 的形式，编写代码。

#### 客户端

在 A 进程新建一个方法：

```go
// 请求远程服务，获得用户信息
func GetUserInfo(tracer opentracing.Tracer, parentSpan opentracing.Span) {
	// 继承上下文关系，创建子 span
	childSpan := tracer.StartSpan(
		"B",
		opentracing.ChildOf(parentSpan.Context()),
	)

	url := "http://127.0.0.1:8081/Get?username=痴者工良"
	req,_ := http.NewRequest("GET", url, nil)
	// 设置 tag，这个 tag 我们后面讲
	ext.SpanKindRPCClient.Set(childSpan)
	ext.HTTPUrl.Set(childSpan, url)
	ext.HTTPMethod.Set(childSpan, "GET")
	tracer.Inject(childSpan.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))
	resp, _ := http.DefaultClient.Do(req)
	_ = resp 	// 丢掉
	defer childSpan.Finish()
}
```

然后复用前面提到的 `CreateTracer` 函数。

main 函数改成：

```go
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
```

完整代码可参考：[https://github.com/whuanle/DistributedTracingGo/issues/1](https://github.com/whuanle/DistributedTracingGo/issues/1)



#### Web 服务端

服务端我们使用 gin 来搭建。

新建一个 go 项目，在 main.go 目录中，执行 `go get -u github.com/gin-gonic/gin`。

创建一个函数，该函数可以从创建一个 tracer，并且继承其它进程传递过来的上下文信息。

```go
// 从上下文中解析并创建一个新的 trace，获得传播的 上下文(SpanContext)
func CreateTracer(serviceName string, header http.Header) (opentracing.Tracer,opentracing.SpanContext, io.Closer, error) {
	var cfg = jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
			// 按实际情况替换你的 ip
			CollectorEndpoint: "http://127.0.0.1:14268/api/traces",
		},
	}

	jLogger := jaegerlog.StdLogger
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
	)
	// 继承别的进程传递过来的上下文
	spanContext, _ := tracer.Extract(opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(header))
	return tracer, spanContext, closer, err
}
```

为了解析 HTTP 传递而来的 span 上下文，我们需要通过中间件来解析了处理一些细节。

```go
func UseOpenTracing() gin.HandlerFunc {
	handler := func(c *gin.Context) {
		// 使用 opentracing.GlobalTracer() 获取全局 Tracer
		tracer,spanContext, closer, _ := CreateTracer("userInfoWebService", c.Request.Header)
		defer closer.Close()
		// 生成依赖关系，并新建一个 span、
		// 这里很重要，因为生成了  References []SpanReference 依赖关系
		startSpan:= tracer.StartSpan(c.Request.URL.Path,ext.RPCServerOption(spanContext))
		defer startSpan.Finish()

		// 记录 tag
		// 记录请求 Url
		ext.HTTPUrl.Set(startSpan, c.Request.URL.Path)
		// Http Method
		ext.HTTPMethod.Set(startSpan, c.Request.Method)
		// 记录组件名称
		ext.Component.Set(startSpan, "Gin-Http")

		// 在 header 中加上当前进程的上下文信息
		c.Request=c.Request.WithContext(opentracing.ContextWithSpan(c.Request.Context(),startSpan))
		// 传递给下一个中间件
		c.Next()
		// 继续设置 tag
		ext.HTTPStatusCode.Set(startSpan, uint16(c.Writer.Status()))
	}

	return handler
}
```

别忘记了 API 服务：

```go
func GetUserInfo(ctx *gin.Context) {
	userName := ctx.Param("username")
	fmt.Println("收到请求，用户名称为:", userName)
	ctx.String(http.StatusOK, "他的博客是 https://whuanle.cn")
}
```

然后是 main 方法：

```go
func main() {
	r := gin.Default()
	// 插入中间件处理
	r.Use(UseOpenTracing())
	r.GET("/Get",GetUserInfo)
	r.Run("0.0.0.0:8081") // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}
```

完整代码可参考：[https://github.com/whuanle/DistributedTracingGo/issues/2](https://github.com/whuanle/DistributedTracingGo/issues/2)

分别启动 webserver、client，会发现打印日志。并且打开 jaerger ui 界面，会出现相关的追踪信息。

![Jaeger追踪记录](./images/Jaeger追踪记录.gif)



### Tag 、 Log 和 Ref

Jaeger 的链路追踪中，可以携带 Tag 和 Log，他们都是键值对的形式：

```json
                        {
                            "key": "http.method",
                            "type": "string",
                            "value": "GET"
                        },
```

Tag 设置方法是 `ext.xxxx`，例如 ：

```
ext.HTTPUrl.Set(startSpan, c.Request.URL.Path)
```

因为 opentracing 已经规定了所有的 Tag 类型，所以我们只需要调用 `ext.xxx.Set()` 设置即可。



前面写示例的时候忘记把日志也加一下了。。。日志其实很简单的，通过 span 对象调用函数即可设置。

示例(在中间件里面加一下)：

```go
        startSpan.LogFields(
            log.String("event", "soft error"),
            log.String("type", "cache timeout"),
            log.Int("waited.millis", 1500))
```

![TAG_LOG](./images/TAG_LOG.png)

ref 就是多个 span 之间的关系。span 可以是跨进程的，也可以是一个进程内的不同函数中的。

其中 span 的依赖关系表示示例：

```json
                    "references": [
                        {
                            "refType": "CHILD_OF",
                            "traceID": "33ba35e7cc40172c",
                            "spanID": "1c7826fa185d1107"
                        }]
```

spanID 为其依赖的父 span。



可以看下面这张图。

一个进程中的 tracer 可以包装一些代码和操作，为多个 span 生成一些信息，或创建父子关系。

而 远程请求中传递的是 SpanContext，传递后，远程服务也创建新的 tracer，然后从 SpanContext 生成 span 依赖关系。

子 span 中，其 reference 列表中，会带有 父 span 的 span id。

![span传播](./images/span传播.png)



关于 Jaeger Client Go 的文章到此完毕，转 Go 没多久，大家可以互相交流哟。