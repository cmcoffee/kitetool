package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/cmcoffee/go-iotimeout"
	"github.com/cmcoffee/go-nfo"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var KWAdmin KWSession

type KWSession string

type APIRequest struct {
	APIVer int
	Method string
	Path   string
	Params []interface{}
	Output interface{}
}

var SetPath = fmt.Sprintf

func SetParams(vars ...interface{}) (output []interface{}) {
	for _, v := range vars {
		output = append(output, v)
	}
	return
}

type PostJSON map[string]interface{}
type PostForm map[string]interface{}
type Query map[string]interface{}

type api_call struct {
	*http.Client
}

// kiteworks API Call Wrapper
func (s KWSession) Call(api_req APIRequest) (err error) {

	req, err := s.NewRequest(api_req.Method, api_req.Path, api_req.APIVer)
	if err != nil {
		return err
	}

	if global.snoop {
		nfo.Stdout("\n[%s]", s)
		nfo.Stdout("--> METHOD: \"%s\" PATH: \"%s\"", strings.ToUpper(api_req.Method), api_req.Path)
	}

	sprinter := func(input interface{}) string {
		switch v := input.(type) {
		case []string:
			return strings.Join(v, ",")
		case []int:
			var output []string
			for _, i := range v {
				output = append(output, fmt.Sprintf("%v", i))
			}
			return strings.Join(output, ",")
		default:
			return fmt.Sprintf("%v", input)
		}
	}

	var body []byte

	for _, in := range api_req.Params {
		switch i := in.(type) {
		case PostForm:
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			p := make(url.Values)
			for k, v := range i {
				p.Add(k, sprinter(v))
				if global.snoop {
					nfo.Stdout("\\-> POST PARAM: \"%s\" VALUE: \"%s\"", k, p[k])
				}
			}
			body = []byte(p.Encode())
		case PostJSON:
			req.Header.Set("Content-Type", "application/json")
			json, err := json.Marshal(i)
			if err != nil {
				return err
			}
			if global.snoop {
				nfo.Stdout("\\-> POST JSON: %s", string(json))
			}
			body = json
		case Query:
			q := req.URL.Query()
			for k, v := range i {
				q.Set(k, sprinter(v))
				if global.snoop {
					nfo.Stdout("\\-> QUERY: %s=%s", k, q[k])
				}
			}
			req.URL.RawQuery = q.Encode()
		default:
			return fmt.Errorf("Unknown request exception.")
		}
	}

	var resp *http.Response

	// Retry call on failures.
	for i := 0; i < MAX_RETRY; i++ {
		req.Body = ioutil.NopCloser(bytes.NewReader(body))
		client := s.NewClient()
		resp, err = client.Do(req)
		if err != nil && RestError(err, ERR_INTERNAL_SERVER_ERROR|TOKEN_ERR) {
			if err := s.SetToken(req, RestError(err, TOKEN_ERR)); err != nil {
				return err
			}
			time.Sleep(time.Second)
			continue
		} else if err != nil {
			return err
		}

		err = DecodeJSON(resp, api_req.Output)
		if err != nil && RestError(err, ERR_INTERNAL_SERVER_ERROR|TOKEN_ERR) {
			nfo.Debug("(JSON) %s -> %s: %s (%d/%d)", s, api_req.Path, err.Error(), i+1, MAX_RETRY)
			if err := s.SetToken(req, RestError(err, TOKEN_ERR)); err != nil {
				return err
			}
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}
	return
}

// New kiteworks Request.
func (s KWSession) NewRequest(method, path string, api_ver int) (req *http.Request, err error) {

	// Set API Version
	if api_ver == 0 {
		api_ver = 11
	}

	server := global.config.Server

	req, err = http.NewRequest(method, fmt.Sprintf("https://%s%s", server, path), nil)
	if err != nil {
		return nil, err
	}

	req.URL.Host = server
	req.URL.Scheme = "https"
	req.Header.Set("X-Accellion-Version", fmt.Sprintf("%d", api_ver))
	req.Header.Set("User-Agent", fmt.Sprintf("%s Admin Assistant/v%v - %v", APPNAME, VERSION_STRING, KWAdmin))
	req.Header.Set("Referer", "https://"+server+"/")

	if err := s.SetToken(req, false); err != nil {
		return nil, err
	}

	return req, nil
}

type Auth struct {
	AccessToken string `json:"access_token"`
	Expires     int64  `json:"expires_in"`
}

// Get a kiteworks token.
func NewToken(username string) (auth *Auth, err error) {

	path := fmt.Sprintf("https://%s/oauth/token", global.config.Server)

	req, err := http.NewRequest(http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}

	http_header := make(http.Header)
	http_header.Set("Content-Type", "application/x-www-form-urlencoded")
	http_header.Set("User-Agent", fmt.Sprintf("%s Admin Assistant/v%v - %v", APPNAME, VERSION_STRING, KWAdmin))

	req.Header = http_header

	client_id := global.config.ClientID
	signature := global.config.Signature

	randomizer := rand.New(rand.NewSource(int64(time.Now().Unix())))
	nonce := randomizer.Int() % 999999
	timestamp := int64(time.Now().Unix())

	base_string := fmt.Sprintf("%s|@@|%s|@@|%d|@@|%d", client_id, username, timestamp, nonce)

	mac := hmac.New(sha1.New, []byte(signature))
	mac.Write([]byte(base_string))
	signature = hex.EncodeToString(mac.Sum(nil))

	auth_code := fmt.Sprintf("%s|@@|%s|@@|%d|@@|%d|@@|%s",
		base64.StdEncoding.EncodeToString([]byte(client_id)),
		base64.StdEncoding.EncodeToString([]byte(username)),
		timestamp, nonce, signature)

	postform := &url.Values{
		"client_id":     {client_id},
		"client_secret": {global.config.ClientSecret},
		"redirect_uri":  {global.config.RedirectURI},
		"scope":         {"*/*/*"},
		"grant_type":    {"authorization_code"},
		"code":          {auth_code},
	}

	if global.snoop {
		nfo.Stdout("\n--> ACTION: \"POST\" PATH: \"%s\"", path)
		for k, v := range *postform {
			if k == "grant_type" || k == "redirect_uri" || k == "scope" {
				nfo.Stdout("\\-> POST PARAM: %s VALUE: %s", k, v)
			} else {
				nfo.Stdout("\\-> POST PARAM: %s VALUE: [HIDDEN]", k)
			}
		}
	}

	req.Body = ioutil.NopCloser(bytes.NewReader([]byte(postform.Encode())))

	client := KWSession(username).NewClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if err := DecodeJSON(resp, &auth); err != nil {
		return nil, err
	}

	auth.Expires = auth.Expires + time.Now().Unix()
	return

}

// Set token for kiteworks.
func (s KWSession) SetToken(req *http.Request, clear bool) (err error) {
	var token *Auth

	id := fmt.Sprintf("kw_token:%s", s)

	found := global.db.Get("tokens", id, &token)

	// If we find a token, check if it's still valid within the next 5 minutes.
	if found && token != nil {
		if token.Expires < time.Now().Add(time.Duration(5*time.Minute)).Unix() {
			global.db.Unset("tokens", id)
			found = false
		}
	}

	if clear {
		found = false
	}

	if !found {
		token, err = NewToken(string(s))
		if err != nil {
			return err
		}
		global.db.CryptSet("tokens", id, &token)
	}

	if token != nil {
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}
	return nil
}

// kiteworks Client
func (s KWSession) NewClient() *api_call {
	var transport http.Transport

	if !global.config.SSLVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if proxy_host := global.config.ProxyURI; proxy_host != NONE {
		proxyURL, err := url.Parse(proxy_host)
		errchk(err)
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	transport.Dial = (&net.Dialer{
		Timeout: time.Second * 10,
	}).Dial

	transport.TLSHandshakeTimeout = time.Second * 10

	return &api_call{Client: &http.Client{Transport: &transport, Timeout: 0}}
}

func (c *api_call) Do(req *http.Request) (resp *http.Response, err error) {
	<-api_call_bank
	defer func() { api_call_bank <- struct{}{} }()

	resp, err = c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	err = respError(resp)
	return
}

// Decodes JSON response body to provided interface.
func DecodeJSON(resp *http.Response, output interface{}) (err error) {

	defer resp.Body.Close()

	var (
		snoop_output map[string]interface{}
		snoop_buffer bytes.Buffer
		body         io.Reader
	)

	resp.Body = iotimeout.NewReadCloser(resp.Body, timeout)

	if global.snoop {
		if output == nil {
			nfo.Stdout("<-- RESPONSE STATUS: %s", resp.Status)
			dec := json.NewDecoder(resp.Body)
			dec.Decode(&snoop_output)
			o, _ := json.MarshalIndent(&snoop_output, "", "  ")
			fmt.Fprintf(os.Stderr, "%s\n", string(o))
			return nil
		} else {
			nfo.Stdout("<-- RESPONSE STATUS: %s", resp.Status)
			body = io.TeeReader(resp.Body, &snoop_buffer)
		}
	} else {
		body = resp.Body
	}

	if output == nil {
		return nil
	}

	dec := json.NewDecoder(body)
	err = dec.Decode(output)
	if err == io.EOF {
		return nil
	}

	if err != nil {
		if global.snoop {
			txt := snoop_buffer.String()
			if err := snoop_request(&snoop_buffer); err != nil {
				nfo.Stdout(txt)
			}
			err = fmt.Errorf("I cannot understand what %s is saying: %s", resp.Request.Host, err.Error())
			return
		} else {
			err = fmt.Errorf("I cannot understand what %s is saying. (Try running %s --snoop): %s", resp.Request.Host, os.Args[0], err.Error())
			return
		}
	}

	if global.snoop {
		snoop_request(&snoop_buffer)
	}
	return
}

// Allows snooping of API Calls.
func snoop_request(body io.Reader) error {
	var snoop_generic map[string]interface{}
	dec := json.NewDecoder(body)
	if err := dec.Decode(&snoop_generic); err != nil {
		return err
	}
	if snoop_generic != nil {
		for v, _ := range snoop_generic {
			switch v {
			case "refresh_token":
				fallthrough
			case "access_token":
				snoop_generic[v] = "[HIDDEN]"
			}
		}
	}
	o, _ := json.MarshalIndent(&snoop_generic, "", "  ")
	fmt.Fprintf(os.Stderr, "%s\n", string(o))
	return nil
}

// Read Error messages out of responses.
func respError(resp *http.Response) (err error) {
	if resp == nil {
		return
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var (
		snoop_buffer bytes.Buffer
		body         io.Reader
	)

	resp.Body = iotimeout.NewReadCloser(resp.Body, timeout)

	if global.snoop {
		nfo.Stdout("<-- RESPONSE STATUS: %s", resp.Status)
		body = io.TeeReader(resp.Body, &snoop_buffer)
	} else {
		body = resp.Body
	}

	// kiteworks API Error
	type KiteErr struct {
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
		Errors    []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}

	output, err := ioutil.ReadAll(body)

	if global.snoop {
		snoop_request(&snoop_buffer)
	}

	if err != nil {
		return err
	}

	var kite_err *KiteErr
	json.Unmarshal(output, &kite_err)
	if kite_err != nil {
		e := NewRestError()
		for _, v := range kite_err.Errors {
			e.AddKWError(v.Code, v.Message)
		}
		if kite_err.ErrorDesc != NONE {
			e.AddKWError(kite_err.Error, kite_err.ErrorDesc)
		}
		return e
	}

	if resp.Status == "401 Unathorized" {
		e := NewRestError()
		e.AddKWError("ERR_AUTH_UNAUTHORIZED", "Unathorized Access Token")
		return e
	}

	return fmt.Errorf("%s says \"%s.\"", resp.Request.Host, resp.Status)
}
