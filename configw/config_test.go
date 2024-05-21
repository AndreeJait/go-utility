package configw

import (
	"reflect"
	"testing"
)

type Config struct {
	Testing Testing `json:"testing" yaml:"testing"`
}

type Testing struct {
	Name string `json:"name" yaml:"name"`
}

func TestConfigW_LoadConfig(t *testing.T) {
	type testCase[T Config] struct {
		name    string
		cfgw    ConfigW[T]
		wantCfg *T
		wantErr bool
	}
	tests := []testCase[Config]{
		{
			name: "success to convert",
			cfgw: New[Config](LocationMap{
				"testing": "./example.yaml",
			}, "testing"),
			wantCfg: &Config{
				Testing: Testing{
					Name: "Andree",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCfg, err := tt.cfgw.LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotCfg, tt.wantCfg) {
				t.Errorf("LoadConfig() gotCfg = %v, want %v", gotCfg, tt.wantCfg)
			}
		})
	}
}
