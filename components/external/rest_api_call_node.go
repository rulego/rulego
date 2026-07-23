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
	"golang.org/x/net/proxy"
)

func init() {
	Registry.Add(&RestApiCallNode{})
}

// exists in the metadata key
const (
	//StatusMetadataKey http response status, Metadata Key
	StatusMetadataKey = "status"
	//StatusCodeMetadataKey http response status code, Metadata Key
	StatusCodeMetadataKey = "statusCode"
	//ErrorBodyMetadataKey http responds to error messages, Metadata Key
	ErrorBodyMetadataKey = "errorBody"
	//EventTypeMetadataKey sso Event Type: Metadata Key: data/event/id/retry
	EventTypeMetadataKey = "eventType"
	ContentTypeKey       = "Content-Type"
	AcceptKey            = "Accept"
	//EventStreamMime Stream response type
	EventStreamMime = "text/event-stream"
)

// RestApiCallNodeConfiguration rest
type RestApiCallNodeConfiguration struct {
	//RestEndpointUrlPattern HTTP URL address, which can be replaced by reading variables from the metadata with ${metadata.key} or by reading variables from the message load with ${msg.key}
	RestEndpointUrlPattern string `json:"restEndpointUrlPattern" label:"Request URL" desc:"HTTP request URL, supports ${msg.xxx}, ${metadata.xxx}, ${global.xxx} variable substitution" required:"true"`
	//RequestMethod, default is POST
	RequestMethod string `json:"requestMethod" label:"Request Method" desc:"HTTP method: GET/POST/PUT/DELETE/PATCH, default POST" component:"{\"type\":\"select\",\"options\":[{\"label\":\"GET\",\"value\":\"GET\"},{\"label\":\"POST\",\"value\":\"POST\"},{\"label\":\"PUT\",\"value\":\"PUT\"},{\"label\":\"DELETE\",\"value\":\"DELETE\"},{\"label\":\"PATCH\",\"value\":\"PATCH\"}]}"`
	// Without request body
	WithoutRequestBody bool `json:"withoutRequestBody" label:"No Request Body" desc:"Set to true to skip sending request body"`
	//Headers can be replaced by reading variables from the metadata with ${metadata.key} or by reading variables from the message load with ${msg.key}
	Headers map[string]string `json:"headers" label:"Headers" desc:"HTTP header key-value pairs, e.g. {\"Content-Type\":\"application/json\",\"Authorization\":\"Bearer ${global.token}\"}"`
	// Body requests body, supporting metadata and msg value construction structures. If empty, the message is sent to the destination address
	Body string `json:"body" label:"Body" desc:"Custom request body template, supports ${msg.xxx} variables. Defaults to msg JSON when empty"`
	//ReadTimeoutMs timeout, unit: milliseconds, default 0: no limit
	ReadTimeoutMs int `json:"readTimeoutMs" label:"Timeout (ms)" desc:"Request timeout in milliseconds, default 2000"`
	//Disable certificate verification
	InsecureSkipVerify bool `json:"insecureSkipVerify" label:"Skip TLS Verify" desc:"Set to true to skip HTTPS certificate verification"`
	//MaxParallelRequestsCount Connection pool size, default 200. 0 means no restrictions
	MaxParallelRequestsCount int `json:"maxParallelRequestsCount" label:"Max Parallel Requests" desc:"Connection pool size, default 200, 0 for unlimited"`
	//Whether EnableProxy is enabled
	EnableProxy bool `json:"enableProxy" label:"Enable Proxy" desc:"Whether to enable proxy"`
	//UseSystemProxyProperties uses a system configuration proxy
	UseSystemProxyProperties bool `json:"useSystemProxyProperties" label:"Use System Proxy" desc:"Use system environment variable proxy settings"`
	//ProxyScheme proxy protocol
	ProxyScheme string `json:"proxyScheme" label:"Proxy Scheme" desc:"Proxy protocol: http/https/socks5"`
	//ProxyHost proxy host
	ProxyHost string `json:"proxyHost" label:"Proxy Host" desc:"Proxy server address"`
	//ProxyPort proxy port
	ProxyPort int `json:"proxyPort" label:"Proxy Port" desc:"Proxy server port"`
	//ProxyUser proxy username
	ProxyUser string `json:"proxyUser" label:"Proxy Username" desc:"Proxy authentication username"`
	//ProxyPassword
	ProxyPassword string `json:"proxyPassword" label:"Proxy Password" desc:"Proxy authentication password"`
}

