package apiclient

import (
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
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
	resp := c.sendRequest("/api/GetGodAddress", nil)
	return string(resp), nil
}

func (c *Client) GetBootNode() (string, error) {
	resp := c.sendRequest("/api/GetBootNode", nil)
	return string(resp), nil
}

func (c *Client) GetIpfsBootNode() (string, error) {
	resp := c.sendRequest("/api/GetIpfsBootNode", nil)
	return string(resp), nil
}

func (c *Client) CreateInvite(address string) error {
	c.sendRequest("/api/CreateInvite", map[string]string{
		"address": address,
	})
	return nil
}

func (c *Client) GetCeremonyTime() (int64, error) {
	resp := c.sendRequest("/api/GetCeremonyTime", nil)
	ceremonyTime, err := strconv.ParseInt(string(resp), 10, 64)
	if err != nil {
		return 0, err
	}
	return ceremonyTime, nil
}

func (c *Client) sendRequest(path string, params map[string]string) []byte {
	strParams := ""
	if len(params) > 0 {
		strParams += "?"
		for k, v := range params {
			strParams += fmt.Sprintf("%s=%s&", k, v)
		}
		strParams = strParams[0 : len(strParams)-1]
	}
	url := c.url + path + strParams

	log.Trace(fmt.Sprintf("Send request to API: %s", url))

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	var resp *http.Response
	counter := 10
	for {
		counter--
		httpClient := &http.Client{}
		resp, err = httpClient.Do(httpReq)
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if counter > 0 {
			time.Sleep(time.Second)
			continue
		}
		panic(err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return respBody
}
