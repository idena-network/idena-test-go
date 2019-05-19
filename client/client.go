package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"idena-test-go/log"
	"idena-test-go/node"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	url         string
	reqIdHolder *ReqIdHolder
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return Identity{}, errors.New(resp.Error.Message)
	}
	return identity, nil
}

func (client *Client) SendInvite(to string) (Invite, error) {
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
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return Invite{}, errors.New(resp.Error.Message)
	}
	return invite, nil
}

func (client *Client) ActivateInvite(to string) (string, error) {
	params := activateInviteArgs{
		To: to,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_activateInvite",
		Payload: []activateInviteArgs{params},
	}
	resp := response{}
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) SubmitFlip(hex string) (FlipSubmitResponse, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_submit",
		Payload: []string{hex},
	}
	submitResp := FlipSubmitResponse{}
	resp := response{Result: &submitResp}
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
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
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return FlipResponse{}, errors.New(resp.Error.Message)
	}
	return flipResponse, nil
}

func (client *Client) SubmitShortAnswers(answers []byte) (SubmitAnswersResponse, error) {
	return client.submitAnswers(answers, "flip_submitShortAnswers")
}

func (client *Client) submitAnswers(answers []byte, method string) (SubmitAnswersResponse, error) {
	params := submitAnswersArgs{}
	for _, a := range answers {
		params.Answers = append(params.Answers, FlipAnswer{false, a})
	}
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Payload: []submitAnswersArgs{params},
	}
	submitResp := SubmitAnswersResponse{}
	resp := response{Result: &submitResp}
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return SubmitAnswersResponse{}, errors.New(resp.Error.Message)
	}
	return submitResp, nil
}

func (client *Client) GetLongFlipHashes() ([]FlipHashesResponse, error) {
	return client.getFlipHashes("flip_longHashes")
}

func (client *Client) SubmitLongAnswers(answers []byte) (SubmitAnswersResponse, error) {
	return client.submitAnswers(answers, "flip_submitLongAnswers")
}

func (client *Client) getReqId() int {
	return client.reqIdHolder.GetNextReqId()
}

func (client *Client) sendRequest(req request) []byte {
	reqBody, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	log.Trace(fmt.Sprintf("%v. Send request: %v", client.url, bytes.NewBuffer(reqBody).String()))

	httpReq, err := http.NewRequest("POST", client.url, bytes.NewBuffer(reqBody))
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	counter := 5
	for {
		counter--
		httpClient := &http.Client{}
		resp, err = httpClient.Do(httpReq)
		if err == nil {
			break
		}
		if counter > 0 {
			time.Sleep(time.Millisecond * 50)
			continue
		}
		if err != nil {
			panic(err)
		}
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	return respBody
}

func (client *Client) sendRequestAndParseResponse(req request, resp *response) {
	responseBytes := client.sendRequest(req)

	log.Trace(fmt.Sprintf("%v. Got response: %v", client.url, string(responseBytes)))

	if err := json.Unmarshal(responseBytes, &resp); err != nil {
		panic(err)
	}
}

func (client *Client) CeremonyIntervals() (CeremonyIntervals, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_ceremonyIntervals",
	}
	ceremonyIntervals := CeremonyIntervals{}
	resp := response{Result: &ceremonyIntervals}
	client.sendRequestAndParseResponse(req, &resp)
	if resp.Error != nil {
		return CeremonyIntervals{}, errors.New(resp.Error.Message)
	}
	return ceremonyIntervals, nil
}
