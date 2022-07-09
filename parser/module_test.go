package parser

import (
	"reflect"
	"testing"
)


func TestParseGoModContent(t *testing.T) {
	type args struct {
		gomod []byte
	}

	tests := []struct {
		name    string
		args    args
		want    *GoModuleInfo
		wantErr bool
	}{
		{
			name: "common",
			args: args{
				gomod: []byte(`module github.com/bootun/veronica
go 1.17`),
			},
			want: &GoModuleInfo{
				Name:      "github.com/bootun/veronica",
				GoVersion: "1.17",
			},
			wantErr: false,
		},
		{
			name: "no module",
			args: args{
				gomod: []byte(`
go 1.17`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no version",
			args: args{
				gomod: []byte(`module github.com/bootun/veronica`),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGoModContent(tt.args.gomod)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGoModuleInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseGoModuleInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
