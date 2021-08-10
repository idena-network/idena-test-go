package user

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/blockchain/attachments"
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/core/flip"
	"github.com/idena-network/idena-go/core/mempool"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/crypto/ecies"
	"github.com/idena-network/idena-go/crypto/sha3"
	"github.com/idena-network/idena-go/crypto/vrf"
	"github.com/idena-network/idena-go/crypto/vrf/p256"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"sync"
)

type sharedNodeUser struct {
	client     *client.Client
	user       User
	parentUser User
	privateKey *ecdsa.PrivateKey
	address    string
	pubKey     string
	mutex      sync.Mutex
}

func (u *sharedNodeUser) GetIndex() int {
	return u.user.GetIndex()
}

func (u *sharedNodeUser) GetInfo() string {
	return fmt.Sprintf("[User %d-shared-%d]", u.GetIndex(), u.parentUser.GetIndex())
}

func (u *sharedNodeUser) IsActive() bool {
	return true
}

func (u *sharedNodeUser) GetAddress() string {
	return u.address
}

func (u *sharedNodeUser) GetPubKey() string {
	return u.pubKey
}

func (u *sharedNodeUser) SharedNode() bool {
	return true
}

func (u *sharedNodeUser) GetTestContext() *TestContext {
	return u.user.GetTestContext()
}

func (u *sharedNodeUser) SetTestContext(testContext *TestContext) {
	u.user.SetTestContext(testContext)
}

func (u *sharedNodeUser) IsTestRun() bool {
	return u.user.IsTestRun()
}

func (u *sharedNodeUser) SetIsTestRun(isTestRun bool) {
	u.user.SetIsTestRun(isTestRun)
}

func (u *sharedNodeUser) SetMultiBotDelegatee(multiBotDelegatee *string) {
	u.user.SetMultiBotDelegatee(multiBotDelegatee)
}

func (u *sharedNodeUser) GetMultiBotDelegatee() *string {
	return u.user.GetMultiBotDelegatee()
}

func (u *sharedNodeUser) GetAutoOnlineSent() bool {
	return u.user.GetAutoOnlineSent()
}

func (u *sharedNodeUser) SetAutoOnlineSent(sentAutoOnline bool) {
	u.user.SetAutoOnlineSent(sentAutoOnline)
}

func (u *sharedNodeUser) SetNodeExecCommandName(command string) {
	// do nothing
}

func (u *sharedNodeUser) Start(mode node.StartMode) error {
	// do nothing
	return nil
}

func (u *sharedNodeUser) Stop() error {
	// do nothing
	return nil
}

func (u *sharedNodeUser) UpdateNodeParameters(godAddress, ipfsBootNode string, ceremonyTime int64) {
	// do nothing
}

func (u *sharedNodeUser) DestroyNode() error {
	// do nothing
	return nil
}

func (u *sharedNodeUser) InitAddress() error {
	if len(u.address) > 0 {
		return nil
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		return err
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	u.privateKey = key
	u.address = addr.Hex()
	u.pubKey = hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))
	return nil
}

func (u *sharedNodeUser) InitPubKey() error {
	return u.InitAddress()
}

func (u *sharedNodeUser) ActivateInvite() (string, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	privateKey := hex.EncodeToString(crypto.FromECDSA(u.privateKey))
	pubKey := hexutil.Bytes(crypto.FromECDSAPub(&u.privateKey.PublicKey))
	params := api.ActivateInviteArgs{
		PubKey: &pubKey,
		Key:    privateKey,
	}
	return u.client.ActivateInvite(params)
}

