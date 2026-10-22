package app

import (
	"slices"
	"testing"
)

func TestMissingPostingDetails(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want []string
	}{
		{
			name: "complete posting",
			job: Job{
				Title:       "Support Engineer",
				Company:     "Example Inc",
				Description: "Troubleshoot customer issues.",
			},
			want: nil,
		},
		{
			name: "missing company and description",
			job: Job{
				Title: "Support Engineer",
			},
			want: []string{
				"Employer name",
				"Job description",
			},
		},
		{
			name: "whitespace counts as missing",
			job: Job{
				Title:       "  ",
				Company:     "Example Inc",
				Description: "\n\t",
			},
			want: []string{
				"Job title",
				"Job description",
			},
		},
		{
			name: "empty posting",
			job:  Job{},
			want: []string{
				"Job title",
				"Employer name",
				"Job description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.job.MissingPostingDetails()

			if !slices.Equal(got, tt.want) {
				t.Errorf("MissingPostingDetails() = %v, want %v",
					got, tt.want)
			}
		})
	}
}