// RestApiCallNode is an HTTP/REST API client component used for external API calls
// RestApiCallNode provides HTTP/REST API client functionality for making external API calls.
//
// Core algorithm:
// Core Algorithm:
// 1. Parse URL, headers, and body with variable substitution using variables
// 2. Build HTTP requests based on configuration (GET/POST/PUT/DELETE, etc.) - Build HTTP request based on configuration
// 3. Send request through configured proxy (optional) - Send request through configured proxy (optional)
// 4. Handle response: JSON, SSE stream, or plain text
// 5. Route to Success/Failure relationship based on HTTP status code
//
// Variable substitution:
//   - ${metadata.key}: Retrieves value from message metadata - Access message metadata
//   - ${msg.key}: Retrieve value from message load - Access message payload fields
//
// Supported HTTP methods:
//   - GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS
//
// Proxy support:
//   - System proxy: HTTP_PROXY. HTTPS_PROXY environment variables - System proxy via environment variables
//   - Custom proxy: HTTP, HTTPS, SOCKS5 protocols - Custom proxy with HTTP, HTTPS, SOCKS5 protocols
//
// Response handling:
//   - HTTP 200: Success relation - Success relation
//   - Non-200: Failure relation, error details stored in metadata - Failure relation with error details stored in metadata
//   - SSE stream: process event data line by line - SSE streams: process event data line by line
//
// Configuration examples:
//
//	Basic POST request - Basic POST request
//	{
//		"id": "apiCall1",
//		"type": "restApiCall",
//		"configuration": {
//			"restEndpointUrlPattern": "https://api.example.com/data",
//			"requestMethod": "POST",
//			"headers": {
//				"Content-Type": "application/json",
//				"Authorization": "Bearer ${metadata.token}"
//			},
//			"readTimeoutMs": 5000
//		}
//	}
//
//	GET request with variable substitution - GET request with variable substitution
//	{
//		"id": "apiCall2",
//		"type": "restApiCall",
//		"configuration": {
//			"restEndpointUrlPattern": "https://api.example.com/users/${msg.userId}/profile",
//			"requestMethod": "GET",
//			"headers": {
//				"Accept": "application/json",
//				"X-API-Key": "${metadata.apiKey}"
//			}
//		}
//	}
//
//	Custom request body - Custom request body
//	{
//		"id": "apiCall3",
//		"type": "restApiCall",
//		"configuration": {
//			"restEndpointUrlPattern": "https://webhook.site/test",
//			"requestMethod": "POST",
//			"body": "{\"name\":\"${msg.name}\",\"age\":${msg.age},\"timestamp\":\"${metadata.timestamp}\"}",
//			"headers": {
//				"Content-Type": "application/json"
//			}
//		}
//	}
//
//	Proxy configuration - Proxy configuration
//	{
//		"id": "apiCall4",
//		"type": "restApiCall",
//		"configuration": {
//			"restEndpointUrlPattern": "https://external-api.com/endpoint",
//			"requestMethod": "POST",
//			"enableProxy": true,
//			"proxyScheme": "http",
//			"proxyHost": "proxy.company.com",
//			"proxyPort": 8080,
//			"proxyUser": "username",
//			"proxyPassword": "password"
//		}
//	}
//
//	SSE streaming response
//	{
//		"id": "apiCall5",
//		"type": "restApiCall",
//		"configuration": {
//			"restEndpointUrlPattern": "https://stream.example.com/events",
//			"requestMethod": "GET",
//			"headers": {
//				"Accept": "text/event-stream",
//				"Cache-Control": "no-cache"
//			}
//		}
//	}
//
// Use cases:
//   - Third-party API integration: Call external service APIs to obtain data
//   - Data pushing: Pushing processing results to downstream systems
//   - Microservice communication: inter-service calls in microservice architecture
//   - Webhook triggering: triggers external system webhook interfaces
//   - Data synchronization: Synchronize data with external sources
//   - Authentication service: Call auth services for user verification
//   - Streaming data processing: processes SSE or long-connection real-time streams
type RestApiCallNode struct {
	//Node configuration
	Config RestApiCallNodeConfiguration
	//httpClient: http client
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

// Type returns the component type
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

// Init initializes the component
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

// OnMsg handles messages, sends HTTP requests, and handles responses
// OnMsg processes messages by sending HTTP requests and handling responses.
func (x *RestApiCallNode) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	var evn map[string]interface{}
	if x.template.HasVar {
		evn = base.NodeUtils.GetEvnAndMetadata(ctx, msg)
	}
	var endpointUrl = x.template.UrlTemplate.ExecuteAsString(evn)
	var req *http.Request
	var err error
	var body []byte
	if x.Config.WithoutRequestBody {
		req, err = http.NewRequest(x.Config.RequestMethod, endpointUrl, nil)
	} else {
		if x.template.BodyTemplate != nil {
			body = []byte(x.template.BodyTemplate.ExecuteAsString(evn))
		} else {
			body = []byte(msg.GetData())
		}
		req, err = http.NewRequest(x.Config.RequestMethod, endpointUrl, bytes.NewReader(body))
	}
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	//Set the header
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

// Desc returns the component description
func (x *RestApiCallNode) Desc() string {
	return "Send HTTP requests to external APIs. Body defaults to msg JSON, response written back to msg. Supports ${msg.xxx}, ${metadata.xxx}, ${global.xxx} substitution. Routes to Success/Failure"
}

// Destroy releases resources
func (x *RestApiCallNode) Destroy() {
}

// NewHttpClient creates an HTTP client
func NewHttpClient(config RestApiCallNodeConfiguration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}
	transport.MaxConnsPerHost = config.MaxParallelRequestsCount

	// Configure the agent
	if config.EnableProxy {
		if config.UseSystemProxyProperties {
			// Use system proxy settings
			if proxyURL := HttpUtils.GetSystemProxy(); proxyURL != nil {
				transport.Proxy = http.ProxyURL(proxyURL)
			}
		} else {
			// Use custom proxy settings
			if proxyURL := HttpUtils.BuildProxyURL(config.ProxyScheme, config.ProxyHost, config.ProxyPort, config.ProxyUser, config.ProxyPassword); proxyURL != nil {
				if config.ProxyScheme == "socks5" {
					// SOCKS5 proxies require special handling
					transport.Dial = HttpUtils.CreateSOCKS5Dialer(proxyURL)
				} else {
					// HTTP/HTTPS proxy
					transport.Proxy = http.ProxyURL(proxyURL)
				}
			}
		}
	}

	return &http.Client{Transport: transport,
		Timeout: time.Duration(config.ReadTimeoutMs) * time.Millisecond}
}

