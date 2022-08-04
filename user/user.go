package user

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/crypto/ecies"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/pkg/errors"
	"time"
)

type User interface {
	GetIndex() int
	GetInfo() string
	IsActive() bool
	GetAddress() string
	GetPubKey() string
	SharedNode() bool

	GetTestContext() *TestContext
	SetTestContext(testContext *TestContext)

	IsTestRun() bool
	SetIsTestRun(isTestRun bool)

	SetMultiBotDelegatee(multiBotDelegatee *string)
	GetMultiBotDelegatee() *string

	GetAutoOnlineSent() bool
	SetAutoOnlineSent(sentAutoOnline bool)

	SetNodeExecCommandName(command string)

	Start(mode node.StartMode) error
	Stop() error
	UpdateNodeParameters(godAddress, ipfsBootNode string, ceremonyTime int64)
	DestroyNode() error
	InitAddress() error
	InitPubKey() error

	CeremonyIntervals() (client.CeremonyIntervals, error)
	GetIdentity(addr string) (api.Identity, error)
	GetIpfsAddress() (string, error)
	SendTransaction(txType uint16, to *string, amount, maxFee float32, payload []byte) (string, error)
	Transaction(hash string) (api.Transaction, error)
	SendInvite(to string, amount float32) (client.Invite, error)
	ActivateInvite() (string, error)
	GetRequiredFlipsInfo() (int, []FlipWords, error)
	SubmitFlip(privateHex, publicHex string, wordPairIdx uint8) (client.FlipSubmitResponse, error)
	SendPrivateFlipKeysPackages() error
	SendPublicFlipKey() error
	GetShortFlipHashes() ([]client.FlipHashesResponse, error)
	GetLongFlipHashes() ([]client.FlipHashesResponse, error)
	GetFlip(decryptFlips bool, hash string) (client.FlipResponse, error)
	SubmitShortAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error)
	SubmitOpenShortAnswers() (string, error)
	SubmitLongAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error)
	BecomeOnline() (string, error)
	BecomeOffline() (string, error)
	GetEpoch() (client.Epoch, error)
	GetFlipWords(hash string) (api.FlipWordsResponse, error)

	AddIpfsData(dataHex string, pin bool) (string, error)
	StoreToIpfs(cid string) (string, error)

	GetPrivateKey() *ecdsa.PrivateKey
}

type FlipWords struct {
	Words [2]FlipWord
}

type FlipWord struct {
	Id          uint32
	Name        string
	Description string
}

type user struct {
	client            *client.Client
	node              *node.Node
	address           string
	pubKey            string
	testContext       *TestContext
	index             int
	active            bool
	sentAutoOnline    bool
	multiBotDelegatee *string
	isTestRun         bool
	ownNode           bool
}

func (u *user) GetIndex() int {
	return u.index
}

func (u *user) IsActive() bool {
	return u.active
}

type TestContext struct {
	Epoch                                     uint16
	ShortAnswersBytes                         []byte
	ShortFlipHashes, LongFlipHashes           []client.FlipHashesResponse
	WordsProof                                []byte
	PublicEncryptionKey, PrivateEncryptionKey *ecies.PrivateKey
	TestStartTime                             time.Time
	RequiredFlips, MadeFlips                  int
	FlipsByHash                               map[string]client.FlipResponse
}

func NewUser(parentUser User, client *client.Client, node *node.Node, index int) User {
	if parentUser == nil {
		return &user{
			client: client,
			node:   node,
			index:  index,
		}
	}
	u := &sharedNodeUser{
		user: &user{
			client: client,
			index:  index,
		},
		client:     client,
		parentUser: parentUser,
	}
	_ = u.InitAddress()
	return u
}

func (u *user) GetPrivateKey() *ecdsa.PrivateKey {
	return u.node.GetPrivateKey()
}

func (u *user) GetInfo() string {
	return fmt.Sprintf("[User %d-%d]", u.index, u.node.RpcPort)
}

