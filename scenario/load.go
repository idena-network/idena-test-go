package scenario

import (
	"encoding/json"
	"github.com/idena-network/idena-test-go/common"
	"io/ioutil"
	"path/filepath"
)

var defaultScenario Scenario

func init() {
	inviter := 0
	defaultScenario = Scenario{
		EpochNewUsersBeforeFlips: map[int][]*NewUsers{
			0: {
				{
					Inviter: &inviter,
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

func Load(workDir string, fileName string, godMode bool, nodes int) Scenario {
	incomingScenario := load(workDir, fileName)
	if err := incomingScenario.validate(); err != nil {
		panic(err)
	}
	sc := convert(incomingScenario, godMode, nodes)
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
