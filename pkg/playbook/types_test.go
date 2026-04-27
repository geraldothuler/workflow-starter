package playbook

import "testing"

func TestPlaybookSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    PlaybookSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec",
			spec: PlaybookSpec{
				ID:    "test-playbook",
				Title: "Test Playbook",
				Steps: []PlaybookStep{
					{
						ID:       "step1",
						Provider: "postgresql",
						Analyzers: []AnalyzerRef{
							{Name: "analyze_inactive_slots"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			spec: PlaybookSpec{
				Title: "Test",
				Steps: []PlaybookStep{
					{
						ID:       "step1",
						Provider: "postgresql",
						Analyzers: []AnalyzerRef{
							{Name: "test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "playbook id is required",
		},
		{
			name: "missing title",
			spec: PlaybookSpec{
				ID: "test",
				Steps: []PlaybookStep{
					{
						ID:       "step1",
						Provider: "postgresql",
						Analyzers: []AnalyzerRef{
							{Name: "test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "playbook title is required",
		},
		{
			name: "no steps",
			spec: PlaybookSpec{
				ID:    "test",
				Title: "Test",
			},
			wantErr: true,
			errMsg:  "playbook must have at least one step",
		},
		{
			name: "step missing provider",
			spec: PlaybookSpec{
				ID:    "test",
				Title: "Test",
				Steps: []PlaybookStep{
					{
						ID: "step1",
						Analyzers: []AnalyzerRef{
							{Name: "test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "provider is required",
		},
		{
			name: "step missing id",
			spec: PlaybookSpec{
				ID:    "test",
				Title: "Test",
				Steps: []PlaybookStep{
					{
						Provider: "postgresql",
						Analyzers: []AnalyzerRef{
							{Name: "test"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "id is required",
		},
		{
			name: "step no analyzers",
			spec: PlaybookSpec{
				ID:    "test",
				Title: "Test",
				Steps: []PlaybookStep{
					{
						ID:       "step1",
						Provider: "postgresql",
					},
				},
			},
			wantErr: true,
			errMsg:  "at least one analyzer is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if got := err.Error(); got != "" {
					// Just check it contains the expected message
					if !containsStr(got, tt.errMsg) {
						t.Errorf("error = %q, want to contain %q", got, tt.errMsg)
					}
				}
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
