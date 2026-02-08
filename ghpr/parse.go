package ghpr

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type TimelineRef struct {
	Owner    string
	Repo     string
	Number   int
	KindHint string
}

func ParseTimelineRef(input string) (TimelineRef, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if ref, err := parsePRURL(input); err == nil {
			return TimelineRef{Owner: ref.Owner, Repo: ref.Repo, Number: ref.Number, KindHint: "pr"}, nil
		}
		if ref, err := parseIssueURL(input); err == nil {
			return TimelineRef{Owner: ref.Owner, Repo: ref.Repo, Number: ref.Number, KindHint: "issue"}, nil
		}
	}

	hashIndex := strings.LastIndex(input, "#")
	if hashIndex <= 0 || hashIndex == len(input)-1 {
		return TimelineRef{}, fmt.Errorf("invalid timeline reference: %q", input)
	}

	repoRef := input[:hashIndex]
	numberPart := input[hashIndex+1:]

	slashIndex := strings.Index(repoRef, "/")
	if slashIndex <= 0 || slashIndex == len(repoRef)-1 || strings.Count(repoRef, "/") != 1 {
		return TimelineRef{}, fmt.Errorf("invalid repository reference: %q", repoRef)
	}

	number, err := strconv.Atoi(numberPart)
	if err != nil || number <= 0 {
		return TimelineRef{}, fmt.Errorf("invalid issue or pull request number: %q", numberPart)
	}

	return TimelineRef{Owner: repoRef[:slashIndex], Repo: repoRef[slashIndex+1:], Number: number}, nil
}

type prRef struct {
	Owner  string
	Repo   string
	Number int
}

type issueRef struct {
	Owner  string
	Repo   string
	Number int
}

func parsePRURL(input string) (prRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return prRef{}, fmt.Errorf("invalid pull request URL: %q", input)
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return prRef{}, fmt.Errorf("unsupported pull request URL host: %q", u.Hostname())
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return prRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	if parts[2] != "pull" && parts[2] != "pulls" {
		return prRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return prRef{}, fmt.Errorf("invalid pull request number in URL: %q", parts[3])
	}

	if parts[0] == "" || parts[1] == "" {
		return prRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	return prRef{Owner: parts[0], Repo: parts[1], Number: number}, nil
}

func parseIssueURL(input string) (issueRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return issueRef{}, fmt.Errorf("invalid issue URL: %q", input)
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return issueRef{}, fmt.Errorf("unsupported issue URL host: %q", u.Hostname())
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return issueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	if parts[2] != "issue" && parts[2] != "issues" {
		return issueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return issueRef{}, fmt.Errorf("invalid issue number in URL: %q", parts[3])
	}

	if parts[0] == "" || parts[1] == "" {
		return issueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	return issueRef{Owner: parts[0], Repo: parts[1], Number: number}, nil
}
