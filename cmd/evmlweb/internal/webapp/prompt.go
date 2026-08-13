package webapp

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// fallbackPrompt is used only if EVENT_MODELING.md / SKILL.md can't be
// read from disk (e.g. the repo docs moved). Keeping a minimal fallback
// means the app still functions, just with a weaker prompt.
const fallbackPrompt = `You help non-technical domain experts build ".evml" event-modeling
diagrams from a plain-language description of a business process. Every
file starts with "eventmodeling" on its own line. Frames look like:
  tf 01 ui ScreenName
  tf 02 cmd VerbNoun { field: "value" }
  tf 03 evt NounPastTense ->> 02
Entity types: ui (screen), cmd (command/intention), evt (event/fact),
rmo (read model), pcr (processor/automation). Use rf instead of tf to
mark an external boundary. Always respond with the complete, valid
.evml document, never a diff or partial snippet.`

// BuildSystemPrompt assembles the LLM's system prompt from this repo's own
// authoring guides (EVENT_MODELING.md grammar reference, SKILL.md
// natural-language-to-DSL guidance), so the model is taught the exact
// grammar this app's parser accepts. docsRoot is the repo root containing
// both files.
func BuildSystemPrompt(docsRoot string) string {
	grammar := readDocOrEmpty(docsRoot, "EVENT_MODELING.md")
	skill := readDocOrEmpty(docsRoot, "SKILL.md")

	if grammar == "" && skill == "" {
		return fallbackPrompt
	}

	var b strings.Builder
	b.WriteString("You help non-technical domain experts — who may come from different " +
		"departments and don't know software jargon — build and refine \".evml\" event-" +
		"modeling diagrams by describing their business process in plain language.\n\n")
	b.WriteString("Speak to them in their own business vocabulary (ubiquitous language), " +
		"never in technical terms like \"schema\", \"API\", or \"database\".\n\n")
	b.WriteString("Below is the complete DSL grammar reference and authoring guide for the " +
		".evml format you must produce.\n\n")
	b.WriteString("=== EVENT_MODELING.md (grammar reference) ===\n")
	b.WriteString(grammar)
	b.WriteString("\n\n=== SKILL.md (authoring guide) ===\n")
	b.WriteString(skill)
	b.WriteString("\n\n=== Response format ===\n")
	b.WriteString("Always reply with a short (1-3 sentence) plain-language explanation of " +
		"what you changed or are proposing, followed by the COMPLETE updated .evml " +
		"document (not a diff, not a partial snippet — the whole file from the " +
		"'eventmodeling' header onward) in a fenced code block tagged evml, like:\n\n" +
		"```evml\neventmodeling\n...\n```\n\n" +
		"If the expert's request is ambiguous, ask a clarifying question in plain " +
		"language instead of guessing — but once you have enough to act, always include " +
		"the full .evml block.")
	return b.String()
}

func readDocOrEmpty(root, name string) string {
	b, err := os.ReadFile(root + string(os.PathSeparator) + name)
	if err != nil {
		return ""
	}
	return string(b)
}

var evmlFenceRe = regexp.MustCompile("(?s)```evml\\s*\\n(.*?)```")

// ExtractEvml pulls the first ```evml fenced block out of an LLM response.
// It returns ok=false if no such block is present.
func ExtractEvml(response string) (string, bool) {
	m := evmlFenceRe.FindStringSubmatch(response)
	if m == nil {
		return "", false
	}
	return strings.TrimSpace(m[1]), true
}

// ValidationErrorsText joins a slice of validation errors into a single
// human-readable block suitable for feeding back to the LLM or the chat
// transcript.
func ValidationErrorsText(errs []error) string {
	var b strings.Builder
	for i, e := range errs {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- %s", e.Error())
	}
	return b.String()
}
