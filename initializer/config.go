package initializer

import (
	"encoding/json"
	"io/ioutil"
)

type config struct {
	Instances      int
	GodHost        string
	BotConfig      string
	NodeBaseConfig string
	Scenario       string
	NodeApp        string
	BotApp         string
}

func loadFromFile(path string) config {
	configJson, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	c := config{}
	if err := json.Unmarshal(configJson, &c); err != nil {
		panic(err)
	}
	return c
}
