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

package action

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
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/maps"
	"github.com/rulego/rulego/utils/str"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func init() {
	Registry.Add(&RestApiCallNode{})
}

//存在到metadata key
const (
	//http响应状态
	status = "status"
	//http响应状态码
	statusCode = "statusCode"
	//http响应错误信息
	errorBody = "errorBody"
)

//RestApiCallNodeConfiguration rest配置
type RestApiCallNodeConfiguration struct {
	//RestEndpointUrlPattern HTTP URL地址目标,可以使用 ${metaKeyName} 替换元数据中的变量
	RestEndpointUrlPattern string
	//RequestMethod 请求方法
	RequestMethod string
	//Headers 请求头,可以使用 ${metaKeyName} 替换元数据中的变量
	Headers map[string]string
	//ReadTimeoutMs 超时，单位毫秒
	ReadTimeoutMs int
	//MaxParallelRequestsCount 连接池大小，默认200
	MaxParallelRequestsCount int
	//EnableProxy 是否开启代理
	EnableProxy bool
	//UseSystemProxyProperties 使用系统配置代理
	UseSystemProxyProperties bool
	//ProxyHost 代理主机
	ProxyHost string
	//ProxyPort 代理端口
	ProxyPort int
	//ProxyUser 代理用户名
	ProxyUser string
	//ProxyPassword 代理密码
	ProxyPassword string
	//ProxyScheme
	ProxyScheme string
}

//RestApiCallNode 将通过REST API调用<code> GET | POST | PUT | DELETE </ code>到外部REST服务。
//如果请求成功，把HTTP响应消息发送到`Success`链, 否则发到`Failure`链，
//metaData.status记录响应错误码和metaData.errorBody记录错误信息。
type RestApiCallNode struct {
	//节点配置
	config RestApiCallNodeConfiguration
	//httpClient http客户端
	httpClient *http.Client
}

//Type 组件类型
func (x *RestApiCallNode) Type() string {
	return "restApiCall"
}

func (x *RestApiCallNode) New() types.Node {
	headers := map[string]string{"Content-Type": "application/json"}
	config := RestApiCallNodeConfiguration{
		RequestMethod:            "POST",
		MaxParallelRequestsCount: 200,
		ReadTimeoutMs:            20000,
		Headers:                  headers,
	}
	return &RestApiCallNode{config: config}
}

//Init 初始化
func (x *RestApiCallNode) Init(ruleConfig types.Config, configuration types.Configuration) error {
	err := maps.Map2Struct(configuration, &x.config)
	if err == nil {
		x.config.RequestMethod = strings.ToUpper(x.config.RequestMethod)
		x.httpClient = NewHttpClient(x.config)
	}
	return err
}

//OnMsg 处理消息
func (x *RestApiCallNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) error {
	metaData := msg.Metadata.Values()
	endpointUrl := str.SprintfDict(x.config.RestEndpointUrlPattern, metaData)
	req, err := http.NewRequest(x.config.RequestMethod, endpointUrl, bytes.NewReader([]byte(msg.Data)))
	if err != nil {
		ctx.TellFailure(msg, err)
	}
	//设置header
	for key, value := range x.config.Headers {
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
	} else if b, err := ioutil.ReadAll(response.Body); err != nil {
		ctx.TellFailure(msg, err)
	} else {
		msg.Metadata.PutValue(status, response.Status)
		msg.Metadata.PutValue(statusCode, strconv.Itoa(response.StatusCode))
		if response.StatusCode == 200 {
			msg.Data = string(b)
			ctx.TellSuccess(msg)
		} else {
			msg.Metadata.PutValue(errorBody, string(b))
			ctx.TellNext(msg, types.Failure)
		}

	}

	return nil
}

//Destroy 销毁
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
