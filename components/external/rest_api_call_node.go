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
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func init() {
	Registry.Add(&RestApiCallNode{})
}

// 存在到metadata key
const (
	//http响应状态，Metadata Key
	statusMetadataKey = "status"
	//http响应状态码，Metadata Key
	statusCodeMetadataKey = "statusCode"
	//http响应错误信息，Metadata Key
	errorBodyMetadataKey = "errorBody"
	//sso事件类型Metadata Key：data/event/id/retry
	eventTypeMetadataKey = "eventType"

	contentTypeKey  = "Content-Type"
	acceptKey       = "Accept"
	eventStreamMime = "text/event-stream"
)

// RestApiCallNodeConfiguration rest配置
type RestApiCallNodeConfiguration struct {
	//RestEndpointUrlPattern HTTP URL地址,可以使用 ${metaKeyName} 替换元数据中的变量
	RestEndpointUrlPattern string
	//RequestMethod 请求方法，默认POST
	RequestMethod string
	//Headers 请求头,可以使用 ${metaKeyName} 替换元数据中的变量
	Headers map[string]string
	//ReadTimeoutMs 超时，单位毫秒，默认0:不限制
	ReadTimeoutMs int
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
	//是否是SSE（Server-Send Events）流式响应
	isStream bool
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
		ReadTimeoutMs:            0,
		Headers:                  headers,
	}
	return &RestApiCallNode{Config: config}
}

// Init 初始化
func (x *RestApiCallNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.Config)
	if err == nil {
		x.Config.RequestMethod = strings.ToUpper(x.Config.RequestMethod)
		x.httpClient = NewHttpClient(x.Config)
		//Server-Send Events 流式响应
		if strings.HasPrefix(x.Config.Headers[acceptKey], eventStreamMime) || strings.HasPrefix(x.Config.Headers[contentTypeKey], eventStreamMime) {
			x.isStream = true
		}
	}
	return err
}

// OnMsg 处理消息
func (x *RestApiCallNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	metaData := msg.Metadata.Values()
	endpointUrl := str.SprintfDict(x.Config.RestEndpointUrlPattern, metaData)
	req, err := http.NewRequest(x.Config.RequestMethod, endpointUrl, bytes.NewReader([]byte(msg.Data)))
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	//设置header
	for key, value := range x.Config.Headers {
		req.Header.Set(str.SprintfDict(key, metaData), str.SprintfDict(value, metaData))
	}

	response, err := x.httpClient.Do(req)
	defer func() {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
	}()

	if err != nil {
		ctx.TellFailure(msg, err)
	} else if x.isStream {
		msg.Metadata.PutValue(statusMetadataKey, response.Status)
		msg.Metadata.PutValue(statusCodeMetadataKey, strconv.Itoa(response.StatusCode))
		if response.StatusCode == 200 {
			readFromStream(ctx, msg, response)
		} else {
			b, _ := io.ReadAll(response.Body)
			msg.Metadata.PutValue(errorBodyMetadataKey, string(b))
			ctx.TellNext(msg, types.Failure)
		}

	} else if b, err := io.ReadAll(response.Body); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		msg.Metadata.PutValue(statusMetadataKey, response.Status)
		msg.Metadata.PutValue(statusCodeMetadataKey, strconv.Itoa(response.StatusCode))
		if response.StatusCode == 200 {
			msg.Data = string(b)
			ctx.TellSuccess(msg)
		} else {
			msg.Metadata.PutValue(errorBodyMetadataKey, string(b))
			ctx.TellNext(msg, types.Failure)
		}

	}
}

// Destroy 销毁
func (x *RestApiCallNode) Destroy() {
}

func NewHttpClient(config RestApiCallNodeConfiguration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	transport.MaxConnsPerHost = config.MaxParallelRequestsCount
	if config.EnableProxy && !config.UseSystemProxyProperties {
		//开启代理
		urli := url.URL{}
		proxyUrl := fmt.Sprintf("%s://%s:%d", config.ProxyScheme, config.ProxyHost, config.ProxyPort)
		urlProxy, _ := urli.Parse(proxyUrl)
		if config.ProxyUser != "" && config.ProxyPassword != "" {
			urlProxy.User = url.UserPassword(config.ProxyUser, config.ProxyPassword)
		}
		transport.Proxy = http.ProxyURL(urlProxy)
	}
	return &http.Client{Transport: transport,
		Timeout: time.Duration(config.ReadTimeoutMs) * time.Millisecond}
}

// SSE 流式数据读取
func readFromStream(ctx types.RuleContext, msg types.RuleMsg, resp *http.Response) {
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
		msg.Metadata.PutValue(eventTypeMetadataKey, eventType)
		msg.Data = eventData
		ctx.TellSuccess(msg)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		ctx.TellFailure(msg, err)
	}
}