func (u *user) Start(mode node.StartMode) error {
	if err := u.node.Start(mode); err != nil {
		return err
	}
	if err := u.waitForSync(); err != nil {
		return err
	}
	u.active = true
	return nil
}

func (u *user) waitForSync() error {
	for {
		syncing, err := u.client.CheckSyncing()
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

func (u *user) Stop() error {
	u.active = false
	if err := u.node.Stop(); err != nil {
		return err
	}
	return nil
}

func (u *user) UpdateNodeParameters(godAddress, ipfsBootNode string, ceremonyTime int64) {
	u.node.GodAddress = godAddress
	u.node.IpfsBootNode = ipfsBootNode
	u.node.CeremonyTime = ceremonyTime
}

func (u *user) DestroyNode() error {
	return u.node.Destroy()
}

func (u *user) GetAddress() string {
	return u.address
}

func (u *user) GetPubKey() string {
	return u.pubKey
}

func (u *user) SharedNode() bool {
	return false
}

func (u *user) InitAddress() error {
	address, err := u.client.GetCoinbaseAddr()
	if err != nil {
		return err
	}
	u.address = address
	return nil
}

func (u *user) InitPubKey() error {
	pubKey, err := u.client.GetPubKey()
	if err != nil {
		return err
	}
	u.pubKey = pubKey
	return nil
}

func (u *user) SetMultiBotDelegatee(multiBotDelegatee *string) {
	u.multiBotDelegatee = multiBotDelegatee
}

func (u *user) GetMultiBotDelegatee() *string {
	return u.multiBotDelegatee
}

func (u *user) GetTestContext() *TestContext {
	return u.testContext
}

func (u *user) SetTestContext(testContext *TestContext) {
	u.testContext = testContext
}

func (u *user) IsTestRun() bool {
	return u.isTestRun
}

func (u *user) SetNodeExecCommandName(command string) {
	u.node.SetExecCommandName(command)
}

func (u *user) SetIsTestRun(isTestRun bool) {
	u.isTestRun = isTestRun
}

func (u *user) GetAutoOnlineSent() bool {
	return u.sentAutoOnline
}

func (u *user) SetAutoOnlineSent(sentAutoOnline bool) {
	u.sentAutoOnline = sentAutoOnline
}

func (u *user) ActivateInvite() (string, error) {
	return u.client.ActivateInvite(api.ActivateInviteArgs{})
}

func (u *user) SubmitShortAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error) {
	return u.client.SubmitShortAnswers(answers)
}

func (u *user) SubmitOpenShortAnswers() (string, error) {
	// do nothing
	return "", nil
}

func (u *user) SubmitLongAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error) {
	return u.client.SubmitLongAnswers(answers)
}

func (u *user) SubmitFlip(privateHex, publicHex string, wordPairIdx uint8) (client.FlipSubmitResponse, error) {
	res, err := u.client.SubmitFlip(privateHex, publicHex, wordPairIdx)
	if err == nil {
		u.GetTestContext().MadeFlips++
	}
	return res, err
}

func (u *user) GetRequiredFlipsInfo() (int, []FlipWords, error) {
	identity, err := u.client.GetIdentity(u.GetAddress())
	if err != nil {
		return 0, nil, errors.Wrap(err, "unable to get identity")
	}
	u.GetTestContext().RequiredFlips = int(identity.RequiredFlips)
	flipsCount := identity.RequiredFlips
	additional := identity.AvailableFlips - identity.RequiredFlips
	if additional > 0 {
		a := []byte(u.GetAddress())
		for i := 0; i < int(additional); i++ {
			if a[len(a)-1-i]%2 == 0 {
				flipsCount++
			}
		}
	}
	flipsWords, err := loadKeyWords(identity.FlipKeyWordPairs, u.client)
	if err != nil {
		return 0, nil, err
	}
	return int(flipsCount) - len(identity.Flips), flipsWords, nil
}

