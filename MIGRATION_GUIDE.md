# WordPress Migration Guide

This document describes the WordPress-to-CMS migration tool implemented for the cms-go project.

## Task Summary

Migrate the first 5 posts and 5 pages from https://adsqoo.id WordPress site, with:
- Content stored as HTML (not file downloads)
- Featured images stored as URL references (phase 2: mount wp-content volume locally)
- Categories/tags created as needed
- Authors matched or assigned to current user

## Implementation

### Core Migrator Package
**File:** `internal/migrator/wordpress.go`

The migrator fetches data from WordPress REST API and saves to the local database:

#### Main Functions
- `MigrateWPData(defaultAuthorID)` — Entry point; fetches 5 posts + 5 pages, orchestrates migration
- `fetchWPPosts(page, perPage)` — GET `/wp/v2/posts` with `_embed` for author, featured media, taxonomy
- `fetchWPPages(page, perPage)` — GET `/wp/v2/pages` with `_embed` for author, featured media
- `migrateSinglePost(wpPost, defaultAuthorID, pageType)` — Converts WordPress post/page to local `Page` record

#### Data Processing
- **Featured Images:** Calls `createOrUpdateMediaFromWP()` to store media URL reference; looks up by URL to avoid duplicates
- **Authors:** Calls `findOrCreateAuthor()` to find user by WP author name or fall back to current user
- **Categories:** Calls `findOrCreateCategories()` to find or create by slug/name (for "post" type only)
- **Tags:** Calls `findOrCreateTags()` to find or create by slug/name (for "post" type only)

#### Database Records Created
- **Page** records with `type="post"` or `type="page"`
- **Media** records with `url` pointing to WordPress uploads
- **Category** records (as needed)
- **Tag** records (as needed)

### Admin Handler
**File:** `internal/handlers/admin_migration.go`

- `AdminMigrateWordPress(c)` — GET `/admin/migrate-wordpress` — displays control panel
- `AdminMigrateWordPressRun(c)` — POST `/admin/migrate-wordpress/run` — triggers migration

### Admin View
**File:** `internal/views/admin/migrate_wordpress.html`

Displays:
- What will be imported (5 posts, 5 pages, media, taxonomy)
- How many records will be created
- A big red button to start the migration
- Post-migration steps (review in Pages section, etc.)

### Router Setup
**File:** `internal/server/router.go` (modified)

Added routes:
```go
admin.GET("/migrate-wordpress", handlers.AdminMigrateWordPress)
admin.POST("/migrate-wordpress/run", handlers.AdminMigrateWordPressRun)
```

### Seeded Admin Menu
**File:** `internal/auth/seed.go` (modified)

Added "WordPress Migration" menu item to the admin sidebar (icon: 📥), positioned after "Layouts".

## How to Use

1. **Access the Migration Panel**
   - Navigate to `/admin/migrate-wordpress` in the admin panel
   - Or click "WordPress Migration" in the sidebar (after deploying/restarting)

2. **Review the Preview**
   - The control panel shows what will be imported
   - Lists how many records will be created

3. **Start Migration**
   - Click "🚀 Start Migration" button
   - You'll be prompted to confirm
   - Migration runs in the request (takes ~5-10 seconds for 10 items + media)
   - Redirects to `/admin/pages` upon success

4. **Review Results**
   - Check the Pages section to see imported posts/pages
   - Featured images link to the WordPress site (can be replaced locally later)
   - Categories and tags are automatically created in the system

## API Responses (WordPress)

### POST Endpoint
```
https://adsqoo.id/wp/wp-json/wp/v2/posts?per_page=5&page=1&_embed
```

Response shape:
- `id`, `date`, `slug`, `status`, `type` — post metadata
- `title.rendered` — HTML post title
- `content.rendered` — HTML post content
- `excerpt.rendered` — HTML excerpt (not used)
- `featured_media` — media ID (0 if none)
- `categories`, `tags` — arrays of term IDs
- `_embedded`:
  - `wp:featuredmedia[0].source_url` — full URL to image
  - `wp:featuredmedia[0].media_details.width/height`
  - `wp:featuredmedia[0].alt_text`
  - `wp:term[][]` — nested array of categories + tags with `name`, `slug`, `taxonomy`
  - `author[0].name` — author display name

### PAGE Endpoint
Same as posts, but for `pages` endpoint (no taxonomy by default in WP).

## Database Schema (No Changes)

Uses existing `Page`, `Media`, `Category`, `Tag` tables:

```sql
-- Page record for a post
INSERT INTO pages (title, slug, type, content, status, author_id, featured_image_id, published_at, created_at, updated_at)
VALUES ('Post Title', 'post-slug', 'post', '<p>HTML content</p>', 'publish', 1, 123, NOW(), NOW(), NOW());

-- Media record for featured image (URL-only)
INSERT INTO media (original_name, path, url, type, mime_type, title, alt_text, created_at, updated_at)
VALUES ('Image Title', 'uploads/2026/08/image.jpg', 'https://adsqoo.id/wp/wp-content/uploads/2026/08/image.jpg', 'image', 'image/jpeg', 'Image Title', 'Alt text', NOW(), NOW());

-- Category record
INSERT INTO categories (name, slug, created_at, updated_at)
VALUES ('Category Name', 'category-slug', NOW(), NOW());

-- Tag record
INSERT INTO tags (name, slug, created_at)
VALUES ('Tag Name', 'tag-slug', NOW());

-- Association: page -> categories (many2many)
INSERT INTO page_categories (page_id, category_id)
VALUES (1, 1);

-- Association: page -> tags (many2many)
INSERT INTO page_tags (page_id, tag_id)
VALUES (1, 1);
```

## Future Phases

### Phase 2: Local Media Storage
When ready, mount WordPress `wp-content` directory via Docker volume:

```dockerfile
# docker-compose.yml
services:
  cms:
    volumes:
      - /path/to/adsqoo/wp-content:/app/wp-content-archive
```

Then update Media records to point to local paths instead of URLs. The Page content (HTML) will remain unchanged; only media URLs need migration.

### Phase 3: Full Migration
Once phase 2 is stable, fetch all posts/pages (paginate through all 1000+ rows) and re-run the migrator with pagination.

## Error Handling

- **Network error:** Returns error in migration, logs to console, migration stops
- **Duplicate slug:** Logs skip message, continues with next item
- **Author not found:** Falls back to `defaultAuthorID` (current user)
- **Missing category/tag:** Log skipped, association not created
- **Featured media error:** Migration continues without featured image

All errors are logged to stdout; the migration doesn't fail hard on individual record failures.

## Testing

### API Reachability
```bash
curl -s "https://adsqoo.id/wp/wp-json/wp/v2/posts?per_page=1&page=1&_embed" | jq '.[] | {id, slug, title, featured_media}'
```

### Code Compilation
```bash
go build -o /tmp/cms-go .
echo "✅ Build successful"
```

## Code Files Created

1. `internal/migrator/wordpress.go` — Core migration logic (250 lines)
2. `internal/handlers/admin_migration.go` — Web handlers (40 lines)
3. `internal/views/admin/migrate_wordpress.html` — Admin UI (80 lines)

## Code Files Modified

1. `internal/server/router.go` — Added 2 routes
2. `internal/auth/seed.go` — Added 1 menu entry

Total new code: ~400 lines
