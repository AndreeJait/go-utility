package configw

import (
	"github.com/AndreeJait/go-utility/errow"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type ConfigMode string

type LocationMap map[ConfigMode]string

type ConfigW[T interface{}] struct {
	ConfigLocations LocationMap
	ConfigMode      ConfigMode
}

func (cfgw ConfigW[T]) LoadConfig() (cfg *T, err error) {

	var fileLocation string

	if val, ok := cfgw.ConfigLocations[cfgw.ConfigMode]; !ok {
		return nil, errow.ErrConfigNotFound
	} else {
		fileLocation = val
	}

	fileName, _ := filepath.Abs(fileLocation)
	fileYaml, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(fileYaml, &cfg)
	return
}

func New[T interface{}](configLocations LocationMap, configMode ConfigMode) ConfigW[T] {
	return ConfigW[T]{
		ConfigLocations: configLocations,
		ConfigMode:      configMode,
	}
}
