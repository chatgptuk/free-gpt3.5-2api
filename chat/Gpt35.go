package chat

import (
	"encoding/json"
	"fmt"
	"free-gpt3.5-2api/ProxyPool"
	"free-gpt3.5-2api/RequestClient"
	"free-gpt3.5-2api/common"
	"free-gpt3.5-2api/config"
	"github.com/aurorax-neo/go-logger"
	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/google/uuid"
	"io"
)

const BaseUrl = "https://chat.openai.com"
const ApiUrl = BaseUrl + "/backend-anon/conversation"
const SessionUrl = BaseUrl + "/backend-anon/sentinel/chat-requirements"

type Gpt35 struct {
	RequestClient RequestClient.RequestClient
	MaxUseCount   int
	ExpiresIn     int64
	Session       *session
	Ua            string
	Language      string
	IsUpdating    bool
}

type session struct {
	OaiDeviceId string           `json:"-"`
	Persona     string           `json:"persona"`
	Arkose      arkose           `json:"arkose"`
	Turnstile   turnstile        `json:"turnstile"`
	ProofWork   common.ProofWork `json:"proofofwork"`
	Token       string           `json:"token"`
}

type arkose struct {
	Required bool   `json:"required"`
	Dx       string `json:"dx"`
}

type turnstile struct {
	Required bool `json:"required"`
}

func NewGpt35() *Gpt35 {
	// 创建 Gpt35 实例
	gpt35 := &Gpt35{
		MaxUseCount: -1,
		ExpiresIn:   -1,
		IsUpdating:  false,
		Session:     &session{},
	}
	// 获取请求客户端
	err := gpt35.getNewRequestClient()
	if err != nil {
		return gpt35
	}
	// 获取新session
	err = gpt35.getNewSession()
	if err != nil {
		return gpt35
	}
	return gpt35
}

func (G *Gpt35) getNewRequestClient() error {
	// 获取代理池
	ProxyPoolInstance := ProxyPool.GetProxyPoolInstance()
	// 获取代理
	proxy := ProxyPoolInstance.GetProxy()
	// 请求客户端
	G.RequestClient = RequestClient.NewTlsClient(300, profiles.Okhttp4Android13)
	if G.RequestClient == nil {
		logger.Logger.Error("RequestClient is nil")
		return fmt.Errorf("RequestClient is nil")
	}
	// 设置代理
	err := G.RequestClient.SetProxy(proxy.Link.String())
	if err != nil {
		logger.Logger.Error(fmt.Sprint("SetProxy Error: ", err))
	}
	// 设置 User-Agent
	G.Ua = proxy.Ua
	// 设置语言
	G.Language = proxy.Language
	return nil
}

func (G *Gpt35) getNewSession() error {
	// 生成新的设备 ID
	G.Session.OaiDeviceId = uuid.New().String()
	// 创建请求
	request, err := G.NewRequest("POST", SessionUrl, nil)
	if err != nil {
		return err
	}
	// 设置请求头
	request.Header.Set("oai-device-id", G.Session.OaiDeviceId)
	// 发送 POST 请求
	response, err := G.RequestClient.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("StatusCode: %d", response.StatusCode)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)
	if err := json.NewDecoder(response.Body).Decode(&G.Session); err != nil {
		return err
	}
	if G.Session.ProofWork.Required {
		G.Session.ProofWork.Ospt = common.CalcProofToken(G.Session.ProofWork.Seed, G.Session.ProofWork.Difficulty, request.Header.Get("User-Agent"))
	}
	// 设置 MaxUseCount
	G.MaxUseCount = 1
	// 设置 ExpiresIn
	G.ExpiresIn = common.GetTimestampSecond(config.AuthED)
	// 设置 IsUpdating
	G.IsUpdating = false
	return nil
}

func (G *Gpt35) NewRequest(method, url string, body io.Reader) (*fhttp.Request, error) {
	request, err := G.RequestClient.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("origin", common.GetOrigin(BaseUrl))
	request.Header.Set("referer", common.GetOrigin(BaseUrl))
	request.Header.Set("accept", "*/*")
	request.Header.Set("cache-control", "no-cache")
	request.Header.Set("content-type", "application/json")
	request.Header.Set("pragma", "no-cache")
	request.Header.Set("sec-ch-ua-mobile", "?0")
	request.Header.Set("sec-fetch-dest", "empty")
	request.Header.Set("sec-fetch-mode", "cors")
	request.Header.Set("sec-fetch-site", "same-origin")
	request.Header.Set("oai-language", G.Language)
	request.Header.Set("accept-language", G.Language)
	request.Header.Set("User-Agent", G.Ua)
	return request, nil
}
