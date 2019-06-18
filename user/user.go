package user

import (
	"fmt"
	"idena-test-go/client"
	"idena-test-go/node"
)

type User struct {
	Client      *client.Client
	Node        *node.Node
	Address     string
	Enode       string
	TestContext *TestContext
	Index       int
	Active      bool
}

type TestContext struct {
	ShortFlipHashes []client.FlipHashesResponse
	ShortFlips      []client.FlipResponse
	LongFlipHashes  []client.FlipHashesResponse
	LongFlips       []client.FlipResponse
}

func NewUser(client *client.Client, node *node.Node, index int) *User {
	return &User{
		Client: client,
		Node:   node,
		Index:  index,
	}
}

func (u *User) GetInfo() string {
	return fmt.Sprintf("[User %d-%d]", u.Index, u.Node.RpcPort)
}

func (u *User) Start(mode node.StartMode) {
	u.Node.Start(mode)
	u.Active = true
}

func (u *User) Stop() {
	u.Node.Stop()
	u.Active = false
}
