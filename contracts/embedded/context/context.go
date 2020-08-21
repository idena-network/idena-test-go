package context

import (
	"github.com/idena-network/idena-go/common"
)

type ContractsContext struct {
	contractAddressesById map[int]common.Address
	contractCodesById     map[int]string
}

func NewContractsContext() *ContractsContext {
	return &ContractsContext{
		contractAddressesById: make(map[int]common.Address),
		contractCodesById:     make(map[int]string),
	}
}

func (c *ContractsContext) SetContractAddress(id int, address common.Address) {
	c.contractAddressesById[id] = address
}

func (c *ContractsContext) GetContractAddressById(id int) (common.Address, bool) {
	address, ok := c.contractAddressesById[id]
	return address, ok
}

func (c *ContractsContext) SetContractCode(id int, code string) {
	c.contractCodesById[id] = code
}

func (c *ContractsContext) GetContractCodeById(id int) (string, bool) {
	code, ok := c.contractCodesById[id]
	return code, ok
}