func (u *sharedNodeUser) SubmitShortAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	apiAnswers := make([]api.FlipAnswer, 0, len(answers))
	for _, answer := range answers {
		apiAnswers = append(apiAnswers, api.FlipAnswer{
			Grade:  types.Grade(answer.Grade),
			Answer: types.Answer(answer.Answer),
			Hash:   answer.Hash,
		})
	}
	flips := make([][]byte, 0, len(u.GetTestContext().ShortFlipHashes))
	for _, flipHash := range u.GetTestContext().ShortFlipHashes {
		flipCid, _ := cid.Parse(flipHash.Hash)
		flips = append(flips, flipCid.Bytes())
	}
	preparedAnswers := prepareAnswers(apiAnswers, flips, true)
	epoch := u.GetTestContext().Epoch
	salt := u.getShortAnswersSalt(epoch)
	var hash [32]byte
	preparedAnswersBytes := preparedAnswers.Bytes()
	u.GetTestContext().ShortAnswersBytes = preparedAnswersBytes
	hash = crypto.Hash(append(preparedAnswersBytes, salt[:]...))
	payload := hash[:]
	txHash, err := u.sendTransaction(types.SubmitAnswersHashTx, nil, 0, 0, payload)
	if err != nil {
		return client.SubmitAnswersResponse{}, err
	}
	return client.SubmitAnswersResponse{
		TxHash: txHash,
	}, nil
}

func (u *sharedNodeUser) getShortAnswersSalt(epoch uint16) []byte {
	seed := []byte(fmt.Sprintf("short-answers-salt-%v", epoch))
	hash := common.Hash(crypto.Hash(seed))
	sig := u.sign(hash.Bytes())
	sha := sha3.Sum256(sig)
	return sha[:]
}

func (u *sharedNodeUser) sign(data []byte) []byte {
	sec := u.privateKey
	sig, _ := crypto.Sign(data, sec)
	return sig
}

func prepareAnswers(answers []api.FlipAnswer, flips [][]byte, isShort bool) *types.Answers {
	findAnswer := func(hash []byte) *api.FlipAnswer {
		for _, h := range answers {
			c, err := cid.Parse(h.Hash)
			if err == nil && bytes.Compare(c.Bytes(), hash) == 0 {
				return &h
			}
		}
		return nil
	}

	result := types.NewAnswers(uint(len(flips)))
	reportsCount := 0

	for i, f := range flips {
		answer := findAnswer(f)
		if answer == nil {
			continue
		}
		switch answer.Answer {
		case types.None:
			continue
		case types.Left:
			result.Left(uint(i))
		case types.Right:
			result.Right(uint(i))
		}
		if isShort {
			continue
		}
		grade := answer.Grade
		if grade == types.GradeReported {
			reportsCount++
			if float32(reportsCount)/float32(len(flips)) >= 0.34 {
				grade = types.GradeNone
			}
		}
		result.Grade(uint(i), grade)
	}

	return result
}

func (u *sharedNodeUser) SubmitOpenShortAnswers() (string, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	answers := u.GetTestContext().ShortAnswersBytes
	wordsSeed, err := u.client.WordsSeed()
	if err != nil {
		return "", errors.Wrap(err, "unable to get words seed")
	}
	_, proof := u.vrfEvaluate(wordsSeed)
	u.GetTestContext().WordsProof = proof
	h, err := vrf.HashFromProof(proof)
	if err != nil {
		return "", errors.Wrap(err, "cannot get hash from proof")
	}
	payload := attachments.CreateShortAnswerAttachment(answers, getWordsRnd(h))
	txHash, err := u.sendTransaction(types.SubmitShortAnswersTx, nil, 0, 0, payload)
	if err != nil {
		return "", err
	}
	return txHash, nil
}

func getWordsRnd(hash [32]byte) uint64 {
	return binary.LittleEndian.Uint64(hash[:])
}

func (u *sharedNodeUser) vrfEvaluate(data []byte) (index [32]byte, proof []byte) {
	sec := u.privateKey
	signer, _ := p256.NewVRFSigner(sec)
	return signer.Evaluate(data)
}

func (u *sharedNodeUser) SubmitLongAnswers(answers []client.FlipAnswer) (client.SubmitAnswersResponse, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	apiAnswers := make([]api.FlipAnswer, 0, len(answers))
	for _, answer := range answers {
		apiAnswers = append(apiAnswers, api.FlipAnswer{
			Grade:  types.Grade(answer.Grade),
			Answer: types.Answer(answer.Answer),
			Hash:   answer.Hash,
		})
	}
	flips := make([][]byte, 0, len(u.GetTestContext().LongFlipHashes))
	for _, flipHash := range u.GetTestContext().LongFlipHashes {
		flipCid, _ := cid.Parse(flipHash.Hash)
		flips = append(flips, flipCid.Bytes())
	}
	preparedAnswers := prepareAnswers(apiAnswers, flips, true)
	preparedAnswersBytes := preparedAnswers.Bytes()

	epoch := u.GetTestContext().Epoch
	salt := u.getShortAnswersSalt(epoch)

	key := u.generateFlipEncryptionKey(true)

	payload := attachments.CreateLongAnswerAttachment(preparedAnswersBytes, u.GetTestContext().WordsProof, salt, key)
	txHash, err := u.sendTransaction(types.SubmitLongAnswersTx, nil, 0, 0, payload)
	if err != nil {
		return client.SubmitAnswersResponse{}, err
	}
	return client.SubmitAnswersResponse{
		TxHash: txHash,
	}, nil
}

