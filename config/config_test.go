package config

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadConfig(t *testing.T) {
	type appConfig struct {
		Opt1 string `yaml:"opt1"`
		Opt2 int    `yaml:"opt2"`
	}

	type app2Config struct {
		Opt1 int    `yaml:"opt1"`
		Opt2 string `yaml:"opt2"`
	}

	type config struct {
		App  appConfig  `yaml:"app"`
		App2 app2Config `yaml:"app2"`
	}

	cExp := "app:\n  opt1: opt1\n  opt2: 2\napp2:\n  opt1: 2\n  opt2: opt2"
	dir, _ := os.Getwd()
	ioutil.WriteFile("config.yaml", []byte(cExp), 0644)

	cAct := new(config)
	ReadConfig(dir+"/config.yaml", cAct)

	assert.Equal(t, "opt1", cAct.App.Opt1)
	assert.Equal(t, 2, cAct.App.Opt2)
	assert.Equal(t, 2, cAct.App2.Opt1)
	assert.Equal(t, "opt2", cAct.App2.Opt2)
}
