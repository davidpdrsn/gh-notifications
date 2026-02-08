package cli

import "testing"

func TestParsePRRefHashFormat(t *testing.T) {
	ref, err := parsePRRef("tokio-rs/axum#2398")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "tokio-rs" || ref.Repo != "axum" || ref.Number != 2398 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParsePRRefURLFormat(t *testing.T) {
	ref, err := parsePRRef("https://github.com/lun-energy/calor/pull/1556")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParsePRRefURLFormatWithFragmentAndQuery(t *testing.T) {
	ref, err := parsePRRef("https://github.com/lun-energy/calor/pull/1556/files?foo=1#diff-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseIssueRefHashFormat(t *testing.T) {
	ref, err := parseIssueRef("tokio-rs/axum#2398")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "tokio-rs" || ref.Repo != "axum" || ref.Number != 2398 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseIssueRefURLFormat(t *testing.T) {
	ref, err := parseIssueRef("https://github.com/lun-energy/calor/issues/1556")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseIssueRefURLFormatWithFragmentAndQuery(t *testing.T) {
	ref, err := parseIssueRef("https://github.com/lun-energy/calor/issues/1556?foo=1#issuecomment-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseTimelineRefHashFormat(t *testing.T) {
	ref, err := parseTimelineRef("tokio-rs/axum#2398")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "tokio-rs" || ref.Repo != "axum" || ref.Number != 2398 || ref.KindHint != "" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseTimelineRefPRURLFormat(t *testing.T) {
	ref, err := parseTimelineRef("https://github.com/lun-energy/calor/pull/1556")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 || ref.KindHint != "pr" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseTimelineRefIssueURLFormat(t *testing.T) {
	ref, err := parseTimelineRef("https://github.com/lun-energy/calor/issues/1556")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref.Owner != "lun-energy" || ref.Repo != "calor" || ref.Number != 1556 || ref.KindHint != "issue" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}
