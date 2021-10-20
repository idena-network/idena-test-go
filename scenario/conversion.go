package scenario

import (
	"github.com/idena-network/idena-test-go/common"
	"sort"
	"time"
)

const (
	defaultDefaultAnswer = common.Left
	defaultInviteAmount  = 100
)

func convert(incomingSc incomingScenario, godMode bool, nodes int) Scenario {
	if nodes > 0 {
		incomingSc.Users = nodes
	}
	sc := Scenario{}
	sc.InviteAmount = convertInviteAmount(incomingSc.InviteAmount)
	sc.EpochNewUsersBeforeFlips = convertEpochNewUsers(incomingSc.Users, incomingSc.NewUsers)
	sc.EpochNewUsersAfterFlips = convertEpochNewUsers(0, incomingSc.NewUsersAfterFlips)
	sc.EpochNodeSwitches = convertNodeSwitches(incomingSc.NodeStarts, incomingSc.NodeStops)
	sc.EpochNodeOnlines = convertEpochsNodes(incomingSc.NodeOnlines)
	sc.EpochNodeOfflines = convertEpochsNodes(incomingSc.NodeOfflines)
	sc.EpochDelayedFlipKeys = convertEpochsNodes(incomingSc.DelayedKeys)
	sc.EpochTxs = convertEpochTxs(incomingSc.Txs)
	sc.CeremonyMinOffset = incomingSc.CeremonyMinOffset
	sc.DefaultAnswer = convertDefaultAnswer(incomingSc.DefaultAnswer)
	sc.Ceremonies = convertCeremonies(incomingSc.Ceremonies, sc.DefaultAnswer)
	sc.EpochNodeUpdates = convertNodeUpdates(incomingSc.NodeUpdates)
	sc.Delegations = convertDelegations(incomingSc.Delegations)
	sc.Undelegations = convertEpochsNodes(incomingSc.Undelegations)
	sc.MultiBotPools = convertMultiBotPools(incomingSc.MultiBotPools)
	sc.KillDelegators = convertKillDelegators(incomingSc.KillDelegators)
	sc.StoreToIpfsTxs = convertStoreToIpfsTxs(incomingSc.StoreToIpfsTxs, godMode)
	sc.Kills = convertEpochsNodes(incomingSc.Kills)
	return sc
}

func convertInviteAmount(incomingInviteAmount *float32) float32 {
	if incomingInviteAmount == nil {
		return defaultInviteAmount
	}
	return *incomingInviteAmount
}

func convertDefaultAnswer(incomingDefaultAnswer byte) byte {
	if incomingDefaultAnswer <= 0 {
		return defaultDefaultAnswer
	}
	return incomingDefaultAnswer
}

func convertEpochNewUsers(incomingUsers int, incomingNewUsers []newUsers) map[int][]*NewUsers {
	newUsersByEpoch := make(map[int][]*NewUsers)
	if incomingUsers > 0 {
		inviter := 0
		newUsersByEpoch[0] = []*NewUsers{
			{
				Inviter: &inviter,
				Count:   incomingUsers,
			},
		}
	}
	for _, du := range incomingNewUsers {
		epochs, _ := parseNums(du.Epochs)
		for _, epoch := range epochs {
			newUsersByEpoch[epoch] = append(newUsersByEpoch[epoch], &NewUsers{
				Inviter:      du.Inviter,
				Command:      du.Command,
				Count:        du.Count,
				SharedNode:   du.SharedNode,
				InviteAmount: du.InviteAmount,
			})
		}
	}
	return newUsersByEpoch
}

func convertEpochsNodes(incomingEpochsNodes []epochsNodes) map[int][]int {
	epochsNodes := make(map[int][]int)
	for _, en := range incomingEpochsNodes {
		epochs, _ := parseNums(en.Epochs)
		nodes, _ := parseNums(en.Nodes)
		for _, epoch := range epochs {
			for _, node := range nodes {
				epochsNodes[epoch] = append(epochsNodes[epoch], node)
			}
		}
	}
	return epochsNodes
}

func convertNodeSwitches(starts []delayedEpochsNodes, stops []delayedEpochsNodes) map[int]map[int][]*NodeSwitch {
	switchesPerEpochAndNode := make(map[int]map[int][]*NodeSwitch)
	type data struct {
		isStart  bool
		switches []delayedEpochsNodes
	}
	dataList := []data{
		{
			true, starts,
		},
		{
			false, stops,
		},
	}
	for _, ctx := range dataList {
		for _, switchItem := range ctx.switches {
			epochs, _ := parseNums(switchItem.Epochs)
			nodes, _ := parseNums(switchItem.Nodes)
			for _, epoch := range epochs {
				for _, node := range nodes {
					epochSwitches, present := switchesPerEpochAndNode[epoch]
					if !present {
						switchesPerEpochAndNode[epoch] = make(map[int][]*NodeSwitch)
					}
					switchesPerEpochAndNode[epoch][node] = append(switchesPerEpochAndNode[epoch][node],
						&NodeSwitch{
							IsStart: ctx.isStart,
							Delay:   time.Second * time.Duration(switchItem.DelaySec),
						})

					sort.SliceStable(epochSwitches[node], func(i, j int) bool {
						return epochSwitches[node][i].Delay < epochSwitches[node][j].Delay
					})
				}
			}
		}
	}
	return switchesPerEpochAndNode
}

