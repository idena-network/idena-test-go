package apiclient

import (
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout  = time.Second * 15
	defaultAttempts = 10
)

type Client struct {
	url string
}

func NewClient(url string) *Client {
	return &Client{
		url: url,
	}
}

func (c *Client) GetGodAddress() (string, error) {
	resp, err := c.sendRequest("/api/GetGodAddress", defaultTimeout, defaultAttempts, nil)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) GetBootNode() (string, error) {
	resp, err := c.sendRequest("/api/GetBootNode", defaultTimeout, defaultAttempts, nil)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) GetIpfsBootNode() (string, error) {
	resp, err := c.sendRequest("/api/GetIpfsBootNode", defaultTimeout, defaultAttempts, nil)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

func (c *Client) CreateInvites(addresses []string) ([]string, error) {
	resp, err := c.sendRequest("/api/CreateInvites", time.Second*120, 1, map[string]string{
		"addresses": strings.Join(addresses, ","),
	})
	if err != nil {
		return nil, err
	}
	stringResp := string(resp)
	if stringResp == "OK" {
		return nil, nil
	}
	delegatees := strings.Split(stringResp, ",")
	return delegatees, err
}

func (c *Client) GetCeremonyTime() (int64, error) {
	resp, err := c.sendRequest("/api/GetCeremonyTime", defaultTimeout, defaultAttempts, nil)
	if err != nil {
		return 0, err
	}
	ceremonyTime, err := strconv.ParseInt(string(resp), 10, 64)
	if err != nil {
		return 0, err
	}
	return ceremonyTime, nil
}

func (c *Client) GetEpoch() (uint16, error) {
	resp, err := c.sendRequest("/api/GetEpoch", defaultTimeout, defaultAttempts, nil)
	if err != nil {
		return 0, err
	}
	epoch, err := strconv.Atoi(string(resp))
	if err != nil {
		return 0, err
	}
	return uint16(epoch), nil
}

func (c *Client) SendFailNotification(message string) error {
	_, err := c.sendRequest("/api/SendFailNotification", defaultTimeout, defaultAttempts, map[string]string{
		"message": message,
	})
	return err
}

func (c *Client) SendWarnNotification(message string) error {
	_, err := c.sendRequest("/api/SendWarnNotification", defaultTimeout, defaultAttempts, map[string]string{
		"message": message,
	})
	return err
}

func (c *Client) sendRequest(path string, timeout time.Duration, attempts int, params map[string]string) ([]byte, error) {
	if params == nil {
		params = make(map[string]string)
	}
	params["reqId"] = strconv.Itoa(rand.Int())
	urlValues := url.Values{}
	if len(params) > 0 {
		for k, v := range params {
			urlValues.Add(k, v)
		}
	}
	fullUrl := c.url + path + "?" + urlValues.Encode()

	log.Trace(fmt.Sprintf("Sending request %s to API: %s", params["reqId"], fullUrl))
	defer log.Trace(fmt.Sprintf("Sent request %s to API", params["reqId"]))

	httpReq, err := http.NewRequest("GET", fullUrl, nil)
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	defer func() {
		if resp == nil || resp.Body == nil {
			return
		}
		resp.Body.Close()
	}()
	counter := attempts
	for {
		counter--
		httpClient := &http.Client{
			Timeout: timeout,
		}
		resp, err = httpClient.Do(httpReq)
		if err == nil && resp.StatusCode != http.StatusOK {
			err = errors.New(fmt.Sprintf("resp code %v", resp.StatusCode))
		}
		if err == nil {
			break
		}
		if counter > 0 {
			log.Warn(fmt.Sprintf("Retrying to send request %v due to error %v", params["reqId"], err))
			time.Sleep(time.Second)
			continue
		}
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}
