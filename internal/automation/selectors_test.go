package automation

import (
	"strings"
	"testing"
)

// TestXPathPlaceholderSubstitution checks that the $LABEL substitution
// pattern used by SetAspectRatio, SetOutputCount, SetModel generates a
// well-formed XPath that quotes the user value safely. A bug here means
// every settings click fails silently because the XPath finds no element.
func TestXPathPlaceholderSubstitution(t *testing.T) {
	tests := []struct {
		name     string
		template string
		label    string
		mustHave []string
	}{
		{
			name:     "AspectTab 16:9",
			template: AspectTabXPath,
			label:    `"16:9"`,
			mustHave: []string{`role="tab"`, `"16:9"`},
		},
		{
			name:     "OutputCount 4",
			template: OutputCountTabXPath,
			label:    `"4"`,
			mustHave: []string{`role="tab"`, `"4"`},
		},
		{
			name:     "ModelMenuItem t2v_fast",
			template: ModelMenuItemXPath,
			label:    `"veo_3_1_t2v_fast"`,
			mustHave: []string{`role="menuitem"`, `"veo_3_1_t2v_fast"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.Replace(tt.template, "$LABEL", tt.label, 1)
			for _, must := range tt.mustHave {
				if !strings.Contains(got, must) {
					t.Errorf("XPath %q missing %q", got, must)
				}
			}
			// Sanity: $LABEL must be fully replaced (one occurrence per template).
			if strings.Contains(got, "$LABEL") {
				t.Errorf("XPath still contains $LABEL placeholder: %q", got)
			}
		})
	}
}

// TestCreateButtonMinYPositive validates the magic Y-pixel filter wasn't
// accidentally set to 0 or a negative value (which would match every button
// on the page including hidden/sidebar duplicates).
func TestCreateButtonMinYPositive(t *testing.T) {
	if CreateButtonMinY < 100 {
		t.Errorf("CreateButtonMinY = %v - too low, will match sidebar Create buttons", CreateButtonMinY)
	}
}

// TestSelectorsHaveContent guards against accidentally emptying a const
// (which would make the helper match EVERY element).
func TestSelectorsHaveContent(t *testing.T) {
	for name, sel := range map[string]string{
		"PromptEditor":        PromptEditor,
		"CreateButtonText":    CreateButtonText,
		"SettingsBtnXPath":    SettingsBtnXPath,
		"AspectTabXPath":      AspectTabXPath,
		"OutputCountTabXPath": OutputCountTabXPath,
		"ModelMenuItemXPath":  ModelMenuItemXPath,
	} {
		if strings.TrimSpace(sel) == "" {
			t.Errorf("selector %s is empty", name)
		}
	}
}
