package client

import (
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common"
	"github.com/pkg/errors"
)

func (client *Client) ContractCallEstimate(method string, payload interface{}) (*api.TxReceipt, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Payload: payload,
		Key:     client.apiKey,
	}
	res := api.TxReceipt{}
	resp := response{Result: &res}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return &res, nil
}

func (client *Client) ContractCall(method string, payload interface{}) (string, error) {
	req := request{
		Id:      client.getReqId(),
		Method:  method,
		Payload: payload,
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

func (client *Client) ReadonlyCallContract(payload interface{}, result interface{}) error {
	req := request{
		Id:      client.getReqId(),
		Method:  "dna_readonlyCallContract",
		Payload: payload,
		Key:     client.apiKey,
	}
	resp := response{}
	if result != nil {
		resp.Result = result
	}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	return nil
}

func (client *Client) ReadContractData(contractAddress common.Address, key string, format string, result interface{}) error {
	req := request{
		Id:     client.getReqId(),
		Method: "dna_readContractData",
		Payload: []interface{}{
			contractAddress,
			key,
			format,
		},
		Key: client.apiKey,
	}
	resp := response{}
	if result != nil {
		resp.Result = result
	}
	if err := client.sendRequestAndParseResponse(req, defaultTimeoutSec, false, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}
	return nil
}
