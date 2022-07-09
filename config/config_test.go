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
			name: "v0",
			args: args{
				content: []byte(configV0),
			},
			want: &Config{
				Version: "0.0.1",
				Project: ProjectInfo{
					GoMod:      "./go.mod",
					Entrypoint: []string{"cmd/api-gateway", "./cmd/assets-manager"},
					Ignore:     []string{"xxx.go"},
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

var configV0 = `
version: 0.0.1
project:
  go.mod: ./go.mod
  # your service
  entrypoint:
    - cmd/api-gateway
    - ./cmd/assets-manager
  ignore:
    - xxx.go

`
