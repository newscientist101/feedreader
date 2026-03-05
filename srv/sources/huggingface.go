package sources

import (
	"context"
	"encoding/json"

	"github.com/newscientist101/feedreader/srv/huggingface"
)

// HuggingFaceSource handles HuggingFace config-based feeds.
type HuggingFaceSource struct{}

func (HuggingFaceSource) Match(_, feedType string) bool {
	return feedType == "huggingface"
}

func (HuggingFaceSource) NormalizeURL(_ context.Context, rawURL string) (string, error) {
	return rawURL, nil
}

func (HuggingFaceSource) ResolveName(ctx context.Context, _, scraperConfig string) string {
	if scraperConfig == "" {
		return ""
	}
	var hfConfig huggingface.FeedConfig
	if err := json.Unmarshal([]byte(scraperConfig), &hfConfig); err != nil {
		return ""
	}
	hfClient := huggingface.NewClient("")
	name, err := hfClient.GetFeedName(ctx, &hfConfig)
	if err != nil {
		return ""
	}
	return name
}

func (HuggingFaceSource) FeedType() string { return "" }
