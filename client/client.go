package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-test-go/events"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTimeoutSec = 10

type Client struct {
	index  int
	url    string
	apiKey string
	mutex  sync.Mutex
	bus    eventbus.Bus
}

func NewClient(nodeRpcPort int, index int, apiKey string, bus eventbus.Bus) *Client {
	return &Client{
		url:    "http://localhost:" + strconv.Itoa(nodeRpcPort) + "/",
		apiKey: apiKey,
		bus:    bus,
		index:  index,
	}
}

func (client *Client) GetEpoch() (Epoch, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_epoch",
		Key:    client.apiKey,
	}
	epoch := Epoch{}
	resp := response{Result: &epoch}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:    client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetPubKey() (string, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_getPublicKey",
		Key:    client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:    client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetIdentity(addr string) (api.Identity, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_identity",
		Payload: []string{addr},
		Key:     client.apiKey,
	}
	var identity api.Identity
	resp := response{Result: &identity}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return api.Identity{}, err
	}
	if resp.Error != nil {
		return api.Identity{}, errors.New(resp.Error.Message)
	}
	return identity, nil
}

func (client *Client) SendInvite(to string, amount float32) (Invite, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := sendInviteArgs{
		To:     to,
		Amount: amount,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_sendInvite",
		Payload: []sendInviteArgs{params},
		Key:     client.apiKey,
	}
	invite := Invite{}
	resp := response{Result: &invite}
	if err := client.sendRequestAndParseResponse(req, 0, false, &resp); err != nil {
		return Invite{}, err
	}
	if resp.Error != nil {
		return Invite{}, errors.New(resp.Error.Message)
	}
	return invite, nil
}

func (client *Client) ActivateInvite(params api.ActivateInviteArgs) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	req := request{
		Id:      client.getReqId(),
		Method:  "dna_activateInvite",
		Payload: []api.ActivateInviteArgs{params},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 0, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) SubmitFlip(privateHex, publicHex string, wordPairIdx uint8) (FlipSubmitResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := flipSubmitArgs{
		PrivateHex: privateHex,
		PublicHex:  publicHex,
		Pair:       wordPairIdx,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_submit",
		Payload: []flipSubmitArgs{params},
		Key:     client.apiKey,
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

func (client *Client) DeleteFlip(hash string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	req := request{
		Id:      client.getReqId(),
		Method:  "flip_delete",
		Payload: []string{hash},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 0, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetShortFlipHashes(address string) ([]FlipHashesResponse, error) {
	return client.getFlipHashes(address, "flip_shortHashes")
}

func (client *Client) getFlipHashes(address, method string) ([]FlipHashesResponse, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Key:     client.apiKey,
		Payload: []string{address},
	}
	var hashes []FlipHashesResponse
	resp := response{Result: &hashes}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:     client.apiKey,
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

func (client *Client) GetFlipWords(hash string) (api.FlipWordsResponse, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_words",
		Payload: []string{hash},
		Key:     client.apiKey,
	}
	flipWordsResponse := api.FlipWordsResponse{}
	resp := response{Result: &flipWordsResponse}
	if err := client.sendRequestAndParseResponse(req, 15, true, &resp); err != nil {
		return api.FlipWordsResponse{}, err
	}
	if resp.Error != nil {
		return api.FlipWordsResponse{}, errors.New(resp.Error.Message)
	}
	return flipWordsResponse, nil
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
		Key:     client.apiKey,
	}
	submitResp := SubmitAnswersResponse{}
	resp := response{Result: &submitResp}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return SubmitAnswersResponse{}, err
	}
	if resp.Error != nil {
		return SubmitAnswersResponse{}, errors.New(resp.Error.Message)
	}
	return submitResp, nil
}

func (client *Client) GetLongFlipHashes(address string) ([]FlipHashesResponse, error) {
	return client.getFlipHashes(address, "flip_longHashes")
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
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
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
		Type:       txType,
		PayloadHex: payloadHex,
	}
	if maxFee > 0 {
		params.MaxFee = &maxFee
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_sendTransaction",
		Payload: []sendTxArgs{params},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) getReqId() int {
	return 1
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

	counter := 5
	for {
		httpReq, err := http.NewRequest("POST", client.url, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, errors.Wrapf(err, "unable to create request")
		}
		httpReq.Header.Set("Content-Type", "application/json")

		httpClient := &http.Client{
			Timeout: time.Second * time.Duration(timeoutSec),
		}
		resp, err := httpClient.Do(httpReq)
		if err == nil {
			var respBody []byte
			respBody, err = ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err == nil {
				return respBody, nil
			}
			err = errors.Wrapf(err, "unable to read response")
		}
		isCrashed := isNodeCrashed(err)
		if isCrashed {
			if client.bus != nil {
				client.bus.Publish(&events.NodeCrashedEvent{
					Index: client.index,
				})
			}
		} else {
			counter--
		}
		if counter > 0 && retry {
			log.Warn(fmt.Sprintf("%v. Retrying to send request (%v) due to error %v", client.url, req.Method, err))
			time.Sleep(time.Millisecond * 50)
			continue
		}
		return nil, errors.Wrapf(err, "unable to send request")
	}
}

func isNodeCrashed(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "target machine actively refused it") ||
		strings.Contains(err.Error(), "connection refused"))
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
		Key:    client.apiKey,
	}
	ceremonyIntervals := CeremonyIntervals{}
	resp := response{Result: &ceremonyIntervals}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:    client.apiKey,
	}
	var peers []Peer
	resp := response{Result: &peers}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:     client.apiKey,
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
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
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
		Key:    client.apiKey,
	}
	var res []BurntCoins
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
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
		Key:     client.apiKey,
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
		Key:     client.apiKey,
	}
	var res ProfileResponse
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return ProfileResponse{}, err
	}
	if resp.Error != nil {
		return ProfileResponse{}, errors.New(resp.Error.Message)
	}
	return res, nil
}