func (u *sharedNodeUser) generateFlipEncryptionKey(public bool) *ecies.PrivateKey {
	if public && u.GetTestContext().PublicEncryptionKey != nil {
		return u.GetTestContext().PublicEncryptionKey
	}
	if !public && u.GetTestContext().PrivateEncryptionKey != nil {
		return u.GetTestContext().PrivateEncryptionKey
	}
	var seed []byte
	if public {
		seed = []byte(fmt.Sprintf("flip-key-for-epoch-%v", u.GetTestContext().Epoch))
	} else {
		seed = []byte(fmt.Sprintf("flip-private-key-for-epoch-%v", u.GetTestContext().Epoch))
	}
	hash := common.Hash(crypto.Hash(seed))
	sig := u.sign(hash.Bytes())
	flipKey, _ := crypto.GenerateKeyFromSeed(bytes.NewReader(sig))
	res := ecies.ImportECDSA(flipKey)
	if public {
		u.GetTestContext().PublicEncryptionKey = res
	} else {
		u.GetTestContext().PrivateEncryptionKey = res
	}
	return res
}

func (u *sharedNodeUser) SendTransaction(txType uint16, to *string, amount, maxFee float32, payload []byte) (string, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	return u.sendTransaction(txType, to, amount, maxFee, payload)
}

func (u *sharedNodeUser) sendTransaction(txType uint16, to *string, amount, maxFee float32, payload []byte) (string, error) {
	rawSignedTx, err := u.buildSignedTransactionRaw(txType, to, amount, maxFee, payload)
	if err != nil {
		return "", errors.Wrap(err, "unable to serialize signed tx")
	}
	return u.client.SendRawTx(rawSignedTx)
}

func (u *sharedNodeUser) buildSignedTransactionRaw(txType uint16, to *string, amount, maxFee float32, payload []byte) ([]byte, error) {
	rawTx, err := u.client.GetRawTx(txType, u.GetAddress(), to, amount, maxFee, payload)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get raw tx")
	}
	bytesTx, err := hexutil.Decode(rawTx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to decode raw tx")
	}
	var tx types.Transaction
	if err := tx.FromBytes(bytesTx); err != nil {
		return nil, errors.Wrap(err, "unable to deserialize raw tx")
	}
	signedTx, err := types.SignTx(&tx, u.privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to sign tx")
	}
	rawSignedTx, err := signedTx.ToBytes()
	if err != nil {
		return nil, errors.Wrap(err, "unable to serialize signed tx")
	}
	return rawSignedTx, nil
}

