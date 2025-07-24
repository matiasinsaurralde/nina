package git

import (
	"testing"
)

func TestExtractAppNameFromRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
		wantErr  bool
	}{
		{
			name:     "SSH URL with .git suffix",
			repoURL:  "git@github.com:matiasinsaurralde/nina.git",
			expected: "nina",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL with .git suffix",
			repoURL:  "https://github.com/matiasinsaurralde/nina.git",
			expected: "nina",
			wantErr:  false,
		},
		{
			name:     "SSH URL without .git suffix",
			repoURL:  "git@github.com:matiasinsaurralde/nina",
			expected: "nina",
			wantErr:  false,
		},
		{
			name:     "HTTPS URL without .git suffix",
			repoURL:  "https://github.com/matiasinsaurralde/nina",
			expected: "nina",
			wantErr:  false,
		},
		{
			name:     "Complex app name with hyphens",
			repoURL:  "https://github.com/org/my-app-name.git",
			expected: "my-app-name",
			wantErr:  false,
		},
		{
			name:     "Empty URL",
			repoURL:  "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "URL with only slashes",
			repoURL:  "///",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractAppNameFromRepoURL(tt.repoURL)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractAppNameFromRepoURL() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractAppNameFromRepoURL() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("ExtractAppNameFromRepoURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractAppNameFromRepoURLEdgeCases(t *testing.T) {
	// Test with various URL formats
	testCases := []string{
		"git@github.com:user/repo.git",
		"https://github.com/user/repo.git",
		"ssh://git@github.com/user/repo.git",
		"git://github.com/user/repo.git",
		"https://gitlab.com/user/repo.git",
		"git@gitlab.com:user/repo.git",
	}

	for _, url := range testCases {
		appName, err := ExtractAppNameFromRepoURL(url)
		if err != nil {
			t.Errorf("Failed to extract app name from %s: %v", url, err)
			continue
		}

		if appName != "repo" {
			t.Errorf("Expected 'repo' for %s, got %s", url, appName)
		}
	}
}

func TestExtractAppNameFromRepoURLWithComplexNames(t *testing.T) {
	testCases := map[string]string{
		"git@github.com:user/my-awesome-app.git":      "my-awesome-app",
		"https://github.com/user/app_with_underscore": "app_with_underscore",
		"git@github.com:user/app123.git":              "app123",
		"https://github.com/user/app-name.git":        "app-name",
	}

	for url, expected := range testCases {
		appName, err := ExtractAppNameFromRepoURL(url)
		if err != nil {
			t.Errorf("Failed to extract app name from %s: %v", url, err)
			continue
		}

		if appName != expected {
			t.Errorf("Expected %s for %s, got %s", expected, url, appName)
		}
	}
}
