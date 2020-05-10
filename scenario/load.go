package scenario

import (
	"encoding/json"
	"github.com/idena-network/idena-test-go/common"
	"io/ioutil"
	"path/filepath"
)

var defaultScenario Scenario

func init() {
	defaultScenario = Scenario{
		EpochNewUsersBeforeFlips: map[int][]*NewUsers{
			0: {
				{
					Inviter: 0,
					Count:   5,
				},
			},
		},
		DefaultAnswer:     defaultDefaultAnswer,
		CeremonyMinOffset: 5,
	}
}

func GetDefaultScenario() Scenario {
	return defaultScenario
}

func Load(workDir string, fileName string) Scenario {
	incomingScenario := load(workDir, fileName)
	if err := incomingScenario.validate(); err != nil {
		panic(err)
	}
	sc := convert(incomingScenario)
	return sc
}

func load(workDir string, fileName string) incomingScenario {
	var scenarioJson []byte
	var err error
	if common.IsValidUrl(fileName) {
		scenarioJson, err = common.LoadData(fileName)
	} else {
		scenarioJson, err = ioutil.ReadFile(filepath.Join(workDir, fileName))
	}
	if err != nil {
		panic(err)
	}
	scenario := incomingScenario{}
	if err := json.Unmarshal(scenarioJson, &scenario); err != nil {
		panic(err)
	}
	return scenario
}
