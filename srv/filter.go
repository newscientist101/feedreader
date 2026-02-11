package srv

import (
	"context"
	"regexp"
	"strings"

	"srv.exe.dev/db/dbgen"
)

// FilterArticles applies exclusion rules to a list of articles
func (s *Server) FilterArticles(ctx context.Context, articles []dbgen.ListUnreadArticlesRow, categoryID int64, userID int64) []dbgen.ListUnreadArticlesRow {
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
	for _, article := range articles {
		if !s.shouldExclude(article.Title, ptrToStr(article.Summary), ptrToStr(article.Author), exclusions) {
			filtered = append(filtered, article)
		}
	}
	return filtered
}

// FilterArticlesByFeed applies exclusion rules for feed articles
func (s *Server) FilterArticlesByFeed(ctx context.Context, articles []dbgen.ListArticlesByFeedRow, feedID int64, userID int64) []dbgen.ListArticlesByFeedRow {
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
	for _, article := range articles {
		if !s.shouldExclude(article.Title, ptrToStr(article.Summary), ptrToStr(article.Author), exclusions) {
			filtered = append(filtered, article)
		}
	}
	return filtered
}

// FilterArticlesByCategory applies exclusion rules for category articles
func (s *Server) FilterArticlesByCategory(ctx context.Context, articles []dbgen.ListUnreadArticlesByCategoryRow, categoryID int64, userID int64) []dbgen.ListUnreadArticlesByCategoryRow {
	q := dbgen.New(s.DB)
	exclusions, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: categoryID,
		UserID:     &userID,
	})
	if err != nil || len(exclusions) == 0 {
		return articles
	}

	var filtered []dbgen.ListUnreadArticlesByCategoryRow
	for _, article := range articles {
		if !s.shouldExclude(article.Title, ptrToStr(article.Summary), ptrToStr(article.Author), exclusions) {
			filtered = append(filtered, article)
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
