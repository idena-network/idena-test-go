package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	url         string
	reqIdHolder *ReqIdHolder
	mutex       sync.Mutex
}

func NewClient(node node.Node, reqIdHolder *ReqIdHolder) *Client {
	return &Client{
		url:         "http://localhost:" + strconv.Itoa(node.RpcPort) + "/",
		reqIdHolder: reqIdHolder,
	}
}

func (client *Client) GetEpoch() (Epoch, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_epoch",
	}
	epoch := Epoch{}
	resp := response{Result: &epoch}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return Epoch{}, err
	}
	if resp.Error != nil {
		return Epoch{}, errors.New(resp.Error.Message)
	}
	return epoch, nil
}

func (client *Client) GetCoinbaseAddr() (string, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_getCoinbaseAddr",
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetEnode() (string, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "net_enode",
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetIpfsAddress() (string, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "net_ipfsAddress",
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetIdentities() ([]Identity, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_identities",
	}
	var identities []Identity
	resp := response{Result: &identities}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return identities, nil
}

func (client *Client) GetIdentity(addr string) (Identity, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_identity",
		Payload: []string{addr},
	}
	var identity Identity
	resp := response{Result: &identity}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return Identity{}, err
	}
	if resp.Error != nil {
		return Identity{}, errors.New(resp.Error.Message)
	}
	return identity, nil
}

func (client *Client) SendInvite(to string) (Invite, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := sendInviteArgs{
		To: to,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_sendInvite",
		Payload: []sendInviteArgs{params},
	}
	invite := Invite{}
	resp := response{Result: &invite}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return Invite{}, err
	}
	if resp.Error != nil {
		return Invite{}, errors.New(resp.Error.Message)
	}
	return invite, nil
}

func (client *Client) ActivateInvite(to string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := activateInviteArgs{
		To: to,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_activateInvite",
		Payload: []activateInviteArgs{params},
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) SubmitFlip(hex string, wordPairIdx uint8) (FlipSubmitResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := flipSubmitArgs{
		Hex:  hex,
		Pair: wordPairIdx,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_submit",
		Payload: []flipSubmitArgs{params},
	}
	submitResp := FlipSubmitResponse{}
	resp := response{Result: &submitResp}
	if err := client.sendRequestAndParseResponse(req, 0, false, &resp); err != nil {
		return FlipSubmitResponse{}, err
	}
	if resp.Error != nil {
		return FlipSubmitResponse{}, errors.New(resp.Error.Message)
	}
	return submitResp, nil
}

func (client *Client) GetShortFlipHashes() ([]FlipHashesResponse, error) {
	return client.getFlipHashes("flip_shortHashes")
}

func (client *Client) getFlipHashes(method string) ([]FlipHashesResponse, error) {
	req := request{
		Id:     client.getReqId(),
		Method: method,
	}
	var hashes []FlipHashesResponse
	resp := response{Result: &hashes}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return hashes, nil
}

func (client *Client) GetFlip(hash string) (FlipResponse, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_get",
		Payload: []string{hash},
	}
	flipResponse := FlipResponse{}
	resp := response{Result: &flipResponse}
	if err := client.sendRequestAndParseResponse(req, 15, true, &resp); err != nil {
		return FlipResponse{}, err
	}
	if resp.Error != nil {
		return FlipResponse{}, errors.New(resp.Error.Message)
	}
	return flipResponse, nil
}

func (client *Client) SubmitShortAnswers(answers []FlipAnswer) (SubmitAnswersResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	return client.submitAnswers(answers, "flip_submitShortAnswers")
}

func (client *Client) submitAnswers(answers []FlipAnswer, method string) (SubmitAnswersResponse, error) {
	params := submitAnswersArgs{}
	for _, a := range answers {
		params.Answers = append(params.Answers, a)
	}
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Payload: []submitAnswersArgs{params},
	}
	submitResp := SubmitAnswersResponse{}
	resp := response{Result: &submitResp}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return SubmitAnswersResponse{}, err
	}
	if resp.Error != nil {
		return SubmitAnswersResponse{}, errors.New(resp.Error.Message)
	}
	return submitResp, nil
}

func (client *Client) GetLongFlipHashes() ([]FlipHashesResponse, error) {
	return client.getFlipHashes("flip_longHashes")
}

func (client *Client) SubmitLongAnswers(answers []FlipAnswer) (SubmitAnswersResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	return client.submitAnswers(answers, "flip_submitLongAnswers")
}

func (client *Client) BecomeOnline() (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	return client.becomeOnline(true)
}

