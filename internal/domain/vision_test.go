package domain

import "testing"

func TestParseImageMetadata(t *testing.T) {
	got, err := ParseImageMetadata([]byte(`{"subject":" red fox ","category":"animal","attributes":["orange fur","forest"],"caption":"A red fox in a forest.","confidence":0.94}`))
	if err != nil {
		t.Fatalf("ParseImageMetadata() error = %v", err)
	}
	if got.Subject != "red fox" || got.Confidence != 0.94 || SubjectKey(got.Subject) != "red fox" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
}

func TestParseImageMetadataRejectsMalformedOrUncertainConfidence(t *testing.T) {
	tests := []string{
		`{"subject":"fox","category":"animal","attributes":[],"caption":"fox"}`,
		`{"subject":"fox","category":"animal","attributes":[],"caption":"fox","confidence":1.1}`,
		`{"subject":"fox","category":"animal","attributes":[],"caption":"fox","confidence":0.5,"extra":true}`,
		`{"subject":"fox","category":"animal","attributes":[],"caption":"fox","confidence":0.5} trailing`,
	}
	for _, input := range tests {
		if _, err := ParseImageMetadata([]byte(input)); err == nil {
			t.Errorf("ParseImageMetadata(%q) error = nil, want validation failure", input)
		}
	}
}
