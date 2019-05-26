package scenario

func convert(incomingSc incomingScenario) Scenario {
	sc := Scenario{}
	sc.EpochNewUsers = convertEpochNewUsers(incomingSc.Users, incomingSc.NewUsers)
	sc.EpochNodeStarts = convertEpochsNodes(incomingSc.NodeStarts)
	sc.EpochNodeStops = convertEpochsNodes(incomingSc.NodeStops)
	sc.CeremonyMinOffset = incomingSc.CeremonyMinOffset
	sc.Ceremonies = convertCeremonies(incomingSc.Ceremonies)
	return sc
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

func convertCeremonies(incomingCeremonies []ceremony) map[int]*Ceremony {
	ceremonies := make(map[int]*Ceremony)
	for _, incomingCeremony := range incomingCeremonies {
		epochs, _ := parseNums(incomingCeremony.Epochs)
		ceremony := convertCeremony(incomingCeremony)
		for _, epoch := range epochs {
			ceremonies[epoch] = ceremony
		}
	}
	return ceremonies
}

func convertCeremony(incomingCeremony ceremony) *Ceremony {
	ceremony := Ceremony{}
	ceremony.UserCeremonies = convertUserCeremonies(incomingCeremony.UserCeremonies)
	ceremony.Assertion = convertAssertion(incomingCeremony.Assertion)
	return &ceremony
}

func convertUserCeremonies(incomingUserCeremonies []userCeremony) map[int]*UserCeremony {
	userCeremonies := make(map[int]*UserCeremony)
	for _, incomingUserCeremony := range incomingUserCeremonies {
		userCeremony := convertUserCeremony(incomingUserCeremony)
		users, _ := parseNums(incomingUserCeremony.Users)
		for _, user := range users {
			userCeremonies[user] = userCeremony
		}
	}
	return userCeremonies
}

func convertUserCeremony(incomingUserCeremony userCeremony) *UserCeremony {
	userCeremony := UserCeremony{}
	userCeremony.SubmitFlips = incomingUserCeremony.SubmitFlips
	userCeremony.ShortAnswers = convertAnswers(incomingUserCeremony.ShortAnswerRates, incomingUserCeremony.ShortAnswers)
	userCeremony.LongAnswers = convertAnswers(incomingUserCeremony.LongAnswerRates, incomingUserCeremony.LongAnswers)
	return &userCeremony
}

func convertAnswers(incomingAnswerRates *answerRates, incomingAnswers []answer) AnswersHolder {
	if incomingAnswerRates != nil {
		answerRates := AnswerRates{}
		answerRates.Inappropriate = incomingAnswerRates.Inappropriate
		answerRates.Wrong = incomingAnswerRates.Wrong
		answerRates.None = incomingAnswerRates.None
		return answerRates
	}
	answers := Answers{
		Answers:   make(map[int]byte),
		Presences: make(map[int]bool),
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