func (client *Client) BecomeOffline() (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	return client.becomeOnline(false)
}

func (client *Client) becomeOnline(online bool) (string, error) {
	var method string
	if online {
		method = "dna_becomeOnline"
	} else {
		method = "dna_becomeOffline"
	}
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Payload: []struct{}{{}},
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) SendTransaction(txType uint16, from string, to *string, amount, maxFee float32, payloadHex *string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := sendTxArgs{
		From:       from,
		To:         to,
		Amount:     amount,
		MaxFee:     maxFee,
		Type:       txType,
		PayloadHex: payloadHex,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_sendTransaction",
		Payload: []sendTxArgs{params},
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) getReqId() int {
	return client.reqIdHolder.GetNextReqId()
}

func cut(text string, limit int) string {
	runes := []rune(text)
	if len(runes) >= limit {
		return string(runes[:limit])
	}
	return text
}

func (client *Client) sendRequest(req request, timeoutSec int, retry bool) ([]byte, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to serialize request")
	}

	log.Trace(fmt.Sprintf("%v. Send request: %v", client.url, cut(string(reqBody), 500)))

	var resp *http.Response
	defer func() {
		if resp == nil || resp.Body == nil {
			return
		}
		resp.Body.Close()
	}()
	counter := 5
	for {
		httpReq, err := http.NewRequest("POST", client.url, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create request")
		}
		httpReq.Header.Set("Content-Type", "application/json")

		counter--
		httpClient := &http.Client{
			Timeout: time.Second * time.Duration(timeoutSec),
		}
		resp, err = httpClient.Do(httpReq)
		if err == nil {
			break
		}
		if counter > 0 && retry {
			log.Warn(fmt.Sprintf("%v. Retrying to send request due to error %v", client.url, err))
			time.Sleep(time.Millisecond * 50)
			continue
		}
		return nil, errors.Wrapf(err, "unable to send request")
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read response")
	}
	return respBody, nil
}

func (client *Client) sendRequestAndParseResponse(req request, timeoutSec int, retry bool, resp *response) error {
	responseBytes, err := client.sendRequest(req, timeoutSec, retry)
	if err != nil {
		return err
	}

	log.Trace(fmt.Sprintf("%v. Got response: %v", client.url, cut(string(responseBytes), 500)))

	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		return errors.Wrapf(err, "unable to deserialize response")
	}
	return nil
}

func (client *Client) CeremonyIntervals() (CeremonyIntervals, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_ceremonyIntervals",
	}
	ceremonyIntervals := CeremonyIntervals{}
	resp := response{Result: &ceremonyIntervals}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return CeremonyIntervals{}, err
	}
	if resp.Error != nil {
		return CeremonyIntervals{}, errors.New(resp.Error.Message)
	}
	return ceremonyIntervals, nil
}

func (client *Client) GetPeers() ([]Peer, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "net_peers",
	}
	var peers []Peer
	resp := response{Result: &peers}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return peers, nil
}

func (client *Client) AddPeer(url string) error {
	req := request{
		Id:      client.getReqId(),
		Method:  "net_addPeer",
		Payload: []string{url},
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 1, false, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	return nil
}

func (client *Client) Burn(from string, amount, maxFee float32, key string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := burnArgs{
		From:   from,
		Amount: amount,
		MaxFee: maxFee,
		Key:    key,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_burn",
		Payload: []burnArgs{params},
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) BurntCoins() ([]BurntCoins, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "bcn_burntCoins",
	}
	var res []BurntCoins
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return res, nil
}

func (client *Client) ChangeProfile(nickname *string, info []byte) (ChangeProfileResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := changeProfileArgs{}
	if len(info) > 0 {
		hex := hexutil.Bytes(info)
		params.Info = &hex
	}
	if nickname != nil {
		params.Nickname = nickname
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_changeProfile",
		Payload: []changeProfileArgs{params},
	}
	res := ChangeProfileResponse{}
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, 0, false, &resp); err != nil {
		return ChangeProfileResponse{}, err
	}
	if resp.Error != nil {
		return ChangeProfileResponse{}, errors.New(resp.Error.Message)
	}
	return res, nil
}

func (client *Client) GetProfile(address string) (ProfileResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	req := request{
		Id:      client.getReqId(),
		Method:  "dna_profile",
		Payload: []string{address},
	}
	var res ProfileResponse
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, 5, true, &resp); err != nil {
		return ProfileResponse{}, err
	}
	if resp.Error != nil {
		return ProfileResponse{}, errors.New(resp.Error.Message)
	}
	return res, nil
}
