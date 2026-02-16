package srv

import (
	"context"
	"regexp"
	"strings"

	"srv.exe.dev/db/dbgen"
)

// FilterArticles applies exclusion rules to a list of articles
func (s *Server) FilterArticles(ctx context.Context, articles []dbgen.ListUnreadArticlesRow, categoryID, userID int64) []dbgen.ListUnreadArticlesRow {
	if categoryID == 0 {
		return articles
	}

	q := dbgen.New(s.DB)
	exclusions, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: categoryID,
		UserID:     &userID,
	})
	if err != nil || len(exclusions) == 0 {
		return articles
	}

	var filtered []dbgen.ListUnreadArticlesRow
	for i := range articles {
		if !s.shouldExclude(articles[i].Title, ptrToStr(articles[i].Summary), ptrToStr(articles[i].Author), exclusions) {
			filtered = append(filtered, articles[i])
		}
	}
	return filtered
}

// FilterAllUnreadArticles applies all folder exclusion rules across all categories
// to a global list of unread articles.
func (s *Server) FilterAllUnreadArticles(ctx context.Context, articles []dbgen.ListUnreadArticlesRow, userID int64) []dbgen.ListUnreadArticlesRow {
	q := dbgen.New(s.DB)

	allExclusions, err := q.ListAllExclusions(ctx, &userID)
	if err != nil || len(allExclusions) == 0 {
		return articles
	}

	// Group exclusions by category_id
	exclusionsByCategory := make(map[int64][]dbgen.CategoryExclusion)
	for _, e := range allExclusions {
		exclusionsByCategory[e.CategoryID] = append(exclusionsByCategory[e.CategoryID], dbgen.CategoryExclusion{
			ID:            e.ID,
			CategoryID:    e.CategoryID,
			ExclusionType: e.ExclusionType,
			Pattern:       e.Pattern,
			IsRegex:       e.IsRegex,
			CreatedAt:     e.CreatedAt,
		})
	}

	// Build feed_id → category_ids map
	mappings, err := q.ListFeedCategoryMappings(ctx, &userID)
	if err != nil {
		return articles
	}
	feedCategories := make(map[int64][]int64)
	for _, m := range mappings {
		feedCategories[m.FeedID] = append(feedCategories[m.FeedID], m.CategoryID)
	}

	var filtered []dbgen.ListUnreadArticlesRow
	for i := range articles {
		excluded := false
		for _, catID := range feedCategories[articles[i].FeedID] {
			if excls, ok := exclusionsByCategory[catID]; ok {
				if s.shouldExclude(articles[i].Title, ptrToStr(articles[i].Summary), ptrToStr(articles[i].Author), excls) {
					excluded = true
					break
				}
			}
		}
		if !excluded {
			filtered = append(filtered, articles[i])
		}
	}
	return filtered
}

// FilterArticlesByFeed applies exclusion rules for feed articles
func (s *Server) FilterArticlesByFeed(ctx context.Context, articles []dbgen.ListArticlesByFeedRow, feedID, userID int64) []dbgen.ListArticlesByFeedRow {
	q := dbgen.New(s.DB)

	// Get the category for this feed
	cats, err := q.GetFeedCategories(ctx, feedID)
	if err != nil || len(cats) == 0 {
		return articles
	}
	categoryID := cats[0].ID

	exclusions, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: categoryID,
		UserID:     &userID,
	})
	if err != nil || len(exclusions) == 0 {
		return articles
	}

	var filtered []dbgen.ListArticlesByFeedRow
	for i := range articles {
		if !s.shouldExclude(articles[i].Title, ptrToStr(articles[i].Summary), ptrToStr(articles[i].Author), exclusions) {
			filtered = append(filtered, articles[i])
		}
	}
	return filtered
}

// FilterArticlesByCategory applies exclusion rules for category articles
func (s *Server) FilterArticlesByCategory(ctx context.Context, articles []dbgen.ListUnreadArticlesByCategoryRow, categoryID, userID int64) []dbgen.ListUnreadArticlesByCategoryRow {
	q := dbgen.New(s.DB)
	exclusions, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: categoryID,
		UserID:     &userID,
	})
	if err != nil || len(exclusions) == 0 {
		return articles
	}

	var filtered []dbgen.ListUnreadArticlesByCategoryRow
	for i := range articles {
		if !s.shouldExclude(articles[i].Title, ptrToStr(articles[i].Summary), ptrToStr(articles[i].Author), exclusions) {
			filtered = append(filtered, articles[i])
		}
	}
	return filtered
}

func (s *Server) shouldExclude(title, summary, author string, exclusions []dbgen.CategoryExclusion) bool {
	for _, excl := range exclusions {
		switch excl.ExclusionType {
		case "author":
			if matchesPattern(author, excl.Pattern, excl.IsRegex != nil && *excl.IsRegex == 1) {
				return true
			}
		case "keyword":
			// Check both title and summary
			if matchesPattern(title, excl.Pattern, excl.IsRegex != nil && *excl.IsRegex == 1) {
				return true
			}
			if matchesPattern(summary, excl.Pattern, excl.IsRegex != nil && *excl.IsRegex == 1) {
				return true
			}
		}
	}
	return false
}

func matchesPattern(text, pattern string, isRegex bool) bool {
	if text == "" || pattern == "" {
		return false
	}

	if isRegex {
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}

	// Case-insensitive substring match
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}

func ptrToStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
