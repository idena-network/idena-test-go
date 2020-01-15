package alive

import (
	"fmt"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

const (
	DisableEventID = eventbus.EventID("disable")
)

type DisableEvent struct {
}

func (e *DisableEvent) EventID() eventbus.EventID {
	return DisableEventID
}

type Manager interface {
	Enabled() (bool, error)
	Disable() error
}

func NewManager(url string) Manager {
	return &managerImpl{
		url: url,
	}
}

func NewEmptyManager() Manager {
	return &emptyManager{}
}

type emptyManager struct {
}

func (m *emptyManager) Enabled() (bool, error) {
	return true, nil
}

func (m *emptyManager) Disable() error {
	return nil
}

type managerImpl struct {
	url string
}

func (m *managerImpl) Enabled() (bool, error) {
	req := m.url + "/enabled"
	resp, err := m.sendRequest(req)
	if err != nil {
		return false, errors.Wrapf(err, "unable to send request %s", req)
	}
	return "1" == string(resp), nil
}

func (m *managerImpl) Disable() error {
	req := m.url + "/disable"
	_, err := m.sendRequest(req)
	return errors.Wrapf(err, "unable to send request %s", req)
}

func (m *managerImpl) sendRequest(url string) ([]byte, error) {
	httpReq, err := http.NewRequest("GET", url, nil)
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
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	resp, err = httpClient.Do(httpReq)
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf("resp code %v", resp.StatusCode))
	}
	if err != nil {
		return nil, err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

type Monitoring struct {
	interval time.Duration
	manager  Manager
	bus      eventbus.Bus
}

func NewMonitoring(manager Manager, interval time.Duration, bus eventbus.Bus) *Monitoring {
	return &Monitoring{
		manager:  manager,
		interval: interval,
		bus:      bus,
	}
}

func (m *Monitoring) Start() error {
	enabled, err := m.enabled()
	if err != nil {
		return err
	}
	if !enabled {
		return errors.New("bot is disabled")
	}
	go m.monitor()
	return nil
}

func (m *Monitoring) monitor() {
	for {
		time.Sleep(m.interval)
		m.check()
	}
}

func (m *Monitoring) check() {
	enabled, err := m.enabled()
	if err != nil {
		log.Warn(errors.Wrap(err, "Unable to check bot alive state").Error())
		return
	}
	if enabled {
		return
	}
	m.bus.Publish(&DisableEvent{})
	os.Exit(0)
}

func (m *Monitoring) enabled() (bool, error) {
	return m.manager.Enabled()
}
