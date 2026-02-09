package timeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gh-pr/internal/github"
	"gh-pr/internal/timelineapi"
)

var knownTimelineEvents = map[string]struct{}{
	"assigned": {}, "unassigned": {}, "labeled": {}, "unlabeled": {},
	"milestoned": {}, "demilestoned": {}, "renamed": {}, "review_requested": {},
	"review_request_removed": {}, "reviewed": {}, "commented": {}, "line-commented": {},
	"committed": {}, "cross-referenced": {}, "closed": {}, "reopened": {},
	"merged": {}, "head_ref_deleted": {}, "head_ref_restored": {}, "head_ref_force_pushed": {},
	"base_ref_changed": {}, "referenced": {}, "ready_for_review": {}, "converted_to_draft": {},
	"locked": {}, "unlocked": {}, "mentioned": {}, "subscribed": {},
	"unsubscribed": {}, "pinned": {}, "unpinned": {}, "deployed": {},
	"deployment_environment_changed": {}, "auto_merge_enabled": {}, "auto_merge_disabled": {},
	"auto_squash_enabled": {}, "auto_squash_disabled": {},
	"review_dismissed": {}, "connected": {}, "disconnected": {}, "transferred": {},
}

var ignoredPRTimelineEvents = map[string]struct{}{
	"cross-referenced":    {},
	"head_ref_deleted":    {},
	"labeled":             {},
	"mentioned":           {},
	"subscribed":          {},
	"auto_squash_enabled": {},
}

func Build(pr github.PullRequest, rawItems []github.TimelineItem) ([]timelineapi.Event, []string) {
	return BuildWithComments(pr, rawItems, nil)
}

func BuildWithComments(pr github.PullRequest, rawItems []github.TimelineItem, reviewComments []github.ReviewComment) ([]timelineapi.Event, []string) {
	warnings := make([]string, 0)
	events := make([]timelineapi.Event, 0, len(rawItems)+len(reviewComments)+1)

	opened := OpenedEvent(pr)

	for _, item := range rawItems {
		mapped, warning, ok := MapTimelineItem(item.Raw)
		if warning != "" {
			warnings = append(warnings, warning)
		}
		if ok {
			if ShouldIgnorePRTimelineEvent(mapped) {
				continue
			}
			events = append(events, mapped)
		}
	}

	for _, comment := range reviewComments {
		events = append(events, MapReviewComment(comment))
	}

	sortEvents(events)
	return append([]timelineapi.Event{opened}, events...), warnings
}

func OpenedEvent(pr github.PullRequest) timelineapi.Event {
	return timelineapi.Event{
		Type:       "pr.opened",
		Id:         syntheticOpenedID(pr),
		OccurredAt: pr.CreatedAt,
		Actor: &timelineapi.Actor{
			Login: pr.User.Login,
			Id:    int(pr.User.ID),
		},
		Pr: &timelineapi.PROpenedData{
			Title: pr.Title,
			Body:  pr.Body,
		},
	}
}

func OpenedIssueEvent(issue github.Issue) timelineapi.Event {
	return timelineapi.Event{
		Type:       "issue.opened",
		Id:         syntheticIssueOpenedID(issue),
		OccurredAt: issue.CreatedAt,
		Actor: &timelineapi.Actor{
			Login: issue.User.Login,
			Id:    int(issue.User.ID),
		},
		Issue: &timelineapi.IssueOpenedData{
			Title: issue.Title,
			Body:  issue.Body,
		},
	}
}

func MapTimelineItem(raw json.RawMessage) (timelineapi.Event, string, bool) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return timelineapi.Event{}, fmt.Sprintf("warning: skipping timeline event; invalid JSON payload: %v", err), false
	}

	eventName, _ := obj["event"].(string)
	if eventName == "" {
		return timelineapi.Event{}, "warning: skipping timeline event; missing type", false
	}

	if _, ok := knownTimelineEvents[eventName]; !ok {
		return timelineapi.Event{}, fmt.Sprintf("warning: skipping unknown timeline event type=%q id=%s occurred_at=%s", eventName, asString(obj["id"]), occurredAtString(obj)), false
	}

	eventID := firstNonEmpty(asString(obj["id"]), asString(obj["node_id"]))
	if eventName == "head_ref_force_pushed" {
		eventID = firstNonEmpty(asString(obj["node_id"]), asString(obj["id"]))
	}
	if eventID == "" {
		eventID = "ghf_" + stableHash("timeline", eventName, canonicalizeRawJSON(raw))
	}

	mapped := timelineapi.Event{
		Type:       "github.timeline." + eventName,
		Event:      ptrString(eventName),
		OccurredAt: occurredAt(obj),
		Id:         eventID,
	}

	if eventName == "committed" {
		sha := firstNonEmpty(asString(obj["sha"]), asString(obj["commit_id"]))
		htmlURL := firstNonEmpty(asString(obj["html_url"]), asString(obj["url"]))
		apiURL := asString(obj["commit_url"])
		url := firstNonEmpty(htmlURL, apiURL)
		diffURL := commitDiffURL(firstNonEmpty(apiURL, url))
		mapped.Commit = &timelineapi.CommitContext{Sha: ptrString(sha), Url: ptrString(url)}
		mapped.DiffUrl = ptrString(diffURL)
	}

	if eventName == "commented" || eventName == "line-commented" {
		mapped.Comment = mapTimelineCommentContext(obj)
	}
	if eventName == "review_requested" {
		if requested := reviewRequestedTarget(obj); requested != "" {
			mapped.Comment = &timelineapi.CommentContext{Body: ptrString(requested)}
		}
	}

	if actor := extractActor(obj); actor != nil {
		mapped.Actor = actor
	}

	return mapped, "", true
}

