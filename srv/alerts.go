package srv

import (
	"context"
	"log/slog"
	"regexp"
	"strings"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// compiledAlert holds a news alert with its pre-compiled regex (if applicable).
type compiledAlert struct {
	alert dbgen.NewsAlert
	re    *regexp.Regexp // nil for plain-keyword alerts
}

// EvaluateAlertsForFeed checks unread articles in the given feed against the
// feed owner's alert rules and inserts matches into article_alerts.
func (s *Server) EvaluateAlertsForFeed(ctx context.Context, feedID int64) {
	q := dbgen.New(s.DB)

	// Look up the feed to find its owner.
	userIDPtr, err := q.GetFeedOwner(ctx, feedID)
	if err != nil {
		slog.Warn("alerts: failed to get feed owner", "feed_id", feedID, "error", err)
		return
	}
	if userIDPtr == nil {
		return
	}
	userID := *userIDPtr

	// Load the user's alert rules.
	alerts, err := q.ListAlertsByUser(ctx, userID)
	if err != nil {
		slog.Warn("alerts: failed to list alerts", "user_id", userID, "error", err)
		return
	}
	if len(alerts) == 0 {
		return
	}

	// Pre-compile regex patterns once per batch.
	compiled := make([]compiledAlert, 0, len(alerts))
	for _, a := range alerts {
		ca := compiledAlert{alert: a}
		if a.IsRegex != 0 {
			re, err := regexp.Compile("(?i)" + a.Pattern)
			if err != nil {
				slog.Warn("alerts: bad regex pattern, skipping",
					"alert_id", a.ID, "pattern", a.Pattern, "error", err)
				continue
			}
			ca.re = re
		}
		compiled = append(compiled, ca)
	}
	if len(compiled) == 0 {
		return
	}

	// Fetch unread articles for the feed.
	articles, err := q.ListUnreadArticlesByFeedForAlerts(ctx, dbgen.ListUnreadArticlesByFeedForAlertsParams{
		FeedID: feedID,
		UserID: &userID,
	})
	if err != nil {
		slog.Warn("alerts: failed to list articles", "feed_id", feedID, "error", err)
		return
	}
	if len(articles) == 0 {
		return
	}

	// Match each article against compiled alerts.
	for _, article := range articles {
		title := article.Title
		summary := ptrToStr(article.Summary)
		for i := range compiled {
			if alertMatches(&compiled[i], title, summary) {
				if err := q.InsertArticleAlert(ctx, dbgen.InsertArticleAlertParams{
					ArticleID: article.ID,
					AlertID:   compiled[i].alert.ID,
				}); err != nil {
					slog.Warn("alerts: failed to insert match",
						"article_id", article.ID, "alert_id", compiled[i].alert.ID, "error", err)
					continue
				}
				slog.Info("alerts: matched",
					"alert_id", compiled[i].alert.ID, "alert_name", compiled[i].alert.Name,
					"article_id", article.ID, "article_title", title)
			}
		}
	}
}

// alertMatches checks whether an article's title/summary matches a compiled alert rule.
func alertMatches(ca *compiledAlert, title, summary string) bool {
	matchField := ca.alert.MatchField
	if matchField == "" {
		matchField = "both" // default: match title+summary
	}

	var fields []string
	switch matchField {
	case "title":
		fields = []string{title}
	case "summary":
		fields = []string{summary}
	default: // "both"
		fields = []string{title, summary}
	}

	for _, text := range fields {
		if text == "" {
			continue
		}
		if ca.re != nil {
			if ca.re.MatchString(text) {
				return true
			}
		} else {
			// Case-insensitive substring match.
			if strings.Contains(strings.ToLower(text), strings.ToLower(ca.alert.Pattern)) {
				return true
			}
		}
	}
	return false
}
