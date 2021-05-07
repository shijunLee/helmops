package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

type ContextType string

type HttpUtil struct {
	*http.Client
	Username              string
	Password              string
	Header                http.Header
	RootCAPath            string
	PrivateKeyPath        string
	CertPath              string
	InsecureSkipVerifyTLS bool
}

type HttpRequestOptions func(r *HttpUtil)

func WithBasicAuth(username, password string) HttpRequestOptions {
	return func(r *HttpUtil) {
		if username != "" && password != "" {
			r.Username = username
			r.Password = password
		}
	}
}

func WithInsecureSkipVerifyTLS(insecureSkipVerifyTLS bool) HttpRequestOptions {
	return func(r *HttpUtil) {
		r.InsecureSkipVerifyTLS = insecureSkipVerifyTLS
	}
}

func WithTLSClientConfig(privateKey, rootCA, certPath string) HttpRequestOptions {
	return func(r *HttpUtil) {
		r.PrivateKeyPath = privateKey
		r.RootCAPath = rootCA
		r.CertPath = certPath
	}
}

func WithContentType(contextType ContextType) HttpRequestOptions {
	return func(r *HttpUtil) {
		r.Header.Set("Content-Type", string(contextType))
	}
}

func WithTimeout(timeout int) HttpRequestOptions {
	return func(r *HttpUtil) {
		if timeout > 0 {
			r.Timeout = time.Duration(timeout) * time.Second
		}
	}
}

func NewHttpRequest() *HttpUtil {
	return &HttpUtil{Header: map[string][]string{}}
}

func (h *HttpUtil) Do(req *http.Request) (*http.Response, error) {
	var tr *http.Transport
	if strings.ToLower(req.URL.Scheme) == "https" {
		if h.InsecureSkipVerifyTLS {
			tr = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		} else if h.RootCAPath != "" && h.PrivateKeyPath != "" {
			certs, err := tls.LoadX509KeyPair(h.RootCAPath, h.PrivateKeyPath)
			if err == nil {
				ca, err := x509.ParseCertificate(certs.Certificate[0])
				if err == nil {
					pool := x509.NewCertPool()
					pool.AddCert(ca)

					tr = &http.Transport{
						TLSClientConfig: &tls.Config{RootCAs: pool},
					}
				}
			}
		} else if h.CertPath != "" && h.PrivateKeyPath != "" {
			cert, err := tls.LoadX509KeyPair(h.CertPath, h.PrivateKeyPath)
			if err == nil {
				tr = &http.Transport{
					TLSClientConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
				}
			}

		}
		if tr == nil {
			tr = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}
	if tr == nil {
		h.Client = http.DefaultClient
	} else {
		h.Client = &http.Client{Transport: tr}
	}
	return h.Client.Do(req)
}

func (h *HttpUtil) httpRequest(method, requestUrl string, headers map[string]string, body io.Reader, opts ...HttpRequestOptions) (data []byte, status int, header map[string][]string, err error) {
	req, err := http.NewRequest(method, requestUrl, body)
	if err != nil {
		return
	}
	for _, opt := range opts {
		opt(h)
	}
	if h.Username != "" && h.Password != "" {
		req.SetBasicAuth(h.Username, h.Password)
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}
	if len(h.Header) > 0 {
		for k, v := range h.Header {
			if len(v) > 0 {
				req.Header.Set(k, v[0])
			}
		}
	}
	resp, err := h.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.Header != nil && len(resp.Header) > 0 {
		for key, value := range resp.Header {
			if header == nil {
				header = map[string][]string{}
			}
			header[key] = value
		}
	}

	status = resp.StatusCode

	data, err = ioutil.ReadAll(resp.Body)

	return
}

func HttpGet(url string, headers map[string]string, opts ...HttpRequestOptions) (data []byte, status int, header map[string][]string, err error) {
	return NewHttpRequest().httpRequest("GET", url, headers, nil, opts...)
}

func HttpPost(url string, headers map[string]string, body io.Reader, opts ...HttpRequestOptions) (data []byte, status int, header map[string][]string, err error) {
	return NewHttpRequest().httpRequest("POST", url, headers, body, opts...)
}

func HttpGetStruct(url string, headers map[string]string, s interface{}, opts ...HttpRequestOptions) error {
	data, code, _, err := HttpGet(url, headers, opts...)
	if err != nil {
		return err
	}
	if !(code >= 200 && code < 300) {
		return fmt.Errorf("invalid http code :%v url:%v data:%v", code, url, getDataMessage(data))
	}
	if (strings.HasPrefix(string(data), "{") || strings.HasPrefix(string(data), "[")) &&
		(strings.HasSuffix(string(data), "}") || strings.HasSuffix(string(data), "]")) {
		return json.Unmarshal(data, &s)
	}
	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s)
}
func HttpPostStruct(url string, headers map[string]string, body io.Reader, out interface{}, opts ...HttpRequestOptions) error {
	data, code, _, err := HttpPost(url, headers, body, opts...)
	if err != nil {
		return err
	}
	if !(code >= 200 && code < 300) {
		return errors.New(string(data))
	}
	if (strings.HasPrefix(string(data), "{") || strings.HasPrefix(string(data), "[")) &&
		(strings.HasSuffix(string(data), "}") || strings.HasSuffix(string(data), "]")) {
		return json.Unmarshal(data, &out)
	}
	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &out)
}
func HttpPatchStruct(url string, headers map[string]string, body io.Reader, out interface{}, opts ...HttpRequestOptions) error {
	data, code, _, err := NewHttpRequest().httpRequest("PATCH", url, headers, body, opts...)
	if err != nil {
		return err
	}
	if !(code >= 200 && code < 300) {

		return fmt.Errorf("invalid http code :%v url:%v data:%v", code, url, getDataMessage(data))
	}
	return json.Unmarshal(data, &out)
}
func HttpPostJsonStruct(url string, headers map[string]string, in, out interface{}, opts ...HttpRequestOptions) error {
	bytes, err := json.Marshal(in)
	if err != nil {
		return err
	}
	body := strings.NewReader(string(bytes))
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Content-Type"] = "application/json"

	data, code, _, err := HttpPost(url, headers, body, opts...)
	if err != nil {
		return err
	}
	if !(code >= 200 && code < 300) {
		return fmt.Errorf("invalid http code :%v url:%v   data:%v", code, url, getDataMessage(data))
	}

	return json.Unmarshal(data, &out)
}

func getDataMessage(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return string(data)
}
