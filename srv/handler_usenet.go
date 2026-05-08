package srv

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/nntp"
	"github.com/newscientist101/feedreader/srv/usenet"
)

// apiGetUsenetCredentials returns the Usenet credential status for the current
// user. It never returns the encrypted or plaintext password. The response
// includes:
//   - enabled: whether USENET_ENABLED is true
//   - configured: whether the user has saved credentials
//   - username: the stored username, or "" when not configured
//   - key_version: the key version used to encrypt the stored credential, or ""
func (s *Server) apiGetUsenetCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	cred, err := q.GetNNTPCredentials(r.Context(), user.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("usenet credential lookup failed", "error", err, "user_id", user.ID)
			jsonError(w, "Failed to retrieve credential status", 500)
			return
		}
		// No row means not configured; return empty status.
		jsonResponse(w, map[string]any{
			"enabled":     true,
			"configured":  false,
			"username":    "",
			"key_version": "",
		})
		return
	}

	jsonResponse(w, map[string]any{
		"enabled":     true,
		"configured":  true,
		"username":    cred.Username,
		"key_version": cred.KeyVersion,
	})
}

// apiPutUsenetCredentials saves (or replaces) the caller's Usenet credentials.
// Body: {"username": "...", "password": "..."}
// On success, returns the same status shape as GET (without the password).
func (s *Server) apiPutUsenetCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}
	trimmedUsername, err := usenet.ValidateCredentials(body.Username, body.Password)
	if err != nil {
		jsonError(w, err.Error(), 400)
		return
	}

	encrypted, err := s.UsenetConfig.Crypto.Encrypt(body.Password)
	if err != nil {
		slog.Error("usenet encrypt password failed", "error", err)
		jsonError(w, "Failed to encrypt credentials", 500)
		return
	}

	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	cred, err := q.UpsertNNTPCredentials(r.Context(), dbgen.UpsertNNTPCredentialsParams{
		UserID:      user.ID,
		Username:    trimmedUsername,
		PasswordEnc: encrypted,
		KeyVersion:  "v1",
	})
	if err != nil {
		slog.Error("usenet save credentials failed", "error", err, "user_id", user.ID)
		jsonError(w, "Failed to save credentials", 500)
		return
	}

	slog.Info("usenet credentials saved", "user_id", user.ID, "username", cred.Username)
	jsonResponse(w, map[string]any{
		"enabled":     true,
		"configured":  true,
		"username":    cred.Username,
		"key_version": cred.KeyVersion,
	})
}

// apiDeleteUsenetCredentials removes the current user's stored Usenet
// credentials. Returns 200 with {"status":"ok"} whether or not a row existed.
func (s *Server) apiDeleteUsenetCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	if err := q.DeleteNNTPCredentials(r.Context(), user.ID); err != nil {
		slog.Error("usenet delete credentials failed", "error", err, "user_id", user.ID)
		jsonError(w, "Failed to delete credentials", 500)
		return
	}

	slog.Info("usenet credentials deleted", "user_id", user.ID)
	jsonResponse(w, map[string]string{"status": "ok"})
}

// apiGetUsenetGroups returns the list of newsgroups the current user is
// subscribed to. Each entry includes the feed ID, name, group name, provider,
// and high_water_article_number (the highest article number fetched so far).
func (s *Server) apiGetUsenetGroups(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	usenetFeeds, err := q.ListUsenetFeeds(r.Context(), &user.ID)
	if err != nil {
		slog.Error("usenet list groups failed", "error", err, "user_id", user.ID)
		jsonError(w, "Failed to list newsgroups", 500)
		return
	}

	type groupItem struct {
		FeedID                 int64  `json:"feed_id"`
		Name                   string `json:"name"`
		GroupName              string `json:"group_name"`
		Provider               string `json:"provider"`
		HighWaterArticleNumber int64  `json:"high_water_article_number"`
	}
	items := make([]groupItem, 0, len(usenetFeeds))
	for i := range usenetFeeds {
		f := &usenetFeeds[i]
		items = append(items, groupItem{
			FeedID:                 f.ID,
			Name:                   f.Name,
			GroupName:              f.GroupName,
			Provider:               f.Provider,
			HighWaterArticleNumber: f.HighWaterArticleNumber,
		})
	}
	jsonResponse(w, items)
}

