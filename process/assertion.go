package process

import (
	"errors"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/idena-network/idena-test-go/user"
	"sync"
)

type epochState struct {
	userStates map[int]*userEpochState // user index -> userEpochState
	wordsByCid map[string][2]uint32
}

type userEpochState struct {
	madeFlips      int
	requiredFlips  int
	availableFlips int
}

type errorsHolder struct {
	errors []error
}

func (process *Process) assert(epoch int, es epochState) {
	nodeStates := process.getNodeStates()
	identitiesPerUserIdx := make(map[int]api.Identity)
	for _, u := range process.users {
		if !u.Active {
			continue
		}
		identitiesPerUserIdx[u.Index] = process.getIdentity(u)
	}

	process.logStats(nodeStates, identitiesPerUserIdx, es)

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

	process.assertNodes(assertion.Nodes, identitiesPerUserIdx, es.userStates, eh)

	if len(eh.errors) <= 0 {
		log.Info("Assertion passed")
		return
	}
	for _, e := range eh.errors {
		log.Error(e.Error())
	}
	process.handleError(errors.New("assertion failed"), "")
}

func (process *Process) logStats(nodeStates map[string]int, identitiesPerUserIdx map[int]api.Identity, es epochState) {
	log.Info("---------------- Verification session stats ----------------")
	for state, count := range nodeStates {
		log.Info(fmt.Sprintf("State %s, count: %d", state, count))
	}
	for i, u := range process.users {
		identity, present := identitiesPerUserIdx[i]
		if !present {
			log.Info(fmt.Sprintf("%s stopped", u.GetInfo()))
			continue
		}
		userEpochState := es.userStates[i]
		log.Info(fmt.Sprintf("%s state: %s, made flips: %d, required flips: %d, available flips: %d, available invites: %d, online: %t",
			u.GetInfo(), identity.State, userEpochState.madeFlips, userEpochState.requiredFlips, userEpochState.availableFlips, identity.Invites, identity.Online))
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
	activeUsers := process.getActiveUsers()
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

func (process *Process) getActiveUsers() []*user.User {
	var activeUsers []*user.User
	for _, u := range process.users {
		if u.Active {
			activeUsers = append(activeUsers, u)
		}
	}
	return activeUsers
}

func (process *Process) assertNodes(nodes map[int]*scenario.NodeAssertion, identitiesPerUserIdx map[int]api.Identity, userEpochStates map[int]*userEpochState, eh *errorsHolder) {
	for userIndex, node := range nodes {
		if userIndex >= len(process.users) {
			process.assertionError(fmt.Sprintf("Assertion for user %d is present, but user is absent", userIndex), nil, nil, eh)
			continue
		}
		process.assertNode(process.users[userIndex], node, identitiesPerUserIdx, userEpochStates[userIndex], eh)
	}
}

func (process *Process) assertNode(u *user.User, node *scenario.NodeAssertion, identitiesPerUserIdx map[int]api.Identity, ues *userEpochState, eh *errorsHolder) {
	if node == nil {
		return
	}
	identity, present := identitiesPerUserIdx[u.Index]

	if !present {
		log.Warn(fmt.Sprintf("Unable to assert state for node %s as it is stopped", u.GetInfo()))
		return
	}

	if node.State != nil && identity.State != *node.State {
		process.assertionError(fmt.Sprintf("Wrong state for node %s", u.GetInfo()), *node.State, identity.State, eh)
	}

	if node.MadeFlips != nil && ues.madeFlips != *node.MadeFlips && !(process.godMode && u.Index == process.godUser.Index && process.getCurrentTestIndex() == 0) {
		process.assertionError(fmt.Sprintf("Wrong made flips for node %s", u.GetInfo()), *node.MadeFlips, ues.madeFlips, eh)
	}

	if node.RequiredFlips != nil && ues.requiredFlips != *node.RequiredFlips {
		process.assertionError(fmt.Sprintf("Wrong required flips for node %s", u.GetInfo()), *node.RequiredFlips, ues.requiredFlips, eh)
	}

	if node.AvailableInvites != nil && identity.Invites != uint8(*node.AvailableInvites) {
		process.assertionError(fmt.Sprintf("Wrong available invites for node %s", u.GetInfo()), *node.AvailableInvites, identity.Invites, eh)
	}

	if node.Online != nil && identity.Online != *node.Online {
		process.assertionError(fmt.Sprintf("Wrong online state for node %s", u.GetInfo()), *node.Online, identity.Online, eh)
	}
}

func (process *Process) assertionError(msg string, expected interface{}, actual interface{}, holder *errorsHolder) {
	holder.errors = append(holder.errors, errors.New(fmt.Sprintf("%s, expected: %v, actual: %v", msg, expected, actual)))
}
