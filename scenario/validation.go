package scenario

import (
	"errors"
	"fmt"
)

func (sc *incomingScenario) validate() error {
	if err := validatePositiveInt(int64(sc.Users), "UsersCount"); err != nil {
		return err
	}
	if err := validatePositiveInt(int64(sc.CeremonyMinOffset), "CeremonyMinOffset"); err != nil {
		return err
	}
	if err := validateAllNewUsers(sc.NewUsers); err != nil {
		return err
	}
	if err := validateAllNewUsers(sc.NewUsersAfterFlips); err != nil {
		return err
	}
	if err := validateDelayedEpochsNodes(sc.NodeStarts); err != nil {
		return err
	}
	if err := validateDelayedEpochsNodes(sc.NodeStops); err != nil {
		return err
	}
	if err := validateEpochsNodes(sc.NodeOnlines); err != nil {
		return err
	}
	if err := validateEpochsNodes(sc.NodeOfflines); err != nil {
		return err
	}
	if err := validateEpochsNodes(sc.DelayedKeys); err != nil {
		return err
	}
	if err := validateTransactions(sc.Txs); err != nil {
		return err
	}
	epochUsersCount := buildEpochUserCounts(sc)
	if err := validateCeremonies(sc.Ceremonies, epochUsersCount); err != nil {
		return err
	}
	return nil
}

func validatePositiveInt(value int64, name string) error {
	if value <= 0 {
		return errors.New(fmt.Sprintf("Value %s must be positive, actual value %d", name, value))
	}
	return nil
}

func validateNotNegativeInt(value int, name string) error {
	if value < 0 {
		return errors.New(fmt.Sprintf("Value %s must not be negative, actual value %d", name, value))
	}
	return nil
}

func validateNotNegativeFloat(value float32, name string) error {
	if value < 0 {
		return errors.New(fmt.Sprintf("Value %s mustn't be negative, actual value %f", name, value))
	}
	return nil
}

func validateAllNewUsers(allNewUsers []newUsers) error {
	usedEpochInviters := make(map[string]bool)
	for _, du := range allNewUsers {
		if err := validateNewUsers(du, usedEpochInviters); err != nil {
			return err
		}
	}
	return nil
}

func validateNewUsers(du newUsers, usedEpochInviters map[string]bool) error {
	if err := validateEpochNodes(du.Epochs, du.Inviter, usedEpochInviters); err != nil {
		return err
	}
	if err := validatePositiveInt(int64(du.Count), "count"); err != nil {
		return err
	}
	if du.Inviter != nil {
		if err := validateNotNegativeInt(*du.Inviter, "inviter"); err != nil {
			return err
		}
	}
	return nil
}

func validateEpochs(epochsStr string, usedEpochs map[int]bool) error {
	epochs, err := parseNums(epochsStr)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to parse epochs str %s: %s", epochsStr, err.Error()))
	}
	for _, epoch := range epochs {
		if err := validateEpoch(epoch, usedEpochs); err != nil {
			return err
		}
	}
	return nil
}

func validateEpochNodes(epochsStr string, node *int, usedEpochNodes map[string]bool) error {
	epochs, err := parseNums(epochsStr)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to parse epochs str %s: %s", epochsStr, err.Error()))
	}
	for _, epoch := range epochs {
		if err := validateEpochNode(epoch, node, usedEpochNodes); err != nil {
			return err
		}
	}
	return nil
}

func validateEpochsNodes(en []epochsNodes) error {
	for _, enItem := range en {
		if err := validateEpochsNodesItem(enItem); err != nil {
			return err
		}
	}
	return nil
}

func validateDelayedEpochsNodes(den []delayedEpochsNodes) error {
	for _, denItem := range den {
		if err := validateDelayedEpochsNodesItem(denItem); err != nil {
			return err
		}
	}
	return nil
}

func validateDelayedEpochsNodesItem(den delayedEpochsNodes) error {
	if err := validateEpochsNodesItem(den.epochsNodes); err != nil {
		return err
	}
	if err := validateNotNegativeInt(int(den.DelaySec), "delaySec"); err != nil {
		return err
	}
	return nil
}

func validateTransactions(txs []transactions) error {
	for _, txsItem := range txs {
		if err := validateTransactionsItem(txsItem); err != nil {
			return err
		}
	}
	return nil
}

func validateTransactionsItem(tx transactions) error {
	if err := validateEpochsNodesItem(tx.epochsNodes); err != nil {
		return err
	}
	if err := validatePositiveInt(tx.PeriodMs, "periodMs"); err != nil {
		return err
	}
	return nil
}

func validateEpochsNodesItem(en epochsNodes) error {
	_, err := parseNums(en.Epochs)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to parse epochs str %s: %s", en.Epochs, err.Error()))
	}
	_, err = parseNums(en.Nodes)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to parse nodes str %s: %s", en.Nodes, err.Error()))
	}
	return nil
}

func validateCeremonies(ceremonies []ceremony, epochUsersCount []int) error {
	epochs := make(map[int]bool)
	for _, c := range ceremonies {
		if err := validateCeremony(c, epochUsersCount, epochs); err != nil {
			return err
		}
	}
	return nil
}