func MapReviewComment(comment github.ReviewComment) timelineapi.Event {
	when := time.Unix(0, 0).UTC()
	if comment.CreatedAt != nil {
		when = comment.CreatedAt.UTC()
	} else if comment.UpdatedAt != nil {
		when = comment.UpdatedAt.UTC()
	}

	id := ""
	if comment.ID > 0 {
		id = fmt.Sprintf("%d", comment.ID)
	}

	if id == "" {
		id = firstNonEmpty(comment.NodeID, "ghrc_"+stableHash(
			"review_comment",
			comment.Path,
			firstNonEmpty(comment.CommitID, comment.OriginalCommitID),
			comment.User.Login,
			fmt.Sprintf("%d", comment.User.ID),
			timeOrZero(comment.CreatedAt),
			timeOrZero(comment.UpdatedAt),
		))
	}

	e := timelineapi.Event{
		Type:       "github.review_comment",
		Event:      ptrString("review_comment"),
		OccurredAt: when,
		Id:         firstNonEmpty(id, comment.NodeID),
		Actor: &timelineapi.Actor{
			Login: comment.User.Login,
			Id:    int(comment.User.ID),
		},
		Comment: &timelineapi.CommentContext{
			Path:             ptrString(comment.Path),
			Body:             ptrString(comment.Body),
			DiffHunk:         ptrString(comment.DiffHunk),
			Position:         comment.Position,
			OriginalPosition: comment.OriginalPosition,
			Line:             comment.Line,
			StartLine:        comment.StartLine,
			ReviewId:         ptrInt(int(comment.PullRequestReview)),
			ThreadId:         ptrString(reviewCommentThreadID(comment)),
			Url:              ptrString(comment.HTMLURL),
		},
		Commit: &timelineapi.CommitContext{
			Sha: ptrString(comment.CommitID),
			Url: ptrString(comment.HTMLURL),
		},
		DiffUrl: ptrString(comment.HTMLURL),
	}

	return e
}

func ShouldIgnorePRTimelineEvent(event timelineapi.Event) bool {
	if event.Event == nil {
		return false
	}
	_, ok := ignoredPRTimelineEvents[*event.Event]
	return ok
}

func reviewCommentThreadID(comment github.ReviewComment) string {
	if comment.InReplyToID != nil && *comment.InReplyToID > 0 {
		return fmt.Sprintf("%d", *comment.InReplyToID)
	}
	if comment.ID > 0 {
		return fmt.Sprintf("%d", comment.ID)
	}
	if comment.NodeID != "" {
		return comment.NodeID
	}

	return "ghrt_" + stableHash(
		"review_thread",
		comment.Path,
		firstNonEmpty(comment.CommitID, comment.OriginalCommitID),
		comment.User.Login,
		fmt.Sprintf("%d", comment.User.ID),
		timeOrZero(comment.CreatedAt),
		timeOrZero(comment.UpdatedAt),
	)
}

func syntheticOpenedID(pr github.PullRequest) string {
	return "ghpr_opened_" + stableHash(
		"pr_opened",
		fmt.Sprintf("%d", pr.Number),
		pr.CreatedAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%d", pr.User.ID),
		pr.User.Login,
	)
}

func syntheticIssueOpenedID(issue github.Issue) string {
	return "ghissue_opened_" + stableHash(
		"issue_opened",
		fmt.Sprintf("%d", issue.Number),
		issue.CreatedAt.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%d", issue.User.ID),
		issue.User.Login,
	)
}

func extractActor(obj map[string]any) *timelineapi.Actor {
	for _, key := range []string{"actor", "user", "author"} {
		rawUser, ok := obj[key].(map[string]any)
		if !ok {
			continue
		}

		login, _ := rawUser["login"].(string)
		id := asInt64(rawUser["id"])
		if login == "" && id == 0 {
			continue
		}

		return &timelineapi.Actor{Login: login, Id: int(id)}
	}

	return nil
}

