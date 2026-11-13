package matching

import (
	"context"
	"testing"
)

func TestListMatchesRejectsInvalidPagination(t *testing.T) {
	service := &Service{}
	for _, pair := range [][2]int{{0, 0}, {101, 0}, {1, -1}} {
		if _, err := service.ListMatches(context.Background(), pair[0], pair[1]); err == nil {
			t.Errorf("ListMatches(%d, %d) accepted invalid pagination", pair[0], pair[1])
		}
	}
}

func TestCandidateDecisionAppliesSubjectMismatchGuard(t *testing.T) {
	fox, reason := candidateDecision("red fox", "red fox", "red fox", "red fox", 0.94, 0.65, 0.82, 0.35, false)
	if !fox || reason != "" {
		t.Fatalf("red fox decision=(%v,%q), want accepted", fox, reason)
	}

	foxBehavior, reason := candidateDecision("red fox hunting behavior", "red fox hunting behavior", "red fox", "red fox", 0.90, 0.65, 0.72, 0.35, false)
	if !foxBehavior || reason != "" {
		t.Fatalf("specific fox article decision=(%v,%q), want accepted for its red fox image", foxBehavior, reason)
	}

	pluralWolf, reason := candidateDecision("gray wolves", "gray wolves", "gray wolf", "gray wolf", 0.90, 0.65, 0.72, 0.35, false)
	if !pluralWolf || reason != "" {
		t.Fatalf("plural wolf subject decision=(%v,%q), want accepted for its singular gray wolf image", pluralWolf, reason)
	}

	wolf, reason := candidateDecision("red fox", "red fox", "gray wolf", "gray wolf", 0.94, 0.65, 0.9, 0.35, false)
	if wolf || reason != "Subject mismatch: article subject 'red fox' does not match image subject 'gray wolf'." {
		t.Fatalf("wolf decision=(%v,%q), want explicit subject mismatch", wolf, reason)
	}

	dog, reason := candidateDecision("red fox", "red fox", "golden retriever", "golden retriever", 0.94, 0.65, 0.72, 0.35, false)
	if dog || reason == "" {
		t.Fatalf("dog decision=(%v,%q), want rejected with an explanation", dog, reason)
	}
}

func TestCandidateDecisionRejectsUncertainAndLowSimilarity(t *testing.T) {
	if accepted, reason := candidateDecision("red fox", "red fox", "red fox", "red fox", 0.94, 0.65, 0.8, 0.35, true); accepted || reason == "" {
		t.Fatal("uncertain image was accepted")
	}
	if accepted, reason := candidateDecision("red fox", "red fox", "red fox", "red fox", 0.94, 0.65, 0.2, 0.35, false); accepted || reason == "" {
		t.Fatal("below-threshold image was accepted")
	}
}

func TestCandidateDecisionAbstainsForUnrelatedArticle(t *testing.T) {
	accepted, reason := candidateDecision("bicycle maintenance", "bicycle maintenance", "red fox", "red fox", 0.96, 0.65, 0.91, 0.35, false)
	if accepted || reason == "" {
		t.Fatalf("unrelated article decision=(%v,%q), want no confident match with an explanation", accepted, reason)
	}
}