func buildEpochUserCounts(sc *incomingScenario) []int {
	var epochUserCounts []int
	epochUserCounts = append(epochUserCounts, sc.Users)
	epochUserCountsMap := make(map[int]int)
	var maxEpoch int
	newUsers := append(sc.NewUsers, sc.NewUsersAfterFlips...)
	for _, epochNewUsers := range newUsers {
		epochs, _ := parseNums(epochNewUsers.Epochs)
		for _, epoch := range epochs {
			epochUserCountsMap[epoch] += epochNewUsers.Count
			if maxEpoch < epoch {
				maxEpoch = epoch
			}
		}
	}
	if c, ok := epochUserCountsMap[0]; ok {
		epochUserCounts[0] += c
	}
	for i := 1; i <= maxEpoch; i++ {
		epochUserCounts = append(epochUserCounts, epochUserCounts[i-1]+epochUserCountsMap[i])
	}
	return epochUserCounts
}

func validateCeremony(c ceremony, epochUsersCount []int, usedEpochs map[int]bool) error {
	if err := validateEpochs(c.Epochs, usedEpochs); err != nil {
		return err
	}
	epochs, _ := parseNums(c.Epochs)
	maxEpoch := epochs[len(epochs)-1]
	var usersCount int
	if maxEpoch > len(epochUsersCount)-1 {
		usersCount = epochUsersCount[len(epochUsersCount)-1]
	} else {
		usersCount = epochUsersCount[maxEpoch]
	}
	if err := validateUserCeremonies(c.UserCeremonies, usersCount); err != nil {
		return err
	}
	if err := validateAssertion(c.Assertion); err != nil {
		return err
	}
	return nil
}

func validateEpochNode(epoch int, node *int, epochNodes map[string]bool) error {
	if err := validateNotNegativeInt(epoch, "epoch"); err != nil {
		return err
	}
	key := fmt.Sprintf("%v_%v", epoch, node)
	if epochNodes[key] {
		return errors.New(fmt.Sprintf("There is more than 1 section with epoch=%d and node=%d", epoch, node))
	}
	epochNodes[key] = true
	return nil
}

func validateEpoch(epoch int, epochs map[int]bool) error {
	if err := validateNotNegativeInt(epoch, "epoch"); err != nil {
		return err
	}
	if epochs[epoch] {
		return errors.New(fmt.Sprintf("There is more than 1 ceremony with epoch=%d", epoch))
	}
	epochs[epoch] = true
	return nil
}

func validateUserCeremonies(userCeremonies []userCeremony, usersCount int) error {
	usedUsers := make(map[int]bool)
	for _, uc := range userCeremonies {
		if err := validateUserCeremony(uc, usersCount, usedUsers); err != nil {
			return err
		}
	}
	return nil
}

func validateUserCeremony(uc userCeremony, usersCount int, usedUsers map[int]bool) error {
	if err := validateCeremonyUsers(uc.Users, usersCount, usedUsers); err != nil {
		return err
	}
	if uc.SubmitFlips != nil {
		if err := validateNotNegativeInt(*uc.SubmitFlips, "submitFlips"); err != nil {
			return err
		}
	}
	if err := validateSessionAnswers(uc.ShortAnswerRates, uc.ShortAnswers, "short"); err != nil {
		return err
	}
	if err := validateSessionAnswers(uc.LongAnswerRates, uc.LongAnswers, "long"); err != nil {
		return err
	}
	return nil
}

func validateSessionAnswers(rates *answerRates, answers []answer, session string) error {
	if rates == nil && len(answers) == 0 {
		return nil
	}
	if rates != nil && len(answers) > 0 {
		return errors.New(fmt.Sprintf("Invalid answers for %s session: answerRates and answers are both not empty", session))
	}
	if rates != nil {
		return validateAnswerRates(rates)
	}
	return validateAnswers(answers)
}

func validateAnswerRates(rates *answerRates) error {
	if err := validateNotNegativeFloat(rates.Left, "left"); err != nil {
		return err
	}
	if err := validateNotNegativeFloat(rates.Right, "right"); err != nil {
		return err
	}
	if err := validateNotNegativeFloat(rates.Inappropriate, "inappropriate"); err != nil {
		return err
	}
	amount := rates.Left + rates.Right + rates.Inappropriate
	if amount > 1 {
		return errors.New(fmt.Sprintf("Rates total amount mustn't be greater than 1.0, actual amount: %f", amount))
	}
	return nil
}

func validateAnswers(answers []answer) error {
	for _, a := range answers {
		if err := validateAnswer(a); err != nil {
			return err
		}
	}
	return nil
}

func validateAnswer(a answer) error {
	if a.Answer > 3 {
		return errors.New(fmt.Sprintf("Invalid answer %d", a.Answer))
	}
	return nil
}

func validateCeremonyUsers(usersStr string, usersCount int, usedUsers map[int]bool) error {
	users, err := parseNums(usersStr)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to parse users str %s: %s", usersStr, err.Error()))
	}
	for _, user := range users {
		if err := validateCeremonyUser(user, usersCount, usedUsers); err != nil {
			return err
		}
	}
	return nil
}

func validateCeremonyUser(user int, usersCount int, usedUsers map[int]bool) error {
	if err := validateNotNegativeInt(user, "user"); err != nil {
		return err
	}
	if user >= usersCount {
		return errors.New(fmt.Sprintf("Invalid user %d, available users count: %d", user, usersCount))
	}
	if usedUsers[user] {
		return errors.New(fmt.Sprintf("There is more than 1 user ceremony with user=%d", user))
	}
	usedUsers[user] = true
	return nil
}

func validateAssertion(a *assertion) error {
	return nil
}