func occurredAt(obj map[string]any) time.Time {
	for _, key := range []string{"created_at", "submitted_at", "updated_at"} {
		raw, _ := obj[key].(string)
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err == nil {
			return t.UTC()
		}
	}

	if author, ok := obj["author"].(map[string]any); ok {
		if raw, _ := author["date"].(string); raw != "" {
			if t, err := time.Parse(time.RFC3339, raw); err == nil {
				return t.UTC()
			}
		}
	}

	if commit, ok := obj["commit"].(map[string]any); ok {
		if author, ok := commit["author"].(map[string]any); ok {
			if raw, _ := author["date"].(string); raw != "" {
				if t, err := time.Parse(time.RFC3339, raw); err == nil {
					return t.UTC()
				}
			}
		}
	}

	return time.Unix(0, 0).UTC()
}

func occurredAtString(obj map[string]any) string {
	t := occurredAt(obj)
	if t.IsZero() || t.Unix() == 0 {
		return ""
	}
	return t.Format(time.RFC3339)
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		return ""
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if item != "" {
			return item
		}
	}
	return ""
}

func commitDiffURL(url string) string {
	if url == "" {
		return ""
	}
	if strings.Contains(url, "api.github.com/repos/") && strings.Contains(url, "/commits/") {
		return url
	}
	if strings.Contains(url, "github.com/") && strings.Contains(url, "/commit/") {
		u := strings.TrimPrefix(url, "https://github.com/")
		u = strings.TrimPrefix(u, "http://github.com/")
		u = strings.TrimSuffix(u, ".diff")
		parts := strings.Split(u, "/")
		if len(parts) >= 4 && parts[2] == "commit" {
			owner, repo, sha := parts[0], parts[1], parts[3]
			if owner != "" && repo != "" && sha != "" {
				return "https://api.github.com/repos/" + owner + "/" + repo + "/commits/" + sha
			}
		}
		return url
	}
	return url
}

func stableHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:16])
}

func canonicalizeRawJSON(raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return canonicalizeValue(value)
}

func canonicalizeValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		b, _ := json.Marshal(t)
		return string(b)
	case float64:
		return fmt.Sprintf("%.17g", t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, canonicalizeValue(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		keys := make([]string, 0, len(t))
		for key := range t {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			keyJSON, _ := json.Marshal(key)
			parts = append(parts, string(keyJSON)+":"+canonicalizeValue(t[key]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func timeOrZero(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func ptrString(s string) *string {
	return &s
}

func ptrInt(v int) *int {
	return &v
}

func mapTimelineCommentContext(obj map[string]any) *timelineapi.CommentContext {
	ctx := &timelineapi.CommentContext{
		Path:             optionalString(firstNonEmpty(asString(obj["path"]), asString(obj["file_path"]))),
		Body:             optionalString(asString(obj["body"])),
		DiffHunk:         optionalString(asString(obj["diff_hunk"])),
		Position:         optionalInt(obj["position"]),
		OriginalPosition: optionalInt(obj["original_position"]),
		Line:             optionalInt(obj["line"]),
		StartLine:        optionalInt(obj["start_line"]),
		ReviewId:         optionalIntFromInt64(asInt64(obj["pull_request_review_id"])),
		Url:              optionalString(firstNonEmpty(asString(obj["html_url"]), asString(obj["url"]))),
	}

	if ctx.Path == nil &&
		ctx.Body == nil &&
		ctx.DiffHunk == nil &&
		ctx.Position == nil &&
		ctx.OriginalPosition == nil &&
		ctx.Line == nil &&
		ctx.StartLine == nil &&
		ctx.ReviewId == nil &&
		ctx.Url == nil {
		return nil
	}

	return ctx
}

func reviewRequestedTarget(obj map[string]any) string {
	if requestedReviewer, ok := obj["requested_reviewer"].(map[string]any); ok {
		login := asString(requestedReviewer["login"])
		if login != "" {
			return "@" + login
		}
	}
	if requestedTeam, ok := obj["requested_team"].(map[string]any); ok {
		slug := asString(requestedTeam["slug"])
		if slug != "" {
			return "@" + slug
		}
		name := asString(requestedTeam["name"])
		if name != "" {
			return name
		}
	}
	return ""
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optionalInt(v any) *int {
	n := asInt64(v)
	if n == 0 {
		return nil
	}
	i := int(n)
	return &i
}

func optionalIntFromInt64(v int64) *int {
	if v == 0 {
		return nil
	}
	i := int(v)
	return &i
}
