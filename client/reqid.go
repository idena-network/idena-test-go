package client

import "sync"

type ReqIdHolder struct {
	mutex *sync.Mutex
	id    int
}

func NewReqIdHolder() *ReqIdHolder {
	return &ReqIdHolder{
		mutex: &sync.Mutex{},
	}
}

func (holder *ReqIdHolder) GetNextReqId() int {
	holder.mutex.Lock()
	defer holder.mutex.Unlock()
	holder.id++
	return holder.id
}
