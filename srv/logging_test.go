package srv

import "testing"

func TestIsStaticPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/static/style.css", true},
		{"/static/app.js", true},
		{"/sw.js", true},
		{"/api/favicon", true},
		{"/", false},
		{"/feeds", false},
		{"/api/feeds", false},
		{"/api/articles/123/read", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isStaticPath(tt.path)
			if got != tt.want {
				t.Errorf("isStaticPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
