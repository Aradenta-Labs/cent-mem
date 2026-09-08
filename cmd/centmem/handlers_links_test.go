package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCLI_Links(t *testing.T) {
	home := newHome(t)
	stubDownloader()

	// 1. Initialize centmem store
	_, _, code := runCLI(t, home, "init")
	if code != 0 {
		t.Fatalf("init failed with code %d", code)
	}

	// 2. Put memory 1
	out1, _, code := runCLI(t, home, "put", "--scope", "project:alpha", "--type", "note", "--content", "Use Fly.io for deployments")
	if code != 0 {
		t.Fatalf("put 1 failed with code %d", code)
	}
	var res1 map[string]any
	if err := json.Unmarshal([]byte(out1), &res1); err != nil {
		t.Fatalf("unmarshal put 1: %v", err)
	}
	m1ID := int64(res1["id"].(float64))

	// 3. Put memory 2 with superseding content
	out2, _, code := runCLI(t, home, "put", "--scope", "project:alpha", "--type", "note", "--content", "Switched from Fly.io to AWS ECS for production")
	if code != 0 {
		t.Fatalf("put 2 failed with code %d", code)
	}
	var res2 map[string]any
	if err := json.Unmarshal([]byte(out2), &res2); err != nil {
		t.Fatalf("unmarshal put 2: %v", err)
	}
	m2ID := int64(res2["id"].(float64))

	// Verify auto-suggested links in put 2 output
	suggestedLinks, ok := res2["suggested_links"].([]any)
	if !ok || len(suggestedLinks) == 0 {
		t.Fatalf("expected suggested_links in put 2 output, got: %v", res2)
	}
	firstSug := suggestedLinks[0].(map[string]any)
	if firstSug["relation"] != "supersedes" {
		t.Errorf("expected relation 'supersedes', got %v", firstSug["relation"])
	}
	linkID := int64(firstSug["id"].(float64))

	// 4. centmem links <m2ID> (default confirmed only -> 0 links)
	outLinks, _, code := runCLI(t, home, "links", fmt.Sprintf("%d", m2ID))
	if code != 0 {
		t.Fatalf("links failed with code %d", code)
	}
	var linksRes map[string]any
	if err := json.Unmarshal([]byte(outLinks), &linksRes); err != nil {
		t.Fatalf("unmarshal links: %v", err)
	}
	outgoing := linksRes["outgoing"].([]any)
	if len(outgoing) != 0 {
		t.Errorf("expected 0 confirmed outgoing links, got %d", len(outgoing))
	}

	// 5. centmem links <m2ID> --all -> includes pending suggested link
	outLinksAll, _, code := runCLI(t, home, "links", fmt.Sprintf("%d", m2ID), "--all")
	if code != 0 {
		t.Fatalf("links --all failed with code %d", code)
	}
	var linksAllRes map[string]any
	if err := json.Unmarshal([]byte(outLinksAll), &linksAllRes); err != nil {
		t.Fatalf("unmarshal links --all: %v", err)
	}
	outgoingAll := linksAllRes["outgoing"].([]any)
	if len(outgoingAll) != 1 {
		t.Fatalf("expected 1 outgoing link with --all, got %d", len(outgoingAll))
	}

	// 6. Confirm the link: centmem link confirm <linkID>
	outConfirm, _, code := runCLI(t, home, "link", "confirm", fmt.Sprintf("%d", linkID))
	if code != 0 {
		t.Fatalf("link confirm failed with code %d", code)
	}
	var confirmRes map[string]any
	if err := json.Unmarshal([]byte(outConfirm), &confirmRes); err != nil {
		t.Fatalf("unmarshal confirm: %v", err)
	}
	if confirmRes["ok"] != true || int64(confirmRes["confirmed"].(float64)) != linkID {
		t.Errorf("unexpected confirm response: %v", confirmRes)
	}

	// Now centmem links <m2ID> returns the confirmed link
	outLinksAfter, _, code := runCLI(t, home, "links", fmt.Sprintf("%d", m2ID))
	if code != 0 {
		t.Fatalf("links after confirm failed with code %d", code)
	}
	if err := json.Unmarshal([]byte(outLinksAfter), &linksRes); err != nil {
		t.Fatalf("unmarshal links after: %v", err)
	}
	if len(linksRes["outgoing"].([]any)) != 1 {
		t.Errorf("expected 1 confirmed outgoing link, got %v", linksRes["outgoing"])
	}

	// 7. centmem recall --include-links
	outRecall, _, code := runCLI(t, home, "recall", "AWS ECS", "--scope", "project:alpha", "--include-links")
	if code != 0 {
		t.Fatalf("recall --include-links failed with code %d", code)
	}
	var recallRes map[string]any
	if err := json.Unmarshal([]byte(outRecall), &recallRes); err != nil {
		t.Fatalf("unmarshal recall: %v", err)
	}
	results := recallRes["results"].([]any)
	if len(results) == 0 {
		t.Fatalf("expected recall results")
	}
	topRes := results[0].(map[string]any)
	topLinks, ok := topRes["links"].([]any)
	if !ok || len(topLinks) == 0 {
		t.Fatalf("expected links in recall result, got: %v", topRes)
	}
	topLink := topLinks[0].(map[string]any)
	if topLink["relation"] != "supersedes" || topLink["direction"] != "outgoing" {
		t.Errorf("unexpected link in recall: %v", topLink)
	}

	// 8. Manual link creation: centmem link <m1ID> <m2ID> --relation depends-on
	outManual, _, code := runCLI(t, home, "link", fmt.Sprintf("%d", m1ID), fmt.Sprintf("%d", m2ID), "--relation", "depends-on")
	if code != 0 {
		t.Fatalf("manual link failed with code %d", code)
	}
	var manualRes map[string]any
	if err := json.Unmarshal([]byte(outManual), &manualRes); err != nil {
		t.Fatalf("unmarshal manual link: %v", err)
	}
	if manualRes["ok"] != true {
		t.Errorf("unexpected manual link response: %v", manualRes)
	}

	// 9. Unlink: centmem unlink <m1ID> <m2ID> --relation depends-on
	outUnlink, _, code := runCLI(t, home, "unlink", fmt.Sprintf("%d", m1ID), fmt.Sprintf("%d", m2ID), "--relation", "depends-on")
	if code != 0 {
		t.Fatalf("unlink failed with code %d", code)
	}
	var unlinkRes map[string]any
	if err := json.Unmarshal([]byte(outUnlink), &unlinkRes); err != nil {
		t.Fatalf("unmarshal unlink: %v", err)
	}
	if unlinkRes["ok"] != true || int64(unlinkRes["deleted"].(float64)) != 1 {
		t.Errorf("unexpected unlink response: %v", unlinkRes)
	}

	// 10. Put with --no-suggest: should omit suggested_links
	outNoSug, _, code := runCLI(t, home, "put", "--scope", "project:alpha", "--type", "note", "--content", "Deprecated: legacy deployment scripts", "--no-suggest")
	if code != 0 {
		t.Fatalf("put --no-suggest failed with code %d", code)
	}
	var noSugRes map[string]any
	if err := json.Unmarshal([]byte(outNoSug), &noSugRes); err != nil {
		t.Fatalf("unmarshal put no-suggest: %v", err)
	}
	if _, hasSug := noSugRes["suggested_links"]; hasSug {
		t.Errorf("expected suggested_links to be omitted with --no-suggest, got %v", noSugRes["suggested_links"])
	}

	// 11. Links on nonexistent memory returns ExitNotFound (code 2)
	_, _, code = runCLI(t, home, "links", "999999")
	if code != 2 {
		t.Errorf("expected exit code 2 for links on nonexistent memory, got %d", code)
	}

	// 12. Link creation on nonexistent memory returns ExitNotFound (code 2)
	_, _, code = runCLI(t, home, "link", "999999", fmt.Sprintf("%d", m1ID), "--relation", "supports")
	if code != 2 {
		t.Errorf("expected exit code 2 for link with nonexistent source, got %d", code)
	}

	// 13. Unlink with invalid relation returns ExitError (code 1)
	_, _, code = runCLI(t, home, "unlink", fmt.Sprintf("%d", m1ID), fmt.Sprintf("%d", m2ID), "--relation", "invalid_rel")
	if code != 1 {
		t.Errorf("expected exit code 1 for unlink with invalid relation, got %d", code)
	}

	// 14. Recall with --include-suggested alone includes links
	// Create an auto-suggested link from m2 to m1
	_, _, code = runCLI(t, home, "put", "--scope", "project:alpha", "--type", "note", "--content", "Instead of Fly.io we now deploy with ECS")
	if code != 0 {
		t.Fatalf("put for suggest test failed: %d", code)
	}
	outRecallSug, _, code := runCLI(t, home, "recall", "ECS", "--scope", "project:alpha", "--include-suggested")
	if code != 0 {
		t.Fatalf("recall --include-suggested failed: %d", code)
	}
	var recallSugRes map[string]any
	if err := json.Unmarshal([]byte(outRecallSug), &recallSugRes); err != nil {
		t.Fatalf("unmarshal recall --include-suggested: %v", err)
	}
	sugResults := recallSugRes["results"].([]any)
	if len(sugResults) == 0 {
		t.Fatalf("expected recall results for --include-suggested")
	}
	sugItem := sugResults[0].(map[string]any)
	if _, hasLinks := sugItem["links"]; !hasLinks {
		t.Errorf("expected 'links' array in results when --include-suggested is passed")
	}
}
