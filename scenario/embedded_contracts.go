package scenario

type ContractMethodCall struct {
	Method string
	Params interface{}
}

type Contracts struct {
	Amount string
	Calls  []ContractMethodCall
}