func convertEpochTxs(incomingTxs []transactions) map[int]*Txs {
	epochTxs := make(map[int]*Txs)
	for _, incomingTxsItem := range incomingTxs {
		epochs, _ := parseNums(incomingTxsItem.Epochs)
		nodes, _ := parseNums(incomingTxsItem.Nodes)
		for _, epoch := range epochs {
			for _, node := range nodes {
				var epochTxsItem *Txs
				var present bool
				if epochTxsItem, present = epochTxs[epoch]; !present {
					epochTxsItem = &Txs{
						Period: time.Millisecond * time.Duration(incomingTxsItem.PeriodMs),
					}
					epochTxs[epoch] = epochTxsItem
				}
				epochTxsItem.Users = append(epochTxsItem.Users, node)
			}
		}
	}
	return epochTxs
}

func convertCeremonies(incomingCeremonies []ceremony, defaultAnswer byte) map[int]*Ceremony {
	ceremonies := make(map[int]*Ceremony)
	for _, incomingCeremony := range incomingCeremonies {
		epochs, _ := parseNums(incomingCeremony.Epochs)
		ceremony := convertCeremony(incomingCeremony, defaultAnswer)
		for _, epoch := range epochs {
			ceremonies[epoch] = ceremony
		}
	}
	return ceremonies
}

func convertCeremony(incomingCeremony ceremony, defaultAnswer byte) *Ceremony {
	ceremony := Ceremony{}
	ceremony.UserCeremonies = convertUserCeremonies(incomingCeremony.UserCeremonies, defaultAnswer)
	ceremony.Assertion = convertAssertion(incomingCeremony.Assertion)
	return &ceremony
}

func convertUserCeremonies(incomingUserCeremonies []userCeremony, defaultAnswer byte) map[int]*UserCeremony {
	userCeremonies := make(map[int]*UserCeremony)
	for _, incomingUserCeremony := range incomingUserCeremonies {
		userCeremony := convertUserCeremony(incomingUserCeremony, defaultAnswer)
		users, _ := parseNums(incomingUserCeremony.Users)
		for _, user := range users {
			userCeremonies[user] = userCeremony
		}
	}
	return userCeremonies
}

func convertUserCeremony(incomingUserCeremony userCeremony, defaultAnswer byte) *UserCeremony {
	userCeremony := UserCeremony{}
	userCeremony.SubmitFlips = incomingUserCeremony.SubmitFlips
	//userCeremony.ShortAnswers = convertAnswers(incomingUserCeremony.ShortAnswerRates, incomingUserCeremony.ShortAnswers, defaultAnswer)
	//userCeremony.LongAnswers = convertAnswers(incomingUserCeremony.LongAnswerRates, incomingUserCeremony.LongAnswers, defaultAnswer)
	userCeremony.SkipValidation = incomingUserCeremony.SkipValidation
	return &userCeremony
}

func convertAnswers(incomingAnswerRates *answerRates, incomingAnswers []answer, defaultAnswer byte) AnswersHolder {
	if incomingAnswerRates != nil {
		answerRates := AnswerRates{}
		answerRates.Inappropriate = incomingAnswerRates.Inappropriate
		answerRates.Left = incomingAnswerRates.Left
		answerRates.Right = incomingAnswerRates.Right

		if answerRates.Inappropriate == 0 && answerRates.Left == 0 && answerRates.Right == 0 {
			switch defaultAnswer {
			case common.Left:
				answerRates.Left = 1.0
				break
			case common.Right:
				answerRates.Right = 1.0
				break
			case common.Inappropriate:
				answerRates.Inappropriate = 1.0
				break
			}
		}

		return answerRates
	}
	answers := Answers{
		defaultAnswer: defaultAnswer,
		Answers:       make(map[int]byte),
		Presences:     make(map[int]bool),
	}
	for _, incomingAnswer := range incomingAnswers {
		answers.Answers[incomingAnswer.Index] = incomingAnswer.Answer
		answers.Presences[incomingAnswer.Index] = true
	}
	return answers
}

func convertAssertion(incomingAssertion *assertion) *Assertion {
	if incomingAssertion == nil {
		return nil
	}
	assertion := Assertion{}
	assertion.Nodes = convertNodeAssertions(incomingAssertion.Nodes)
	assertion.States = convertStateAssertions(incomingAssertion.States)
	return &assertion
}

