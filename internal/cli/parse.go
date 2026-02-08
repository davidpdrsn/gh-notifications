package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

type IssueRef struct {
	Owner  string
	Repo   string
	Number int
}

type TimelineRef struct {
	Owner    string
	Repo     string
	Number   int
	KindHint string
}

func parsePRRef(input string) (PRRef, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if ref, err := parsePRURL(input); err == nil {
			return ref, nil
		}
	}

	hashIndex := strings.LastIndex(input, "#")
	if hashIndex <= 0 || hashIndex == len(input)-1 {
		return PRRef{}, fmt.Errorf("invalid pull request reference: %q", input)
	}

	repoRef := input[:hashIndex]
	numberPart := input[hashIndex+1:]

	slashIndex := strings.Index(repoRef, "/")
	if slashIndex <= 0 || slashIndex == len(repoRef)-1 || strings.Count(repoRef, "/") != 1 {
		return PRRef{}, fmt.Errorf("invalid repository reference: %q", repoRef)
	}

	number, err := strconv.Atoi(numberPart)
	if err != nil || number <= 0 {
		return PRRef{}, fmt.Errorf("invalid pull request number: %q", numberPart)
	}

	return PRRef{
		Owner:  repoRef[:slashIndex],
		Repo:   repoRef[slashIndex+1:],
		Number: number,
	}, nil
}

func parsePRURL(input string) (PRRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return PRRef{}, fmt.Errorf("invalid pull request URL: %q", input)
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return PRRef{}, fmt.Errorf("unsupported pull request URL host: %q", u.Hostname())
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return PRRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	if parts[2] != "pull" && parts[2] != "pulls" {
		return PRRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return PRRef{}, fmt.Errorf("invalid pull request number in URL: %q", parts[3])
	}

	owner := parts[0]
	repo := parts[1]
	if owner == "" || repo == "" {
		return PRRef{}, fmt.Errorf("invalid pull request URL path: %q", u.Path)
	}

	return PRRef{Owner: owner, Repo: repo, Number: number}, nil
}

func parseIssueRef(input string) (IssueRef, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if ref, err := parseIssueURL(input); err == nil {
			return ref, nil
		}
	}

	hashIndex := strings.LastIndex(input, "#")
	if hashIndex <= 0 || hashIndex == len(input)-1 {
		return IssueRef{}, fmt.Errorf("invalid issue reference: %q", input)
	}

	repoRef := input[:hashIndex]
	numberPart := input[hashIndex+1:]

	slashIndex := strings.Index(repoRef, "/")
	if slashIndex <= 0 || slashIndex == len(repoRef)-1 || strings.Count(repoRef, "/") != 1 {
		return IssueRef{}, fmt.Errorf("invalid repository reference: %q", repoRef)
	}

	number, err := strconv.Atoi(numberPart)
	if err != nil || number <= 0 {
		return IssueRef{}, fmt.Errorf("invalid issue number: %q", numberPart)
	}

	return IssueRef{
		Owner:  repoRef[:slashIndex],
		Repo:   repoRef[slashIndex+1:],
		Number: number,
	}, nil
}

func parseIssueURL(input string) (IssueRef, error) {
	u, err := url.Parse(input)
	if err != nil {
		return IssueRef{}, fmt.Errorf("invalid issue URL: %q", input)
	}

	host := strings.ToLower(u.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return IssueRef{}, fmt.Errorf("unsupported issue URL host: %q", u.Hostname())
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 4 {
		return IssueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	if parts[2] != "issue" && parts[2] != "issues" {
		return IssueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return IssueRef{}, fmt.Errorf("invalid issue number in URL: %q", parts[3])
	}

	owner := parts[0]
	repo := parts[1]
	if owner == "" || repo == "" {
		return IssueRef{}, fmt.Errorf("invalid issue URL path: %q", u.Path)
	}

	return IssueRef{Owner: owner, Repo: repo, Number: number}, nil
}

func parseTimelineRef(input string) (TimelineRef, error) {
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

	return TimelineRef{
		Owner:    repoRef[:slashIndex],
		Repo:     repoRef[slashIndex+1:],
		Number:   number,
		KindHint: "",
	}, nil
}