func loadKeyWords(apiFlipWords []api.FlipWords, client *client.Client) ([]FlipWords, error) {
	flipsWords := make([]FlipWords, 0, len(apiFlipWords))
	for _, pair := range apiFlipWords {
		flipWord := FlipWords{}
		for i, word := range pair.Words {
			fw, err := client.KeyWord(int(word))
			if err != nil {
				return nil, errors.Wrapf(err, "unable to get keyword %v", word)
			}
			flipWord.Words[i] = FlipWord{
				Id:          word,
				Name:        fw.Name,
				Description: fw.Desc,
			}
		}
		flipsWords = append(flipsWords, flipWord)
	}
	return flipsWords, nil
}

func (u *user) SendPrivateFlipKeysPackages() error {
	// do nothing
	return nil
}

func (u *user) SendPublicFlipKey() error {
	// do nothing
	return nil
}

func (u *user) GetFlip(decryptFlips bool, hash string) (client.FlipResponse, error) {
	if !decryptFlips {
		return u.client.GetFlip(hash)
	}
	flip, err := u.client.GetFlipRaw(hash)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to get flip raw")
	}
	keys, err := u.client.GetFlipKeys(u.GetAddress(), hash)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to get flip keys")
	}
	publicEncryptionKey, err := bytesToPrivateKey(keys.PublicKey)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to convert public key")
	}
	privateEncryptionKey, err := bytesToPrivateKey(keys.PrivateKey)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to convert private key")
	}
	decryptedPublicPart, err := publicEncryptionKey.Decrypt(flip.PublicHex, nil, nil)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to decrypt flip public part")
	}
	decryptedPrivatePart, err := privateEncryptionKey.Decrypt(flip.PrivateHex, nil, nil)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to decrypt flip private part")
	}
	return client.FlipResponse{
		PublicHex:  hexutil.Bytes(decryptedPublicPart).String(),
		PrivateHex: hexutil.Bytes(decryptedPrivatePart).String(),
	}, nil
}

func bytesToPrivateKey(b []byte) (*ecies.PrivateKey, error) {
	ecdsaKey, err := crypto.ToECDSA(b)
	if err != nil {
		return nil, err
	}
	return ecies.ImportECDSA(ecdsaKey), nil
}

func (u *user) GetShortFlipHashes() ([]client.FlipHashesResponse, error) {
	return u.client.GetShortFlipHashes(u.GetAddress())
}

func (u *user) GetLongFlipHashes() ([]client.FlipHashesResponse, error) {
	return u.client.GetLongFlipHashes(u.GetAddress())
}

func (u *user) SendInvite(to string, amount float32) (client.Invite, error) {
	return u.client.SendInvite(to, amount)
}

func (u *user) SendTransaction(txType uint16, to *string, amount, maxFee float32, payload []byte) (string, error) {
	var payloadHex *string
	if len(payload) > 0 {
		h := hexutil.Encode(payload)
		payloadHex = &h
	}
	return u.client.SendTransaction(txType, u.GetAddress(), to, amount, maxFee, payloadHex)
}

func (u *user) Transaction(hash string) (api.Transaction, error) {
	return u.client.Transaction(hash)
}

func (u *user) GetIpfsAddress() (string, error) {
	return u.client.GetIpfsAddress()
}

func (u *user) GetIdentity(addr string) (api.Identity, error) {
	return u.client.GetIdentity(addr)
}

func (u *user) AddIpfsData(dataHex string, pin bool) (string, error) {
	return u.client.AddIpfsData(dataHex, pin)
}

func (u *user) StoreToIpfs(cid string) (string, error) {
	return u.client.StoreToIpfs(cid)
}

func (u *user) CeremonyIntervals() (client.CeremonyIntervals, error) {
	return u.client.CeremonyIntervals()
}

func (u *user) BecomeOnline() (string, error) {
	return u.client.BecomeOnline()
}

func (u *user) BecomeOffline() (string, error) {
	return u.client.BecomeOffline()
}

func (u *user) GetEpoch() (client.Epoch, error) {
	return u.client.GetEpoch()
}

func (u *user) GetFlipWords(hash string) (api.FlipWordsResponse, error) {
	return u.client.GetFlipWords(hash)
}