// SSE streaming data reading
func readFromStream(ctx types.RuleContext, msg types.RuleMsg, resp *http.Response) {
	HttpUtils.ReadFromStream(ctx, msg, resp)
}

// HttpUtils Global HttpUtils instance
var HttpUtils = NewHttpUtils()

// httpUtils: A collection of HTTP-related utility functions
type httpUtils struct{}

// NewHttpUtils creates an HttpUtils instance
func NewHttpUtils() *httpUtils {
	return &httpUtils{}
}

// GetSystemProxy to obtain the system proxy settings
func (h *httpUtils) GetSystemProxy() *url.URL {
	// Check environmental variables
	for _, env := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		if proxyStr := os.Getenv(env); proxyStr != "" {
			if proxyURL, err := url.Parse(proxyStr); err == nil {
				return proxyURL
			}
		}
	}
	return nil
}

// BuildProxyURL: Build the proxy URL
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

// CreateSOCKS5Dialer Creates a SOCKS5 dialer
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

// Base64Encode Simple base64 encoding (multiplex function)
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

// ReadFromStream reads data from an SSE stream
func (h *httpUtils) ReadFromStream(ctx types.RuleContext, msg types.RuleMsg, resp *http.Response) {
	defer resp.Body.Close()
	// Read data from the responsive Body using bufio.Scanner reads line by line
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		// Retrieve a line of data
		line := scanner.Text()
		// If it is a blank line, it means one event has ended and the next event is read
		if line == "" {
			continue
		}
		// If it is a comment line, ignore it
		if strings.HasPrefix(line, ":") {
			continue
		}
		// Parse data and process it according to different event types and data content
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
	//Server-Send Events streaming response
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