func (u *sharedNodeUser) SubmitFlip(privateHex, publicHex string, wordPairIdx uint8) (client.FlipSubmitResponse, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	publicEncryptionKey, privateEncryptionKey := u.generateFlipEncryptionKey(true), u.generateFlipEncryptionKey(false)
	flipPublicPart, err := hexutil.Decode(publicHex)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to decode public part")
	}
	encryptedPublic, err := ecies.Encrypt(rand.Reader, &publicEncryptionKey.PublicKey, flipPublicPart, nil, nil)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to encrypt public part")
	}
	flipPrivatePart, err := hexutil.Decode(privateHex)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to decode private part")
	}
	encryptedPrivate, err := ecies.Encrypt(rand.Reader, &privateEncryptionKey.PublicKey, flipPrivatePart, nil, nil)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to encrypt private part")
	}
	encryptedPublicHex := hexutil.Bytes(encryptedPublic)
	encryptedPrivateHex := hexutil.Bytes(encryptedPrivate)

	ipfsFlip := &flip.IpfsFlip{
		PublicPart:  encryptedPublic,
		PrivatePart: encryptedPrivate,
		PubKey:      crypto.FromECDSAPub(&u.privateKey.PublicKey),
	}
	ipfsFlipData, _ := ipfsFlip.ToBytes()
	flipCidStr, err := u.client.IpfsDataCid(hexutil.Encode(ipfsFlipData))
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to get flip cid")
	}
	flipCid, err := cid.Parse(flipCidStr)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to parse flip cid")
	}
	attachment := attachments.CreateFlipSubmitAttachment(flipCid.Bytes(), wordPairIdx)
	payload := attachment
	txRaw, err := u.buildSignedTransactionRaw(types.SubmitFlipTx, nil, 0, 0, payload)
	if err != nil {
		return client.FlipSubmitResponse{}, errors.Wrap(err, "unable to build flip tx")
	}
	txRawHex := hexutil.Bytes(txRaw)
	res, err := u.client.SubmitRawFlip(&txRawHex, &encryptedPublicHex, &encryptedPrivateHex)
	if err == nil {
		u.GetTestContext().MadeFlips++
	}
	if len(res.Hash) == 0 {
		res.Hash = flipCidStr
	}
	return res, err
}

func (u *sharedNodeUser) GetRequiredFlipsInfo() (int, []api.FlipWords, error) {
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
	wordsSeed, err := u.client.WordsSeed()
	if err != nil {
		return 0, nil, errors.Wrap(err, "unable to get words seed")
	}
	hash, _ := u.vrfEvaluate(wordsSeed)
	flipKeyWordPairs, err := u.client.WordPairs(u.GetAddress(), hash[:])
	return int(flipsCount) - len(identity.Flips), flipKeyWordPairs, nil
}

func (u *sharedNodeUser) SendPrivateFlipKeysPackages() error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if u.GetTestContext().MadeFlips == 0 {
		return nil
	}
	pubKeyHexes, err := u.client.PrivateEncryptionKeyCandidates(common.HexToAddress(u.GetAddress()))
	if err != nil {
		return errors.Wrap(err, "unable to get candidates")
	}
	pubKeys := make([][]byte, 0, len(pubKeyHexes))
	for _, pubKeyHex := range pubKeyHexes {
		pubKeys = append(pubKeys, pubKeyHex)
	}
	publicFlipKey := u.generateFlipEncryptionKey(true)
	privateFlipKey := u.generateFlipEncryptionKey(false)
	data := mempool.EncryptPrivateKeysPackage(publicFlipKey, privateFlipKey, pubKeys)

	msg := types.PrivateFlipKeysPackage{
		Data:  data,
		Epoch: u.GetTestContext().Epoch,
	}
	signedMsg, err := types.SignFlipKeysPackage(&msg, u.privateKey)
	if err != nil {
		return errors.Wrap(err, "unable to sign")
	}

	args := api.EncryptionKeyArgs{
		Epoch:     signedMsg.Epoch,
		Data:      signedMsg.Data,
		Signature: signedMsg.Signature,
	}
	err = u.client.SendPrivateEncryptionKeysPackage(args)
	if err == nil {
		log.Info(fmt.Sprintf("%v sent private flip keys package", u.GetInfo()))
	}
	return err
}

func (u *sharedNodeUser) SendPublicFlipKey() error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	if u.GetTestContext().MadeFlips == 0 {
		return nil
	}
	key := u.generateFlipEncryptionKey(true)
	msg := types.PublicFlipKey{
		Epoch: u.GetTestContext().Epoch,
		Key:   crypto.FromECDSA(key.ExportECDSA()),
	}
	signedMsg, err := types.SignFlipKey(&msg, u.privateKey)
	if err != nil {
		return errors.Wrap(err, "unable to sign")
	}
	args := api.EncryptionKeyArgs{
		Epoch:     signedMsg.Epoch,
		Data:      signedMsg.Key,
		Signature: signedMsg.Signature,
	}
	err = u.client.SendPublicEncryptionKey(args)
	if err == nil {
		log.Info(fmt.Sprintf("%v sent public flip key", u.GetInfo()))
	}
	return err
}

