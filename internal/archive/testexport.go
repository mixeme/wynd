package archive

import "gitea.mixdep.ru/mix/wynd/internal/chronicle"

// BuildFeedHTMLForTest renders feed layout HTML for tests.
func BuildFeedHTMLForTest(circleName, cutoff string, posts []chronicle.FeedPost) string {
	return renderFeedIndex(circleName, cutoff, posts, nil, nil, nil)
}

// BuildPostsIndexForTest renders the posts-layout index for tests.
func BuildPostsIndexForTest(circleName, cutoff string, posts []chronicle.FeedPost) string {
	return renderPostsIndex(circleName, cutoff, posts, nil)
}
