package process

import (
	"errors"
	"fmt"
	"idena-test-go/client"
	"idena-test-go/log"
	"idena-test-go/scenario"
	"idena-test-go/user"
	"sync"
)

type epochState struct {
	userStates map[int]*userEpochState // user index -> userEpochState
}

type userEpochState struct {
	madeFlips     int
	requiredFlips int
}

type errorsHolder struct {
	errors []error
}

func (process *Process) assert(epoch int, es epochState) {
	nodeStates := process.getNodeStates()
	var identities []client.Identity
	for _, u := range process.users {
		identities = append(identities, process.getIdentity(u))
	}

	process.logStats(nodeStates, identities, es)

	ceremony := process.sc.Ceremonies[epoch]
	if ceremony == nil {
		log.Info("Skipped assertion due to empty scenario ceremony")
		return
	}
	assertion := ceremony.Assertion
	if assertion == nil {
		log.Info("Skipped assertion due to empty scenario ceremony assertion")
		return
	}
	eh := &errorsHolder{}

	process.assertNodeStates(nodeStates, assertion.States, eh)

	process.assertNodes(assertion.Nodes, identities, es.userStates, eh)

	if len(eh.errors) <= 0 {
		log.Info("Assertion passed")
		return
	}
	for _, e := range eh.errors {
		log.Error(e.Error())
	}
	process.handleError(errors.New("assertion failed"), "")
}

func (process *Process) logStats(nodeStates map[string]int, identities []client.Identity, es epochState) {
	log.Info("---------------- Verification session stats ----------------")
	for state, count := range nodeStates {
		log.Info(fmt.Sprintf("State %s, count: %d", state, count))
	}
	for i, u := range process.users {
		identity := identities[i]
		userEpochState := es.userStates[i]
		log.Info(fmt.Sprintf("%s state: %s, made flips: %d, required flips: %d, available invites: %d",
			u.GetInfo(), identity.State, userEpochState.madeFlips, userEpochState.requiredFlips, identity.Invites))
	}
	log.Info("------------------------------------------------------------")
}

func (process *Process) assertNodeStates(nodeStates map[string]int, states []scenario.StateAssertion, eh *errorsHolder) {
	for _, state := range states {
		if nodeStates[state.State] != state.Count {
			process.assertionError(fmt.Sprintf("Wrong states count for %s", state.State), state.Count, nodeStates[state.State], eh)
		}
	}
}

func (process *Process) getNodeStates() map[string]int {
	states := make(map[string]int)
	wg := sync.WaitGroup{}
	activeUsers := process.users
	activeUsersCount := len(activeUsers)
	wg.Add(activeUsersCount)
	mutex := sync.Mutex{}
	for _, u := range activeUsers {
		go func(u *user.User) {
			mutex.Lock()
			defer mutex.Unlock()
			identity := process.getIdentity(u)
			states[identity.State]++
			wg.Done()
		}(u)
	}
	wg.Wait()
	return states
}

func (process *Process) assertNodes(nodes map[int]*scenario.NodeAssertion, identities []client.Identity, userEpochStates map[int]*userEpochState, eh *errorsHolder) {
	for userIndex, node := range nodes {
		if userIndex >= len(process.users) {
			process.assertionError(fmt.Sprintf("Assertion for user %d is present, but user is absent", userIndex), nil, nil, eh)
			continue
		}
		process.assertNode(process.users[userIndex], node, identities, userEpochStates[userIndex], eh)
	}
}

func (process *Process) assertNode(u *user.User, node *scenario.NodeAssertion, identities []client.Identity, ues *userEpochState, eh *errorsHolder) {
	if node == nil {
		return
	}
	identity := identities[u.Index]

	if identity.State != node.State {
		process.assertionError(fmt.Sprintf("Wrong state for node %s", u.GetInfo()), node.State, identity.State, eh)
	}

	if ues.madeFlips != node.MadeFlips {
		process.assertionError(fmt.Sprintf("Wrong made flips for node %s", u.GetInfo()), ues.madeFlips, identity.MadeFlips, eh)
	}

	if ues.requiredFlips != node.RequiredFlips {
		process.assertionError(fmt.Sprintf("Wrong required flips for node %s", u.GetInfo()), ues.requiredFlips, identity.RequiredFlips, eh)
	}

	if identity.Invites != node.AvailableInvites {
		process.assertionError(fmt.Sprintf("Wrong available invites for node %s", u.GetInfo()), node.AvailableInvites, identity.Invites, eh)
	}
}

func (process *Process) assertionError(msg string, expected interface{}, actual interface{}, holder *errorsHolder) {
	holder.errors = append(holder.errors, errors.New(fmt.Sprintf("%s, expected: %v, actual: %v", msg, expected, actual)))
}
