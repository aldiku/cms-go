# Migrator Post WP

## Post
get url paginated: https://adsqoo.id/wp/wp-json/wp/v2/posts?per_page=10&page=1&_embed
get by slug : https://adsqoo.id/wp/wp-json/wp/v2/posts?slug={slug}&_embed

## Page 
get url paginated: https://adsqoo.id/wp/wp-json/wp/v2/posts?per_page=10&page=1&_embed
get page by slug: https://adsqoo.id/wp/wp-json/wp/v2/pages?slug={slug}&_embed

# Task 
1. for page and post above try clone 5 page and 5 post for now for testing.
2. create golang function to clone it, we will run full clone after this testing is ok
3. save to pages database and set the type page for pages and post for post, all wil be use html editor
4. for the thumbnail /featured image try to create the media on our media db but using url instead of cloning the image, keep the url from adsqoo, we just save media as url , on next phase i will mount the wp-content to our assets folder via docker volume mounting, for now just save the media as url path