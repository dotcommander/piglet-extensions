package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

// truncateToolResults truncates large tool-result text content in messages
// to maxChars. This prevents bash/read output from dominating the LLM
// summarizer's token budget during compaction.
func truncateToolResults(msgs []wireMsg, maxChars int) {
	for i := range msgs {
		if msgs[i].Type != "tool_result" {
			continue
		}

		var tr wireToolResult
		if json.Unmarshal(msgs[i].Data, &tr) != nil {
			continue
		}

		modified := false
		for j := range tr.Content {
			if tr.Content[j].Type != "text" {
				continue
			}
			runes := []rune(tr.Content[j].Text)
			if len(runes) > maxChars {
				tr.Content[j].Text = string(runes[:maxChars]) + "\n[...truncated for compaction]"
				modified = true
			}
		}

		if modified {
			data, err := json.Marshal(tr)
			if err != nil {
				// Truncation failed — replace with error marker so
				// untruncated content does not survive compaction.
				data, _ = json.Marshal(map[string]any{
					"content": fmt.Sprintf("[tool result truncation failed: %s]", err),
				})
			}
			msgs[i].Data = data
		}
	}
}

// extractPriorFileLists scans messages for prior compaction summaries
// containing <read-files> and <modified-files> XML tags and returns
// the accumulated file paths. This enables cumulative file tracking
// across compaction boundaries — files from earlier compactions are
// preserved when the next compaction runs.
func extractPriorFileLists(msgs []wireMsg) (readFiles, modifiedFiles []string) {
	readSet := make(map[string]struct{})
	modSet := make(map[string]struct{})

	for _, msg := range msgs {
		var m struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(msg.Data, &m) != nil || m.Content == "" {
			continue
		}

		for _, path := range parseXMLTag(m.Content, "read-files") {
			readSet[path] = struct{}{}
		}
		for _, path := range parseXMLTag(m.Content, "modified-files") {
			modSet[path] = struct{}{}
		}
	}

	for path := range readSet {
		readFiles = append(readFiles, path)
	}
	for path := range modSet {
		modifiedFiles = append(modifiedFiles, path)
	}
	return readFiles, modifiedFiles
}

// mergeFileLists appends prior compaction file paths into the current
// summary's <read-files> and <modified-files> tags, deduplicating entries.
func mergeFileLists(summary string, priorRead, priorModified []string) string {
	if len(priorRead) == 0 && len(priorModified) == 0 {
		return summary
	}

	existingRead := parseXMLTag(summary, "read-files")
	existingMod := parseXMLTag(summary, "modified-files")

	mergedRead := dedup(append(existingRead, priorRead...))
	mergedMod := dedup(append(existingMod, priorModified...))

	summary = stripXMLTag(summary, "read-files")
	summary = stripXMLTag(summary, "modified-files")
	summary = strings.TrimRight(summary, "\n")

	var b strings.Builder
	b.WriteString(summary)

	if len(mergedRead) > 0 {
		b.WriteString("\n\n")
		writeXMLTagBlock(&b, "read-files", mergedRead)
	}
	if len(mergedMod) > 0 {
		b.WriteString("\n\n")
		writeXMLTagBlock(&b, "modified-files", mergedMod)
	}

	return b.String()
}

// findTagBounds returns byte offsets for content between <tag> and </tag>.
// contentStart..contentEnd is the inner content; closeEnd is past the closing tag.
// Returns ok=false if the tag pair is not found.
func findTagBounds(text, tag string) (contentStart, contentEnd, closeEnd int, ok bool) {
	open := "<" + tag + ">"
	closeTok := "</" + tag + ">"

	start := strings.Index(text, open)
	if start < 0 {
		return 0, 0, 0, false
	}
	rel := strings.Index(text[start:], closeTok)
	if rel < 0 {
		return 0, 0, 0, false
	}
	return start + len(open), start + rel, start + rel + len(closeTok), true
}

// parseXMLTag extracts trimmed non-empty lines between <tag> and </tag>.
func parseXMLTag(text, tag string) []string {
	contentStart, contentEnd, _, ok := findTagBounds(text, tag)
	if !ok {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(text[contentStart:contentEnd], "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

// stripXMLTag removes a <tag>...</tag> block from text.
func stripXMLTag(text, tag string) string {
	open := "<" + tag + ">"
	start := strings.Index(text, open)
	if start < 0 {
		return text
	}
	_, _, closeEnd, ok := findTagBounds(text, tag)
	if !ok {
		return text
	}
	return text[:start] + text[closeEnd:]
}

// writeXMLTagBlock writes a <tag>\nitem\nitem\n</tag> block to b.
// Does nothing if items is empty.
func writeXMLTagBlock(b *strings.Builder, tag string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("<")
	b.WriteString(tag)
	b.WriteString(">\n")
	for _, item := range items {
		b.WriteString(item)
		b.WriteByte('\n')
	}
	b.WriteString("</")
	b.WriteString(tag)
	b.WriteString(">")
}

// dedup returns unique strings preserving first-seen order.
func dedup(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}
