package scenario

import (
	"encoding/json"
	"io/ioutil"
)

var defaultScenario Scenario

func init() {
	defaultScenario = Scenario{
		EpochNewUsers: map[int]int{
			0: 5,
		},
		CeremonyMinOffset: 5,
	}
}

func GetDefaultScenario() Scenario {
	return defaultScenario
}

func Load(fileName string) Scenario {
	incomingScenario := load(fileName)
	if err := incomingScenario.validate(); err != nil {
		panic(err)
	}
	sc := convert(incomingScenario)
	return sc
}

func load(fileName string) incomingScenario {
	scenarioJson, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(err)
	}
	scenario := incomingScenario{}
	if err := json.Unmarshal(scenarioJson, &scenario); err != nil {
		panic(err)
	}
	return scenario
}