func (client *Client) GetFlipRaw(hash string) (FlipResponse2, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_getRaw",
		Payload: []string{hash},
		Key:     client.apiKey,
	}
	flipResponse := FlipResponse2{}
	resp := response{Result: &flipResponse}
	if err := client.sendRequestAndParseResponse(req, 5, false, &resp); err != nil {
		return FlipResponse2{}, err
	}
	if resp.Error != nil {
		return FlipResponse2{}, errors.New(resp.Error.Message)
	}
	return flipResponse, nil
}

func (client *Client) GetFlipKeys(address, hash string) (FlipKeysResponse, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_getKeys",
		Payload: []string{address, hash},
		Key:     client.apiKey,
	}
	flipKeysResponse := FlipKeysResponse{}
	resp := response{Result: &flipKeysResponse}
	if err := client.sendRequestAndParseResponse(req, 15, true, &resp); err != nil {
		return FlipKeysResponse{}, err
	}
	if resp.Error != nil {
		return FlipKeysResponse{}, errors.New(resp.Error.Message)
	}
	return flipKeysResponse, nil
}

func (client *Client) CheckSyncing() (api.Syncing, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "bcn_syncing",
		Key:    client.apiKey,
	}
	syncingResponse := api.Syncing{}
	resp := response{Result: &syncingResponse}
	if err := client.sendRequestAndParseResponse(req, 15, true, &resp); err != nil {
		return api.Syncing{}, err
	}
	if resp.Error != nil {
		return api.Syncing{}, errors.New(resp.Error.Message)
	}
	return syncingResponse, nil
}

func (client *Client) Transaction(hash string) (api.Transaction, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "bcn_transaction",
		Payload: []string{hash},
		Key:     client.apiKey,
	}
	res := api.Transaction{}
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, 15, false, &resp); err != nil {
		return api.Transaction{}, err
	}
	if resp.Error != nil {
		return api.Transaction{}, errors.New(resp.Error.Message)
	}
	return res, nil
}

func (client *Client) AddIpfsData(dataHex string, pin bool) (string, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "ipfs_add",
		Payload: []interface{}{dataHex, pin},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 60, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) IpfsDataCid(dataHex string) (string, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "ipfs_cid",
		Payload: []interface{}{dataHex},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, 60, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) StoreToIpfs(cid string) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := storeToIpfsTxArgs{
		Cid: cid,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_storeToIpfs",
		Payload: []storeToIpfsTxArgs{params},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) SendRawTx(raw []byte) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	dataHex := hexutil.Encode(raw)
	req := request{
		Id:      client.getReqId(),
		Method:  "bcn_sendRawTx",
		Payload: []string{dataHex},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) GetRawTx(txType uint16, from string, to *string, amount, maxFee float32, payload []byte) (string, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	var payloadHex *string
	if len(payload) > 0 {
		ph := hexutil.Encode(payload)
		payloadHex = &ph
	}

	params := sendTxArgs{
		From:       from,
		To:         to,
		Amount:     amount,
		Type:       txType,
		PayloadHex: payloadHex,
		UseProto:   true,
	}
	if maxFee > 0 {
		params.MaxFee = &maxFee
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "bcn_getRawTx",
		Payload: []sendTxArgs{params},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(resp.Error.Message)
	}
	return resp.Result.(string), nil
}

func (client *Client) WordsSeed() (hexutil.Bytes, error) {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_wordsSeed",
		Key:    client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return hexutil.Bytes{}, err
	}
	if resp.Error != nil {
		return hexutil.Bytes{}, errors.New(resp.Error.Message)
	}
	return hexutil.Decode(resp.Result.(string))
}

func (client *Client) SubmitRawFlip(tx *hexutil.Bytes, encryptedPublicHex *hexutil.Bytes, encryptedPrivateHex *hexutil.Bytes) (FlipSubmitResponse, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	params := api.RawFlipSubmitArgs{
		Tx:                  tx,
		EncryptedPrivateHex: encryptedPrivateHex,
		EncryptedPublicHex:  encryptedPublicHex,
	}
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_rawSubmit",
		Payload: []api.RawFlipSubmitArgs{params},
		Key:     client.apiKey,
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

func (client *Client) WordPairs(addr string, vrfHash hexutil.Bytes) ([]api.FlipWords, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_wordPairs",
		Payload: []interface{}{addr, vrfHash},
		Key:     client.apiKey,
	}
	var res []api.FlipWords
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return res, nil
}

func (client *Client) SendPublicEncryptionKey(args api.EncryptionKeyArgs) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	req := request{
		Id:      client.getReqId(),
		Method:  "flip_sendPublicEncryptionKey",
		Payload: []api.EncryptionKeyArgs{args},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	return nil
}

func (client *Client) SendPrivateEncryptionKeysPackage(args api.EncryptionKeyArgs) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	req := request{
		Id:      client.getReqId(),
		Method:  "flip_sendPrivateEncryptionKeysPackage",
		Payload: []api.EncryptionKeyArgs{args},
		Key:     client.apiKey,
	}
	resp := response{}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	return nil
}

func (client *Client) PrivateEncryptionKeyCandidates(addr common.Address) ([]hexutil.Bytes, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  "flip_privateEncryptionKeyCandidates",
		Payload: []interface{}{addr},
		Key:     client.apiKey,
	}
	var res []hexutil.Bytes
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, true, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return res, nil
}
