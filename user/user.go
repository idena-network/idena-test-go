package user

import (
	"fmt"
	"idena-test-go/client"
	"idena-test-go/node"
)

type User struct {
	Client          *client.Client
	Node            *node.Node
	Address         string
	ShortFlipHashes []client.FlipHashesResponse
	ShortFlips      []client.FlipResponse
	LongFlipHashes  []client.FlipHashesResponse
	LongFlips       []client.FlipResponse
	index           int
}

func NewUser(client *client.Client, node *node.Node, index int) *User {
	return &User{
		Client: client,
		Node:   node,
		index:  index,
	}
}

func (user *User) GetInfo() string {
	return fmt.Sprintf("[User %v; %v; %v]", user.index, user.Address, user.Node.RpcPort)
}
