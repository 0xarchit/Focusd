package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	ModelURL         = "https://models.github.ai/inference/chat/completions"
	ModelName        = "gpt-4o-mini"
	MaxCharsPerChunk = 15000
	MaxDiffChars     = 1500
	OutputFile       = "release_notes.md"
)

var IncludedPaths = []string{
	"cli/*", "cmd/*", "core/*", "storage/*", "system/*", "tui/*", "ui/*", "installer.nsi", "go.mod",
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatPayload struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type ChatChoice struct {
	Message Message `json:"message"`
}

type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
}

func main() {
	apiKey := os.Getenv("GH_MODELS_API_KEY")
	commitData := getCommitData()

	var rawLines []string
	for _, c := range commitData {
		lines := strings.Split(c, "\n")
		if len(lines) > 1 {
			rawLines = append(rawLines, lines[1])
		}
	}

	rawChangelog := "## Commits\n"
	if len(rawLines) > 0 {
		rawChangelog += strings.Join(rawLines, "\n")
	} else {
		rawChangelog += "Maintenance release."
	}

	if apiKey == "" {
		fmt.Println("No GH_MODELS_API_KEY environment variable found. Writing raw changelog fallback...")
		_ = os.WriteFile(OutputFile, []byte(rawChangelog), 0644)
		return
	}

	if len(commitData) == 0 {
		fmt.Println("No commits detected. Writing default fallback...")
		_ = os.WriteFile(OutputFile, []byte("System optimizations and stability improvements."), 0644)
		return
	}

	err := generateAIReleaseNotes(commitData, apiKey)
	if err != nil {
		fmt.Printf("AI generation failed (falling back to commit messages): %v\n", err)
		_ = os.WriteFile(OutputFile, []byte(rawChangelog), 0644)
	} else {
		fmt.Println("Successfully generated release notes via AI!")
	}
}

func getCommitData() []string {
	var commitData []string

	// 1. Get tags
	cmdTags := exec.Command("git", "tag", "--sort=-creatordate")
	outTags, err := cmdTags.Output()
	if err != nil {
		return nil
	}
	tags := strings.Fields(string(outTags))

	var logRange []string
	if len(tags) == 0 {
		logRange = []string{"log", "--pretty=format:%h"}
	} else if len(tags) >= 2 {
		logRange = []string{"log", fmt.Sprintf("%s..%s", tags[1], tags[0]), "--pretty=format:%h"}
	} else {
		logRange = []string{"log", tags[0], "--pretty=format:%h"}
	}

	cmdLog := exec.Command("git", logRange...)
	outLog, err := cmdLog.Output()
	if err != nil {
		return nil
	}
	hashes := strings.Fields(string(outLog))

	count := 0
	for _, h := range hashes {
		if count >= 100 {
			break
		}
		cmdShowMsg := exec.Command("git", "show", "-s", "--format=%s", h)
		outMsg, err := cmdShowMsg.Output()
		if err != nil {
			continue
		}
		msg := strings.TrimSpace(string(outMsg))

		args := append([]string{"show", "--patch", "--stat", "--format=", h, "--"}, IncludedPaths...)
		cmdShowDiff := exec.Command("git", args...)
		outDiff, err := cmdShowDiff.Output()
		if err != nil {
			continue
		}
		diff := string(outDiff)
		if strings.TrimSpace(diff) == "" {
			continue
		}

		if len(diff) > MaxDiffChars {
			diff = diff[:MaxDiffChars] + "\n...[truncated]"
		}

		commitData = append(commitData, fmt.Sprintf("Commit: %s\nMessage: %s\nChanges:\n%s", h, msg, diff))
		count++
	}

	return commitData
}

func generateAIReleaseNotes(commitData []string, apiKey string) error {
	var chunks []string
	curr := ""
	for _, d := range commitData {
		if len(curr)+len(d) > MaxCharsPerChunk {
			chunks = append(chunks, curr)
			curr = ""
		}
		curr += d + "\n\n"
	}
	if curr != "" {
		chunks = append(chunks, curr)
	}

	var summaries []string
	for _, chunk := range chunks {
		payload := ChatPayload{
			Model: ModelName,
			Messages: []Message{
				{Role: "system", Content: "You are a technical lead. Summarize these commits and their code changes into clear bullet points. Focus on Features, Fixes, and Refactors. No emojis. Technical tone only."},
				{Role: "user", Content: fmt.Sprintf("Commits and diffs:\n%s", chunk)},
			},
			Temperature: 0.2,
		}
		summary, err := callAIWithRetries(payload, apiKey)
		if err != nil {
			return err
		}
		if summary != "" {
			summaries = append(summaries, summary)
		}
		time.Sleep(2 * time.Second)
	}

	combinedSummaries := strings.Join(summaries, "\n\n")
	finalPayload := ChatPayload{
		Model: ModelName,
		Messages: []Message{
			{Role: "system", Content: "You are a professional software release manager. Create a high-quality GitHub release description from the provided summaries. Use headers: ## Key Features, ## Bug Fixes, and ## Technical Improvements. Strictly NO emojis. Use a clean, engineering-focused tone."},
			{Role: "user", Content: fmt.Sprintf("Partial summaries:\n%s", combinedSummaries)},
		},
		Temperature: 0.4,
	}

	finalNotes, err := callAIWithRetries(finalPayload, apiKey)
	if err != nil {
		return err
	}
	if finalNotes == "" {
		return fmt.Errorf("empty AI response")
	}

	return os.WriteFile(OutputFile, []byte(finalNotes), 0644)
}

func callAIWithRetries(payload ChatPayload, apiKey string) (string, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	jsonData, _ := json.Marshal(payload)

	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequest("POST", ModelURL, bytes.NewBuffer(jsonData))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == 429 {
					time.Sleep(time.Duration(10*(attempt+1)) * time.Second)
					continue
				}
			}
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		var chatResponse ChatResponse
		err = json.NewDecoder(resp.Body).Decode(&chatResponse)
		resp.Body.Close()
		if err == nil && len(chatResponse.Choices) > 0 {
			return chatResponse.Choices[0].Message.Content, nil
		}
	}

	return "", fmt.Errorf("failed to fetch AI summary")
}
