package git

import (
	"reflect"
	"testing"
)

func TestGetMetricsForOrg(t *testing.T) {
	type args struct {
		org string
	}
	tests := []struct {
		name    string
		args    args
		want    RepoMetric
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "maliceio",
			args:    args{org: "malice-plugins"},
			want:    RepoMetric{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMetricsForOrg(tt.args.org)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetricsForOrg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMetricsForOrg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMetricsForUser(t *testing.T) {
	type args struct {
		user string
	}
	tests := []struct {
		name    string
		args    args
		want    RepoMetric
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "me",
			args:    args{user: "blacktop"},
			want:    RepoMetric{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMetricsForUser(tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetricsForUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMetricsForUser() = %v, want %v", got, tt.want)
			}
		})
	}
}
