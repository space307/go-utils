// Copyright 2016 Aleksandr Demakin. All rights reserved.

package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// MustReadConfig parses a yaml file and panics on an error.
func MustReadConfig(name string, config interface{}) {
	if err := ParseYamlFile(name, config); err == nil {
		return
	} else if os.IsNotExist(err) { // if 'name' file exists and cannot be parsed, do not check other locations.
		loc := dirLocator()
		for {
			dir, ok := loc()
			if !ok {
				break
			}
			fullName := dir + "config/" + name
			err := ParseYamlFile(fullName, config)
			if err == nil {
				return
			} else if !os.IsNotExist(err) {
				log.Errorf("Failed to open config '%s': %v", fullName, err)
			}
		}
	} else {
		log.Errorf("Failed to open config '%s': %v", name, err)
	}
	log.Fatalf("Unable to find config file '%s'", name)
}

// ParseYamlFile reads file with the given name and parses its content as yaml.
func ParseYamlFile(fullName string, data interface{}) error {
	bytes, err := ioutil.ReadFile(fullName)
	if err == nil {
		err = yaml.Unmarshal(bytes, data)
	}
	return err
}

// dirLocator returns a function which will iterate over a set of folders,
// where configuration files can be found. when finished, it returns "", false
func dirLocator() func() (string, bool) {
	var cur int
	return func() (string, bool) {
		cur++
		switch cur {
		case 1:
			return "", true
		case 2:
			if dir, err := os.Getwd(); err == nil {
				return dir + "/", true
			}
			fallthrough
		case 3:
			if dir, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
				return dir + "/", true
			}
			fallthrough
		case 4:
			// linux-only implementation from
			// https://github.com/kardianos/osext/blob/master/osext_procfs.go
			const deletedTag = " (deleted)"
			execpath, err := os.Readlink("/proc/self/exe")
			if err == nil {
				execpath = strings.TrimSuffix(execpath, deletedTag)
				execpath = strings.TrimPrefix(execpath, deletedTag)
				return filepath.Dir(execpath) + "/", true
			}
			fallthrough
		default:
			return "", false
		}
	}
}

func ReadConfig(explicitPath string, data interface{}) {
	if len(explicitPath) > 0 {
		if err := ParseYamlFile(explicitPath, data); err != nil {
			log.Fatalf("Failed to open config '%s': %v", explicitPath, err)
		}
	} else {
		MustReadConfig("config.yaml", data)
	}
}