// apiPostUsenetGroups subscribes the current user to a newsgroup.
// Body: {"group_name": "...", "category_id": 0}
// Requires Usenet to be enabled and credentials to be configured.
func (s *Server) apiPostUsenetGroups(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	var body struct {
		GroupName  string `json:"group_name"`
		CategoryID int64  `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}

	normName, err := usenet.ValidateGroupName(body.GroupName)
	if err != nil {
		jsonError(w, "Invalid newsgroup name: "+err.Error(), 400)
		return
	}

	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	// Require credentials to be configured before adding a group.
	if _, err := q.GetNNTPCredentials(ctx, user.ID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.Error("usenet credential lookup failed", "error", err, "user_id", user.ID)
			jsonError(w, "Failed to retrieve credential status", 500)
			return
		}
		jsonError(w, "Usenet credentials must be configured before adding a newsgroup", 400)
		return
	}

	// Reject duplicate subscriptions.
	if _, err := q.GetUsenetFeedStateByGroup(ctx, dbgen.GetUsenetFeedStateByGroupParams{
		Provider:  nntp.ProviderName,
		GroupName: normName,
		UserID:    &user.ID,
	}); err == nil {
		jsonError(w, "Already subscribed to "+normName, 409)
		return
	}

	// Construct the canonical feed URL.
	feedURL := "nntp://" + nntp.EternalSeptemberHost + "/" + normName

	feed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name:     normName,
		Url:      feedURL,
		FeedType: "nntp",
		UserID:   &user.ID,
	})
	if err != nil {
		slog.Error("usenet create feed failed", "error", err, "user_id", user.ID, "group", normName)
		jsonError(w, "Failed to create newsgroup feed", 500)
		return
	}

	_, err = q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID:    feed.ID,
		Provider:  nntp.ProviderName,
		GroupName: normName,
	})
	if err != nil {
		// Roll back the feed row if state creation fails.
		_ = q.DeleteFeed(ctx, dbgen.DeleteFeedParams{ID: feed.ID, UserID: &user.ID})
		slog.Error("usenet create feed state failed", "error", err, "user_id", user.ID, "group", normName)
		jsonError(w, "Failed to initialise newsgroup state", 500)
		return
	}

	if body.CategoryID > 0 {
		// Verify the category belongs to the current user before linking.
		if _, err := q.GetCategory(ctx, dbgen.GetCategoryParams{
			ID:     body.CategoryID,
			UserID: &user.ID,
		}); err != nil {
			// Roll back the feed and state rows.
			_ = q.DeleteFeed(ctx, dbgen.DeleteFeedParams{ID: feed.ID, UserID: &user.ID})
			jsonError(w, "Invalid or inaccessible category_id", 400)
			return
		}
		if err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{
			FeedID:     feed.ID,
			CategoryID: body.CategoryID,
		}); err != nil {
			slog.Warn("usenet set feed category failed", "error", err, "feed_id", feed.ID)
		}
	}

	slog.Info("usenet group added", "user_id", user.ID, "group", normName, "feed_id", feed.ID)
	jsonResponse(w, map[string]any{
		"feed_id":    feed.ID,
		"name":       feed.Name,
		"group_name": normName,
		"provider":   nntp.ProviderName,
	})
}

// apiDeleteUsenetGroup removes a newsgroup subscription for the current user.
// This deletes the feed row; cascade-deletes handle the usenet_feed_state row.
func (s *Server) apiDeleteUsenetGroup(w http.ResponseWriter, r *http.Request) {
	if !s.UsenetConfig.Enabled {
		jsonError(w, "Usenet is not enabled on this server", 503)
		return
	}

	feedID, err := strconv.ParseInt(r.PathValue("feed_id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	// Verify the feed belongs to this user and is an NNTP feed.
	// GetUsenetFeedState already joins feeds to scope by user.
	if _, err := q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: feedID,
		UserID: &user.ID,
	}); err != nil {
		jsonError(w, "Newsgroup not found", 404)
		return
	}

	if err := q.DeleteFeed(ctx, dbgen.DeleteFeedParams{ID: feedID, UserID: &user.ID}); err != nil {
		slog.Error("usenet delete feed failed", "error", err, "user_id", user.ID, "feed_id", feedID)
		jsonError(w, "Failed to remove newsgroup", 500)
		return
	}

	s.CountsCache.Invalidate(user.ID)
	slog.Info("usenet group removed", "user_id", user.ID, "feed_id", feedID)
	jsonResponse(w, map[string]string{"status": "ok"})
}
