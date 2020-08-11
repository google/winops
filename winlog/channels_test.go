// +build windows

package winlog

import (
	"testing"

	"bitbucket.org/creachadair/stringset"
)

func TestAvailableChannels(t *testing.T) {
	tests := []struct {
		name    string
		want    []string
		wantErr bool
	}{
		{
			// Exists on all Windows machines.
			name:    "must exist",
			want:    []string{"Application", "Security", "System"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AvailableChannels()
			if (err != nil) != tt.wantErr {
				t.Errorf("AvailableChannels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !stringset.New(got...).Contains(tt.want...) {
				t.Errorf("AvailableChannels() = %v, must contain %v", got, tt.want)
			}
		})
	}
}
