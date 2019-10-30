package common

import (
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"
)

func WaitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return true
	case <-time.After(timeout):
		return false
	}
}

func LoadData(url string) ([]byte, error) {
	var resp *http.Response
	defer func() {
		if resp == nil || resp.Body == nil {
			return
		}
		resp.Body.Close()
	}()
	counter := 5
	for {
		httpReq, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create request to %s", url)
		}
		counter--
		httpClient := &http.Client{
			Timeout: time.Second * 10,
		}
		resp, err = httpClient.Do(httpReq)
		if err == nil {
			break
		}
		if counter > 0 {
			time.Sleep(time.Millisecond * 5)
			continue
		}
		return nil, errors.Wrapf(err, "unable to send request to %s", url)
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read response from %s", url)
	}
	return respBody, nil
}

func IsValidUrl(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	} else {
		return true
	}
}
