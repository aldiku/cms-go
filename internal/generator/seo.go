package generator

import (
	"cms-go/internal/models"
	"html/template"
	"strings"
)

// BuildSEOHead renders the <title>, meta description, canonical link, robots
// directive, and Open Graph/Twitter Card tags for a page, applying the same
// fallback chain WordPress SEO plugins use (SEO title -> title, social
// fields -> meta title/description, etc). All user-supplied values are
// escaped here since this is the one place they become raw <head> markup.
func BuildSEOHead(page models.Page, siteURL string) template.HTML {
	title := firstNonEmpty(page.MetaTitle, page.Title)
	description := page.MetaDescription
	canonical := firstNonEmpty(page.CanonicalURL, pageURL(siteURL, page.Slug))

	ogTitle := firstNonEmpty(page.OGTitle, title)
	ogDescription := firstNonEmpty(page.OGDescription, description)
	ogImage := page.OGImage

	twitterTitle := firstNonEmpty(page.TwitterTitle, ogTitle)
	twitterDescription := firstNonEmpty(page.TwitterDescription, ogDescription)
	twitterImage := firstNonEmpty(page.TwitterImage, ogImage)
	twitterCard := page.TwitterCard
	if twitterCard == "" {
		if twitterImage != "" {
			twitterCard = "summary_large_image"
		} else {
			twitterCard = "summary"
		}
	}

	var b strings.Builder

	b.WriteString("<title>" + esc(title) + "</title>\n")

	if description != "" {
		writeMeta(&b, "name", "description", description)
	}

	if canonical != "" {
		b.WriteString(`<link rel="canonical" href="` + esc(canonical) + `">` + "\n")
	}

	if robots := robotsContent(page.MetaRobotsNoindex, page.MetaRobotsNofollow); robots != "" {
		writeMeta(&b, "name", "robots", robots)
	}

	writeMeta(&b, "property", "og:type", "website")
	writeMeta(&b, "property", "og:title", ogTitle)
	if ogDescription != "" {
		writeMeta(&b, "property", "og:description", ogDescription)
	}
	if ogImage != "" {
		writeMeta(&b, "property", "og:image", ogImage)
	}
	if canonical != "" {
		writeMeta(&b, "property", "og:url", canonical)
	}

	writeMeta(&b, "name", "twitter:card", twitterCard)
	writeMeta(&b, "name", "twitter:title", twitterTitle)
	if twitterDescription != "" {
		writeMeta(&b, "name", "twitter:description", twitterDescription)
	}
	if twitterImage != "" {
		writeMeta(&b, "name", "twitter:image", twitterImage)
	}

	return template.HTML(b.String())
}

func writeMeta(b *strings.Builder, attr, key, content string) {
	b.WriteString(`<meta ` + attr + `="` + esc(key) + `" content="` + esc(content) + `">` + "\n")
}

func robotsContent(noindex, nofollow bool) string {
	switch {
	case noindex && nofollow:
		return "noindex,nofollow"
	case noindex:
		return "noindex"
	case nofollow:
		return "nofollow"
	default:
		return ""
	}
}

// pageURL builds an absolute URL for a page's slug under siteURL. Returns ""
// if siteURL isn't configured, since a bare-path canonical isn't useful.
func pageURL(siteURL, slug string) string {
	if siteURL == "" {
		return ""
	}
	slug = strings.TrimPrefix(slug, "/")
	if slug == "" {
		return siteURL + "/"
	}
	return siteURL + "/" + slug
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func esc(s string) string {
	return template.HTMLEscapeString(s)
}
