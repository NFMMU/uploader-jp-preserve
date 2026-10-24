package api

import "testing"

func TestShouldQueueSurplusDeletion(t *testing.T) {
	tests := []struct {
		name        string
		healthy     int64
		activeTasks int64
		want        bool
	}{
		{name: "minimum only", healthy: 2, want: false},
		{name: "one spare allowed", healthy: 3, want: false},
		{name: "surplus with no active task", healthy: 4, want: true},
		{name: "large surplus", healthy: 7, want: true},
		{name: "do not delete during active work", healthy: 5, activeTasks: 1, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldQueueSurplusDeletion(tt.healthy, tt.activeTasks); got != tt.want {
				t.Fatalf("shouldQueueSurplusDeletion(%d, %d) = %v, want %v", tt.healthy, tt.activeTasks, got, tt.want)
			}
		})
	}
}
