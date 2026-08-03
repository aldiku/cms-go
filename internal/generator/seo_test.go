package generator

import (
	"cms-go/internal/models"
	"strings"
	"testing"
)

func TestBuildSEOHead_FeaturedImageFallback(t *testing.T) {
	tests := []struct {
		name     string
		page     models.Page
		siteURL  string
		want     []string
		notWant  []string
	}{
		{
			name: "MetaTitle empty - uses Page.Title for all title tags",
			page: models.Page{
				Title:       "My Page Title",
				MetaTitle:   "", // empty - should fallback to Title
				Slug:        "my-page",
				OGImage:     "", // empty - should use featured image
				FeaturedImage: models.Media{
					URL: "/assets/uploads/2026/08/featured.jpg",
				},
			},
			siteURL: "https://example.com",
			want: []string{
				"<title>My Page Title</title>",
				`<meta property="og:title" content="My Page Title">`,
				`<meta name="twitter:title" content="My Page Title">`,
				`<meta property="og:image" content="/assets/uploads/2026/08/featured.jpg">`,
				`<meta name="twitter:image" content="/assets/uploads/2026/08/featured.jpg">`,
			},
		},
		{
			name: "MetaTitle set - overrides Page.Title",
			page: models.Page{
				Title:       "Page Title",
				MetaTitle:   "Custom SEO Title",
				Slug:        "my-page",
				OGImage:     "",
				FeaturedImage: models.Media{
					URL: "/assets/uploads/2026/08/featured.jpg",
				},
			},
			siteURL: "https://example.com",
			want: []string{
				"<title>Custom SEO Title</title>",
				`<meta property="og:title" content="Custom SEO Title">`,
				`<meta name="twitter:title" content="Custom SEO Title">`,
			},
		},
		{
			name: "OGImage set - takes priority over featured image",
			page: models.Page{
				Title:         "My Page",
				MetaTitle:     "",
				Slug:          "my-page",
				OGImage:       "/assets/custom-og.jpg",
				TwitterImage:  "",
				FeaturedImage: models.Media{
					URL: "/assets/uploads/2026/08/featured.jpg",
				},
			},
			siteURL: "https://example.com",
			want: []string{
				`<meta property="og:image" content="/assets/custom-og.jpg">`,
				`<meta name="twitter:image" content="/assets/custom-og.jpg">`,
			},
			notWant: []string{
				"/assets/uploads/2026/08/featured.jpg",
			},
		},
		{
			name: "No featured image - og/twitter images empty",
			page: models.Page{
				Title:         "My Page",
				MetaTitle:     "",
				Slug:          "my-page",
				OGImage:       "",
				TwitterImage:  "",
				FeaturedImage: models.Media{}, // empty featured image
			},
			siteURL: "https://example.com",
			want: []string{
				"<title>My Page</title>",
			},
			notWant: []string{
				`<meta property="og:image"`,
				`<meta name="twitter:image"`,
			},
		},
		{
			name: "TwitterImage set - overrides ogImage (which has featured image)",
			page: models.Page{
				Title:         "My Page",
				MetaTitle:     "",
				Slug:          "my-page",
				OGImage:       "",
				TwitterImage:  "/assets/twitter-card.jpg",
				FeaturedImage: models.Media{
					URL: "/assets/uploads/2026/08/featured.jpg",
				},
			},
			siteURL: "https://example.com",
			want: []string{
				`<meta property="og:image" content="/assets/uploads/2026/08/featured.jpg">`,
				`<meta name="twitter:image" content="/assets/twitter-card.jpg">`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(BuildSEOHead(tt.page, tt.siteURL))

			// Check wanted strings
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("Expected to find %q in output, but didn't.\nGot:\n%s", want, result)
				}
			}

			// Check not-wanted strings
			for _, notWant := range tt.notWant {
				if strings.Contains(result, notWant) {
					t.Errorf("Did NOT expect to find %q in output, but did.\nGot:\n%s", notWant, result)
				}
			}
		})
	}
}
