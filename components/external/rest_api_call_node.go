/*
 * Copyright 2023 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package external

//规则链节点配置示例：
// {
//        "id": "s3",
//        "type": "restApiCall",
//        "name": "推送数据",
//        "debugMode": false,
//        "configuration": {
//          "restEndpointUrlPattern": "http://192.168.118.29:8080/msg",
//          "requestMethod": "POST",
//          "maxParallelRequestsCount": 200
//        }
//      }
import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/components/base"
	"github.com/rulego/rulego/utils/el"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"golang.org/x/net/proxy"
)

func init() {
	Registry.Add(&RestApiCallNode{})
}

// 存在到metadata key
const (
	//StatusMetadataKey http响应状态，Metadata Key
	StatusMetadataKey = "status"
	//StatusCodeMetadataKey http响应状态码，Metadata Key
	StatusCodeMetadataKey = "statusCode"
	//ErrorBodyMetadataKey http响应错误信息，Metadata Key
	ErrorBodyMetadataKey = "errorBody"
	//EventTypeMetadataKey sso事件类型Metadata Key：data/event/id/retry
	EventTypeMetadataKey = "eventType"
	ContentTypeKey       = "Content-Type"
	AcceptKey            = "Accept"
	//EventStreamMime 流式响应类型
	EventStreamMime = "text/event-stream"
)

// RestApiCallNodeConfiguration rest配置
type RestApiCallNodeConfiguration struct {
	//RestEndpointUrlPattern HTTP URL地址,可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	RestEndpointUrlPattern string
	//RequestMethod 请求方法，默认POST
	RequestMethod string
	// Without request body
	WithoutRequestBody bool
	//Headers 请求头,可以使用 ${metadata.key} 读取元数据中的变量或者使用 ${msg.key} 读取消息负荷中的变量进行替换
	Headers map[string]string
	// Body 请求body,支持metadata、msg取值构建body。如果空，则把消息符合传输到目标地址
	// 例如：
	// 表达式取值：${msg.value}
	// 或者构建JSON格式：
	// {
	//  "name":"${msg.name}",
	//  "age":"${msg.age}",
	//  "type":"admin"
	// }
	// 或者输入字符串：01010101
	Body string
	//ReadTimeoutMs 超时，单位毫秒，默认0:不限制
	ReadTimeoutMs int
	//禁用证书验证
	InsecureSkipVerify bool
	//MaxParallelRequestsCount 连接池大小，默认200。0代表不限制
	MaxParallelRequestsCount int
	//EnableProxy 是否开启代理
	EnableProxy bool
	//UseSystemProxyProperties 使用系统配置代理
	UseSystemProxyProperties bool
	//ProxyScheme 代理协议
	ProxyScheme string
	//ProxyHost 代理主机
	ProxyHost string
	//ProxyPort 代理端口
	ProxyPort int
	//ProxyUser 代理用户名
	ProxyUser string
	//ProxyPassword 代理密码
	ProxyPassword string
}

// RestApiCallNode 将通过REST API调用GET | POST | PUT | DELETE到外部REST服务。
// 如果请求成功，把HTTP响应消息发送到`Success`链, 否则发到`Failure`链，
// metaData.status记录响应错误码和metaData.errorBody记录错误信息。
type RestApiCallNode struct {
	//节点配置
	Config RestApiCallNodeConfiguration
	//httpClient http客户端
	httpClient *http.Client
	template   *HTTPRequestTemplate
}

type HTTPRequestTemplate struct {
	IsStream        bool
	UrlTemplate     el.Template
	HeadersTemplate map[*el.MixedTemplate]*el.MixedTemplate
	BodyTemplate    el.Template
	HasVar          bool
}

// Type 组件类型
func (x *RestApiCallNode) Type() string {
	return "restApiCall"
}

func (x *RestApiCallNode) New() types.Node {
	headers := map[string]string{"Content-Type": "application/json"}
	config := RestApiCallNodeConfiguration{
		RequestMethod:            "POST",
		MaxParallelRequestsCount: 200,
		ReadTimeoutMs:            2000,
		Headers:                  headers,
		InsecureSkipVerify:       true,
	}
	return &RestApiCallNode{Config: config}
}

// Init 初始化
func (x *RestApiCallNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		x.Config.RequestMethod = strings.ToUpper(x.Config.RequestMethod)
		x.httpClient = NewHttpClient(x.Config)
		if tmp, err := HttpUtils.BuildRequestTemplate(&x.Config); err != nil {
			return err
		} else {
			x.template = tmp
		}
	}
	return err
}

// OnMsg 处理消息
func (x *RestApiCallNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var evn map[string]interface{}
	if x.template.HasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	var endpointUrl = ""
	if v, err := x.template.UrlTemplate.Execute(evn); err != nil {
		ctx.TellFailure(msg, err)
		return
	} else {
		endpointUrl = str.ToString(v)
	}
	var req *http.Request
	var err error
	var body []byte
	if x.Config.WithoutRequestBody {
		req, err = http.NewRequest(x.Config.RequestMethod, endpointUrl, nil)
	} else {
		if x.template.BodyTemplate != nil {
			if v, err := x.template.BodyTemplate.Execute(evn); err != nil {
				ctx.TellFailure(msg, err)
				return
			} else {
				body = []byte(str.ToString(v))
			}
		} else {
			body = []byte(msg.GetData())
		}
		req, err = http.NewRequest(x.Config.RequestMethod, endpointUrl, bytes.NewReader(body))
	}
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	//设置header
	for key, value := range x.template.HeadersTemplate {
		req.Header.Set(key.ExecuteAsString(evn), value.ExecuteAsString(evn))
	}

	response, err := x.httpClient.Do(req)
	defer func() {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
	}()

	if err != nil {
		msg.Metadata.PutValue(ErrorBodyMetadataKey, err.Error())
		ctx.TellFailure(msg, err)
	} else if x.template.IsStream {
		msg.Metadata.PutValue(StatusMetadataKey, response.Status)
		msg.Metadata.PutValue(StatusCodeMetadataKey, strconv.Itoa(response.StatusCode))
		if response.StatusCode == 200 {
			readFromStream(ctx, msg, response)
		} else {
			b, _ := io.ReadAll(response.Body)
			msg.Metadata.PutValue(ErrorBodyMetadataKey, string(b))
			ctx.TellNext(msg, types.Failure)
		}

	} else if b, err := io.ReadAll(response.Body); err != nil {
		msg.Metadata.PutValue(ErrorBodyMetadataKey, err.Error())
		ctx.TellFailure(msg, err)
	} else {
		msg.Metadata.PutValue(StatusMetadataKey, response.Status)
		msg.Metadata.PutValue(StatusCodeMetadataKey, strconv.Itoa(response.StatusCode))
		if response.StatusCode == 200 {
			msg.SetData(string(b))
			ctx.TellSuccess(msg)
		} else {
			strB := string(b)
			msg.Metadata.PutValue(ErrorBodyMetadataKey, strB)
			ctx.TellFailure(msg, errors.New(strB))
		}
	}
}

// Destroy 销毁
func (x *RestApiCallNode) Destroy() {
}

// NewHttpClient 创建http客户端
func NewHttpClient(config RestApiCallNodeConfiguration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}
	transport.MaxConnsPerHost = config.MaxParallelRequestsCount

	// 配置代理
	if config.EnableProxy {
		if config.UseSystemProxyProperties {
			// 使用系统代理设置
			if proxyURL := HttpUtils.GetSystemProxy(); proxyURL != nil {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		} else {
			// 使用自定义代理设置
			if proxyURL := HttpUtils.BuildProxyURL(config.ProxyScheme, config.ProxyHost, config.ProxyPort, config.ProxyUser, config.ProxyPassword); proxyURL != nil {
				if config.ProxyScheme == "socks5" {
					// SOCKS5代理需要特殊处理
					transport.Dial = HttpUtils.CreateSOCKS5Dialer(proxyURL)
				} else {
					// HTTP/HTTPS代理
					transport.Proxy = http.ProxyURL(proxyURL)
				}
			}
		}
	}

	return &http.Client{Transport: transport,
		Timeout: time.Duration(config.ReadTimeoutMs) * time.Millisecond}
}

// SSE 流式数据读取
func readFromStream(ctx types.RuleContext, msg types.RuleMsg, resp *http.Response) {
	HttpUtils.ReadFromStream(ctx, msg, resp)
}

// HttpUtils 全局HttpUtils实例
var HttpUtils = NewHttpUtils()

// httpUtils HTTP相关工具函数集合
type httpUtils struct{}

// NewHttpUtils 创建HttpUtils实例
func NewHttpUtils() *httpUtils {
	return &httpUtils{}
}

// GetSystemProxy 获取系统代理设置
func (h *httpUtils) GetSystemProxy() *url.URL {
	// 检查环境变量
	for _, env := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if proxyStr := os.Getenv(env); proxyStr != "" {
			if proxyURL, err := url.Parse(proxyStr); err == nil {
				return proxyURL
			}
		}
	}
	return nil
}

// BuildProxyURL 构建代理URL
func (h *httpUtils) BuildProxyURL(scheme, host string, port int, user, password string) *url.URL {
	if scheme == "" || host == "" || port == 0 {
		return nil
	}

	proxyURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)
	if user != "" && password != "" {
		proxyURL = fmt.Sprintf("%s://%s:%s@%s:%d", scheme, user, password, host, port)
	}

	if parsedURL, err := url.Parse(proxyURL); err == nil {
		return parsedURL
	}
	return nil
}

// CreateSOCKS5Dialer 创建SOCKS5拨号器
func (h *httpUtils) CreateSOCKS5Dialer(proxyURL *url.URL) func(network, addr string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		var auth *proxy.Auth
		if proxyURL.User != nil {
			if password, ok := proxyURL.User.Password(); ok {
				auth = &proxy.Auth{
					User:     proxyURL.User.Username(),
					Password: password,
				}
			}
		}

		dialer, err := proxy.SOCKS5(network, proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		return dialer.Dial(network, addr)
	}
}

const base64Table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// Base64Encode 简单的base64编码（复用函数）
func (h *httpUtils) Base64Encode(s string) string {
	data := []byte(s)
	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		b := uint32(data[i]) << 16
		if i+1 < len(data) {
			b |= uint32(data[i+1]) << 8
		}
		if i+2 < len(data) {
			b |= uint32(data[i+2])
		}

		for j := 0; j < 4; j++ {
			if i*8/6+j < (len(data)*8+5)/6 {
				result = append(result, base64Table[(b>>(18-j*6))&0x3F])
			} else {
				result = append(result, '=')
			}
		}
	}

	return string(result)
}

// ReadFromStream 从SSE流中读取数据
func (h *httpUtils) ReadFromStream(ctx types.RuleContext, msg types.RuleMsg, resp *http.Response) {
	defer resp.Body.Close()
	// 从响应的Body中读取数据，使用bufio.Scanner按行读取
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// 获取一行数据
		line := scanner.Text()
		// 如果是空行，表示一个事件结束，继续读取下一个事件
		if line == "" {
			continue
		}
		// 如果是注释行，忽略
		if strings.HasPrefix(line, ":") {
			continue
		}
		// 解析数据，根据不同的事件类型和数据内容进行处理
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		eventType := strings.TrimSpace(parts[0])
		eventData := strings.TrimSpace(parts[1])
		msg.Metadata.PutValue(EventTypeMetadataKey, eventType)
		msg.SetData(eventData)
		ctx.TellSuccess(msg)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		ctx.TellFailure(msg, err)
	}
}

func (h *httpUtils) BuildRequestTemplate(config *RestApiCallNodeConfiguration) (*HTTPRequestTemplate, error) {
	reqTemplate := &HTTPRequestTemplate{}
	//Server-Send Events 流式响应
	if strings.HasPrefix(config.Headers[AcceptKey], EventStreamMime) ||
		strings.HasPrefix(config.Headers[ContentTypeKey], EventStreamMime) {
		reqTemplate.IsStream = true
	}
	if tmpl, err := el.NewTemplate(config.RestEndpointUrlPattern); err != nil {
		return nil, err
	} else {
		reqTemplate.UrlTemplate = tmpl
		if reqTemplate.UrlTemplate.HasVar() {
			reqTemplate.HasVar = true
		}
	}

	var headerTemplates = make(map[*el.MixedTemplate]*el.MixedTemplate)
	for key, value := range config.Headers {
		keyTmpl, _ := el.NewMixedTemplate(key)
		valueTmpl, _ := el.NewMixedTemplate(value)
		headerTemplates[keyTmpl] = valueTmpl
		if keyTmpl.HasVar() || valueTmpl.HasVar() {
			reqTemplate.HasVar = true
		}
	}
	reqTemplate.HeadersTemplate = headerTemplates

	config.Body = strings.TrimSpace(config.Body)
	if config.Body != "" {
		if bodyTemplate, err := el.NewTemplate(config.Body); err != nil {
			return nil, err
		} else {
			reqTemplate.BodyTemplate = bodyTemplate
			if bodyTemplate.HasVar() {
				reqTemplate.HasVar = true
			}
		}
	}
	return reqTemplate, nil
}
