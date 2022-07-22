package config

import (
	"reflect"
	"testing"
)

func TestParseConfig(t *testing.T) {
	type args struct {
		content []byte
	}
	var tests = []struct {
		name    string
		args    args
		want    *Config
		wantErr bool
	}{
		{
			name: "v0.1",
			args: args{
				content: []byte(configV01),
			},
			want: &Config{
				Version: "0.1.0",
				Services: map[string]*Service{
					"api-gateway": &Service{
						Name:       "api-gateway",
						Entrypoint: "cmd/api-gateway",
						Ignore:     []string{"xxx.go"},
						Hooks:      []string{"a.file"},
					},
					"assets-manager": &Service{
						Name:       "assets-manager",
						Entrypoint: "cmd/assets-manager",
						Ignore:     nil,
						Hooks:      nil,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfig(tt.args.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("parseConfig() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
var configV01 = `
version: 0.1.0
services: 
  api-gateway:
    # main package
    entrypoint: cmd/api-gateway
    ignore:
      - xxx.go
    hooks:
      - a.file
  assets-manager:
    entrypoint: cmd/assets-manager
`
