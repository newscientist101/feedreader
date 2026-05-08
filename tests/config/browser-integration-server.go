package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv"
)

const (
	externalID = "browser-integ-user"
	email      = "browser-integ@example.com"
	feedURL    = "https://example.com/browser-integration-feed.xml"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <db-path>", filepath.Base(os.Args[0]))
	}

	s, err := srv.New(os.Args[1], "browser-integration.test")
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	if err := seed(s); err != nil {
		log.Fatal(err)
	}

	if err := s.Serve(":3200"); err != nil {
		log.Fatal(err)
	}
}

func seed(s *srv.Server) error {
	ctx := context.Background()
	q := dbgen.New(s.DB)

	user, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{ExternalID: externalID, Email: email})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	settings := map[string]string{
		"autoMarkRead":     "true",
		"hideReadArticles": "hide",
		"defaultView":      "expanded",
	}
	for key, value := range settings {
		if err := q.SetUserSetting(ctx, dbgen.SetUserSettingParams{UserID: user.ID, Key: key, Value: value}); err != nil {
			return fmt.Errorf("set setting %s: %w", key, err)
		}
	}

	interval := int64(60)
	feed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name:                 "Browser Integration Feed",
		Url:                  feedURL,
		FeedType:             "rss",
		FetchIntervalMinutes: &interval,
		UserID:               &user.ID,
	})
	if err != nil {
		return fmt.Errorf("create feed: %w", err)
	}

	base := time.Now().UTC().Truncate(time.Second)
	for i := 1; i <= 12; i++ {
		published := base.Add(-time.Duration(i) * time.Minute)
		content := fmt.Sprintf("Browser integration article %02d content", i)
		if _, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
			FeedID:      feed.ID,
			Guid:        fmt.Sprintf("browser-integration-%02d", i),
			Title:       fmt.Sprintf("Browser Integration Article %02d", i),
			Content:     &content,
			PublishedAt: &published,
		}); err != nil {
			return fmt.Errorf("create article %d: %w", i, err)
		}
	}

	return nil
}
