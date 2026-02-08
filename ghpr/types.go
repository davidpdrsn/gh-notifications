package ghpr

import "time"

type Actor struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

type PROpenedData struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type IssueOpenedData struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type CommentContext struct {
	Path             *string `json:"path,omitempty"`
	Body             *string `json:"body,omitempty"`
	DiffHunk         *string `json:"diff_hunk,omitempty"`
	Position         *int    `json:"position,omitempty"`
	OriginalPosition *int    `json:"original_position,omitempty"`
	Line             *int    `json:"line,omitempty"`
	StartLine        *int    `json:"start_line,omitempty"`
	ReviewID         *int    `json:"review_id,omitempty"`
	ThreadID         *string `json:"thread_id,omitempty"`
	URL              *string `json:"url,omitempty"`
}

type CommitContext struct {
	SHA *string `json:"sha,omitempty"`
	URL *string `json:"url,omitempty"`
}

type TimelineEvent struct {
	Type       string           `json:"type"`
	OccurredAt time.Time        `json:"occurred_at"`
	ID         string           `json:"id"`
	Actor      *Actor           `json:"actor,omitempty"`
	Event      *string          `json:"event,omitempty"`
	Pr         *PROpenedData    `json:"pr,omitempty"`
	Issue      *IssueOpenedData `json:"issue,omitempty"`
	Comment    *CommentContext  `json:"comment,omitempty"`
	Commit     *CommitContext   `json:"commit,omitempty"`
	DiffURL    *string          `json:"diff_url,omitempty"`
}

type NotificationRepository struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type NotificationSubject struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type NotificationTarget struct {
	Kind   string `json:"kind"`
	Number int    `json:"number"`
	Ref    string `json:"ref"`
}

type NotificationEvent struct {
	ID         string                 `json:"id"`
	Repository NotificationRepository `json:"repository"`
	Subject    NotificationSubject    `json:"subject"`
	Target     NotificationTarget     `json:"target"`
}