func (u *sharedNodeUser) GetFlip(decryptFlips bool, hash string) (client.FlipResponse, error) {
	if u.GetTestContext().FlipsByHash != nil {
		if res, ok := u.GetTestContext().FlipsByHash[hash]; ok {
			return res, nil
		}
	}
	f, err := u.client.GetFlipRaw(hash)
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
	decryptedPrivateKey, err := u.decryptMessage(keys.PrivateKey)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to decrypt private key")
	}
	privateEncryptionKey, err := bytesToPrivateKey(decryptedPrivateKey)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to convert private key")
	}
	decryptedPublicPart, err := publicEncryptionKey.Decrypt(f.PublicHex, nil, nil)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to decrypt flip public part")
	}
	decryptedPrivatePart, err := privateEncryptionKey.Decrypt(f.PrivateHex, nil, nil)
	if err != nil {
		return client.FlipResponse{}, errors.Wrap(err, "unable to decrypt flip private part")
	}
	if u.GetTestContext().FlipsByHash == nil {
		u.GetTestContext().FlipsByHash = make(map[string]client.FlipResponse)
	}
	res := client.FlipResponse{
		PublicHex:  hexutil.Bytes(decryptedPublicPart).String(),
		PrivateHex: hexutil.Bytes(decryptedPrivatePart).String(),
	}
	u.GetTestContext().FlipsByHash[hash] = res
	return res, nil
}

func (u *sharedNodeUser) decryptMessage(data []byte) ([]byte, error) {
	sec := u.privateKey
	return ecies.ImportECDSA(sec).Decrypt(data, nil, nil)
}

func (u *sharedNodeUser) GetShortFlipHashes() ([]client.FlipHashesResponse, error) {
	res, err := u.client.GetShortFlipHashes(u.GetAddress())
	if err != nil {
		return nil, err
	}
	for i, item := range res {
		_, err := u.GetFlip(true, item.Hash)
		res[i].Ready = err == nil
	}
	return res, nil
}

func (u *sharedNodeUser) GetLongFlipHashes() ([]client.FlipHashesResponse, error) {
	res, err := u.client.GetLongFlipHashes(u.GetAddress())
	if err != nil {
		return nil, err
	}
	for i, item := range res {
		_, err := u.GetFlip(true, item.Hash)
		res[i].Ready = err == nil
	}
	return res, nil
}

func (u *sharedNodeUser) SendInvite(to string, amount float32) (client.Invite, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	txHash, err := u.sendTransaction(types.InviteTx, &to, amount, 0, nil)
	if err != nil {
		return client.Invite{}, err
	}
	return client.Invite{
		Hash: txHash,
	}, nil
}

func (u *sharedNodeUser) Transaction(hash string) (api.Transaction, error) {
	return u.parentUser.Transaction(hash)
}

func (u *sharedNodeUser) GetIpfsAddress() (string, error) {
	return "", nil
}

func (u *sharedNodeUser) GetIdentity(addr string) (api.Identity, error) {
	return u.parentUser.GetIdentity(addr)
}

func (u *sharedNodeUser) AddIpfsData(dataHex string, pin bool) (string, error) {
	return u.parentUser.AddIpfsData(dataHex, pin)
}

func (u *sharedNodeUser) StoreToIpfs(cid string) (string, error) {
	return u.parentUser.StoreToIpfs(cid)
}

func (u *sharedNodeUser) CeremonyIntervals() (client.CeremonyIntervals, error) {
	return u.parentUser.CeremonyIntervals()
}

func (u *sharedNodeUser) BecomeOnline() (string, error) {
	to := u.parentUser.GetAddress()
	return u.sendTransaction(types.DelegateTx, &to, 0, 0, nil)
}

func (u *sharedNodeUser) BecomeOffline() (string, error) {
	return u.sendTransaction(types.UndelegateTx, nil, 0, 0, nil)
}

func (u *sharedNodeUser) GetEpoch() (client.Epoch, error) {
	return u.parentUser.GetEpoch()
}

func (u *sharedNodeUser) GetFlipWords(hash string) (api.FlipWordsResponse, error) {
	return u.parentUser.GetFlipWords(hash)
}
