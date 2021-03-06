package user

import (
	"fmt"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"time"
)

type User struct {
	Client            *client.Client
	Node              *node.Node
	Address           string
	PubKey            string
	TestContext       *TestContext
	Index             int
	Active            bool
	SentAutoOnline    bool
	MultiBotDelegatee *string
	IsTestRun         bool
}

type TestContext struct {
	ShortFlipHashes []client.FlipHashesResponse
	LongFlipHashes  []client.FlipHashesResponse
	TestStartTime   time.Time
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

func (u *User) Start(mode node.StartMode) error {
	if err := u.Node.Start(mode); err != nil {
		return err
	}
	if err := u.waitForSync(); err != nil {
		return err
	}
	u.Active = true
	return nil
}

func (u *User) waitForSync() error {
	for {
		syncing, err := u.Client.CheckSyncing()
		if err != nil {
			return err
		}
		if syncing.Syncing {
			log.Info(fmt.Sprintf("%s syncing", u.GetInfo()))
			time.Sleep(time.Second * 10)
		}
		return nil
	}
}

func (u *User) Stop() error {
	u.Active = false
	if err := u.Node.Stop(); err != nil {
		return err
	}
	return nil
}
