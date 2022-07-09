package path

import "testing"

// TODO: more tests
func TestJoin(t *testing.T) {
	type args struct {
		base  string
		paths []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "git-repo",
			args: args{
				base:  "github.com",
				paths: []string{"bootun", "veronica"},
			},
			want: "github.com/bootun/veronica",
		},
		{
			name: "relative-local-path",
			args: args{
				base:  "./",
				paths: []string{"bootun", "veronica", "veronica.yml"},
			},
			want: "bootun/veronica/veronica.yml",
		},
		{
			name: "abstract-local-path",
			args: args{
				base:  "/home/code/github.com",
				paths: []string{"bootun", "veronica", "veronica.yaml"},
			},
			want: "/home/code/github.com/bootun/veronica/veronica.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := New(tt.args.base).Join(tt.args.paths...); got.String() != tt.want {
				t.Errorf("Join() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TODO: perfect it
func TestRel(t *testing.T) {
	rel, err := New("/bootun").Rel("/home/bootun")
	if err != nil {
		t.Error(err)
	}
	t.Log(rel.String())
}