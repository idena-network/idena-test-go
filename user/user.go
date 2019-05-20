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
}

func NewUser(client *client.Client, node *node.Node) *User {
	return &User{
		Client: client,
		Node:   node,
	}
}

func (user *User) GetInfo() string {
	return fmt.Sprintf("[User %v; %v]", user.Node.RpcPort, user.Address)
}