func convertNodeAssertions(incomingNodeAssertions []nodeAssertion) map[int]*NodeAssertion {
	nodeAssertions := make(map[int]*NodeAssertion)
	for _, incomingNodeAssertion := range incomingNodeAssertions {
		nodeAssertion := convertNodeAssertion(incomingNodeAssertion)
		users, _ := parseNums(incomingNodeAssertion.Users)
		for _, user := range users {
			nodeAssertions[user] = nodeAssertion
		}
	}
	return nodeAssertions
}

func convertNodeAssertion(incomingNodeAssertion nodeAssertion) *NodeAssertion {
	nodeAssertion := NodeAssertion{}
	nodeAssertion.MadeFlips = incomingNodeAssertion.MadeFlips
	nodeAssertion.RequiredFlips = incomingNodeAssertion.RequiredFlips
	nodeAssertion.AvailableInvites = incomingNodeAssertion.AvailableInvites
	nodeAssertion.State = incomingNodeAssertion.State
	nodeAssertion.Online = incomingNodeAssertion.Online
	return &nodeAssertion
}

func convertStateAssertions(incomingStateAssertions []stateAssertion) []StateAssertion {
	var stateAssertions []StateAssertion
	for _, incomingStateAssertion := range incomingStateAssertions {
		stateAssertions = append(stateAssertions, convertStateAssertion(incomingStateAssertion))
	}
	return stateAssertions
}

func convertStateAssertion(incomingStateAssertion stateAssertion) StateAssertion {
	stateAssertion := StateAssertion{}
	stateAssertion.State = incomingStateAssertion.State
	stateAssertion.Count = incomingStateAssertion.Count
	return stateAssertion
}

func convertNodeUpdates(updates []nodeUpdates) map[int]map[int]*NodeUpdate {
	result := make(map[int]map[int]*NodeUpdate)
	for _, update := range updates {
		epochs, _ := parseNums(update.Epochs)
		nodes, _ := parseNums(update.Nodes)
		for _, epoch := range epochs {
			delay := time.Second * time.Duration(update.DelaySec)
			for _, node := range nodes {
				if _, present := result[epoch]; !present {
					result[epoch] = make(map[int]*NodeUpdate)
				}
				delay += time.Second * time.Duration(update.StepDelaySec)
				result[epoch][node] = &NodeUpdate{
					Delay:   delay,
					Command: update.Command,
				}
			}
		}
	}
	return result
}

func convertDelegations(incomingDelegations []delegations) map[int]map[int]int {
	result := make(map[int]map[int]int)
	for _, incomingDelegation := range incomingDelegations {
		epochs, err := parseNums(incomingDelegation.Epochs)
		if err != nil {
			panic(err)
		}
		nodes, err := parseNums(incomingDelegation.Nodes)
		if err != nil {
			panic(err)
		}
		for _, epoch := range epochs {
			for _, node := range nodes {
				if _, present := result[epoch]; !present {
					result[epoch] = make(map[int]int)
				}
				result[epoch][node] = incomingDelegation.Delegatee
			}
		}
	}
	return result
}

func convertMultiBotPools(incomingMultiBotPools *multiBotPools) *MultiBotPools {
	if incomingMultiBotPools == nil {
		return nil
	}
	return &MultiBotPools{
		Sizes:             incomingMultiBotPools.Sizes,
		BotDelegatorsRate: incomingMultiBotPools.BotDelegatorsRate,
	}
}

func convertKillDelegators(incomingKillDelegators []killDelegators) map[int]map[int][]int {
	result := make(map[int]map[int][]int)
	for _, incomingKillDelegatorsItem := range incomingKillDelegators {
		epochs, err := parseNums(incomingKillDelegatorsItem.Epochs)
		if err != nil {
			panic(err)
		}
		nodes, err := parseNums(incomingKillDelegatorsItem.Nodes)
		if err != nil {
			panic(err)
		}
		for _, epoch := range epochs {
			if _, present := result[epoch]; !present {
				result[epoch] = make(map[int][]int)
			}
			result[epoch][incomingKillDelegatorsItem.Delegatee] = append(result[epoch][incomingKillDelegatorsItem.Delegatee], nodes...)
		}
	}
	return result
}

func convertStoreToIpfsTxs(storeToIpfsTxs []storeToIpfsTxs, godMode bool) map[int]map[int]*StoreToIpfsTxs {
	result := make(map[int]map[int]*StoreToIpfsTxs)
	for _, item := range storeToIpfsTxs {
		epochs, _ := parseNums(item.Epochs)
		nodes, _ := parseNums(item.Nodes)
		if item.OnlyGodMode && !godMode {
			continue
		}
		for _, epoch := range epochs {
			delay := time.Second * time.Duration(item.DelaySec)
			stepDelay := time.Second * time.Duration(item.StepDelaySec)
			for _, node := range nodes {
				if _, present := result[epoch]; !present {
					result[epoch] = make(map[int]*StoreToIpfsTxs)
				}
				result[epoch][node] = &StoreToIpfsTxs{
					Delay:     delay,
					StepDelay: stepDelay,
					Sizes:     item.Sizes,
				}
			}
		}
	}
	return result
}
