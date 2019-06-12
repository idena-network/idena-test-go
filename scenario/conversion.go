package scenario

import (
	"idena-test-go/common"
	"time"
)

const defaultDefaultAnswer = common.Left

func convert(incomingSc incomingScenario) Scenario {
	sc := Scenario{}
	sc.EpochNewUsers = convertEpochNewUsers(incomingSc.Users, incomingSc.NewUsers)
	sc.EpochNodeStarts = convertEpochsNodes(incomingSc.NodeStarts)
	sc.EpochNodeStops = convertEpochsNodes(incomingSc.NodeStops)
	sc.EpochNodeOnlines = convertEpochsNodes(incomingSc.NodeOnlines)
	sc.EpochNodeOfflines = convertEpochsNodes(incomingSc.NodeOfflines)
	sc.EpochDelayedFlipKeys = convertEpochsNodes(incomingSc.DelayedKeys)
	sc.EpochTxs = convertEpochTxs(incomingSc.Txs)
	sc.CeremonyMinOffset = incomingSc.CeremonyMinOffset
	sc.DefaultAnswer = convertDefaultAnswer(incomingSc.DefaultAnswer)
	sc.Ceremonies = convertCeremonies(incomingSc.Ceremonies, sc.DefaultAnswer)
	return sc
}

func convertDefaultAnswer(incomingDefaultAnswer byte) byte {
	if incomingDefaultAnswer <= 0 {
		return defaultDefaultAnswer
	}
	return incomingDefaultAnswer
}

func convertEpochNewUsers(incomingUsers int, incomingNewUsers []newUsers) map[int]int {
	epochUsers := make(map[int]int)
	epochUsers[0] = incomingUsers
	for _, du := range incomingNewUsers {
		epochs, _ := parseNums(du.Epochs)
		for _, epoch := range epochs {
			epochUsers[epoch] = du.Count
		}
	}
	return epochUsers
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
	userCeremony.ShortAnswers = convertAnswers(incomingUserCeremony.ShortAnswerRates, incomingUserCeremony.ShortAnswers, defaultAnswer)
	userCeremony.LongAnswers = convertAnswers(incomingUserCeremony.LongAnswerRates, incomingUserCeremony.LongAnswers, defaultAnswer)
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
