package notify

import "testing"

func TestRender(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		params map[string]string
		want   string
	}{
		{
			name:   "simple substitution",
			body:   "Hello {{user_name}}, welcome to {{site_name}}!",
			params: map[string]string{"user_name": "Ada", "site_name": "Adsqoo"},
			want:   "Hello Ada, welcome to Adsqoo!",
		},
		{
			name:   "tolerates internal whitespace in braces",
			body:   "{{  user_name }} / {{user_name  }}",
			params: map[string]string{"user_name": "Ada"},
			want:   "Ada / Ada",
		},
		{
			name:   "unmapped placeholder left visible, not blanked",
			body:   "Hi {{user_name}}, code: {{missing_key}}",
			params: map[string]string{"user_name": "Ada"},
			want:   "Hi Ada, code: {{missing_key}}",
		},
		{
			name:   "no placeholders is a no-op",
			body:   "<p>Plain HTML, no tokens here.</p>",
			params: map[string]string{"user_name": "Ada"},
			want:   "<p>Plain HTML, no tokens here.</p>",
		},
		{
			name:   "repeated placeholder substituted every occurrence",
			body:   "{{user_name}} {{user_name}} {{user_name}}",
			params: map[string]string{"user_name": "Ada"},
			want:   "Ada Ada Ada",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Render(tc.body, tc.params)
			if got != tc.want {
				t.Errorf("Render(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}
