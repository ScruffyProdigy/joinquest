package spiritanimal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/openai"
	"github.com/scruffyprodigy/joinquest/internal/store"
	"github.com/scruffyprodigy/joinquest/internal/tarot"
)

const artDirectionVersion = 7

// LLM generates structured JSON for spirit animal steps.
type LLM interface {
	ChatJSON(ctx context.Context, systemPrompt, userPrompt string) (json.RawMessage, error)
}

// ImageGenerator creates mascot images.
type ImageGenerator interface {
	GenerateImage(ctx context.Context, prompt string) ([]byte, error)
}

// Storage persists generated mascot images and builds public URLs.
type Storage interface {
	SavePNG(ctx context.Context, readingID uuid.UUID, totemSlug string, data []byte) (publicURL string, err error)
	TotemExists(readingID uuid.UUID, totemSlug string) bool
}

// Runner executes the async spirit-animal pipeline.
type Runner struct {
	Store   *store.Store
	LLM     LLM
	Images  ImageGenerator
	Storage Storage
}

func readingArchived(reading *store.AvatarReading) bool {
	return reading == nil || reading.CompletedAt != nil
}

func (r *Runner) readingAborted(ctx context.Context, readingID uuid.UUID) bool {
	if r == nil || r.Store == nil {
		return true
	}
	reading, err := r.Store.GetAvatarReadingByID(ctx, readingID)
	return err != nil || readingArchived(reading)
}

// GenerateQuestions runs step 2 for a reading.
func (r *Runner) GenerateQuestions(ctx context.Context, readingID uuid.UUID) {
	if !markResuming(readingID) {
		return
	}
	defer unmarkResuming(readingID)
	r.generateQuestions(ctx, readingID)
}

func (r *Runner) generateQuestions(ctx context.Context, readingID uuid.UUID) {
	started := time.Now()
	reading, err := r.Store.GetAvatarReadingByID(ctx, readingID)
	if err != nil {
		log.Printf("spiritanimal: reading %s: %v", readingID, err)
		return
	}
	if readingArchived(reading) {
		return
	}
	if len(reading.QuestionsJSON) > 0 {
		if reading.Status == store.ReadingStatusGeneratingQuestions {
			if _, err := r.Store.SaveAvatarReadingQuestions(ctx, readingID, reading.QuestionsJSON); err != nil {
				log.Printf("spiritanimal: promote questions %s: %v", readingID, err)
			}
		}
		return
	}

	drawPayload, _ := json.Marshal(map[string][]int{"draw": intsFromDraw(reading.Draw)})
	userPrompt := string(drawPayload)
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if r.readingAborted(ctx, readingID) {
			return
		}
		prompt := userPrompt
		if attempt > 1 {
			prompt = userPrompt + "\n\nYour previous response was invalid. Each card MUST include exactly five answers with ids A, B, C, D, E. Use slot keys compass, coin, storm, campfire, beacon (not numbers). Answer labels must be short phrases, NOT questions — no question marks. The five answers on each card must be sharply different from each other — no synonyms, overlapping vibes, or repeated word roots."
		}
		raw, err := r.LLM.ChatJSON(ctx, questionsSystemPrompt, prompt)
		if err != nil {
			lastErr = err
			continue
		}
		normalized, err := normalizeQuestionsJSON(raw)
		if err != nil {
			lastErr = err
			log.Printf("spiritanimal: reading %s questions attempt %d: %v", readingID, attempt, err)
			continue
		}
		if _, err := r.Store.SaveAvatarReadingQuestions(ctx, readingID, normalized); err != nil {
			log.Printf("spiritanimal: save questions %s: %v", readingID, err)
			return
		}
		RecordQuestionsDuration(time.Since(started))
		return
	}
	r.fail(ctx, readingID, lastErr)
}

// ProcessAnswers runs steps 4–7 after the user submits answers.
func (r *Runner) ProcessAnswers(ctx context.Context, readingID uuid.UUID) {
	if !markResuming(readingID) {
		return
	}
	defer unmarkResuming(readingID)
	r.processAnswers(ctx, readingID)
}

func (r *Runner) processAnswers(ctx context.Context, readingID uuid.UUID) {
	started := time.Now()
	reading, err := r.Store.GetAvatarReadingByID(ctx, readingID)
	if err != nil {
		log.Printf("spiritanimal: reading %s: %v", readingID, err)
		return
	}
	if readingArchived(reading) {
		return
	}
	if reading.Status == store.ReadingStatusReady || reading.Status == store.ReadingStatusCompleted {
		return
	}
	if len(reading.UserAnswers) != 5 || len(reading.QuestionsJSON) == 0 {
		r.fail(ctx, readingID, fmt.Errorf("reading missing questions or answers"))
		return
	}

	personalityRaw := reading.PersonalityJSON
	if len(personalityRaw) == 0 {
		if r.readingAborted(ctx, readingID) {
			return
		}
		answersJSON, _ := json.Marshal(reading.UserAnswers)
		interpretPrompt := fmt.Sprintf("Cards:\n\n%s\n\nUser Answers:\n\n%s", string(reading.QuestionsJSON), string(answersJSON))
		personalityRaw, err = r.LLM.ChatJSON(ctx, interpretSystemPrompt, interpretPrompt)
		if err != nil {
			r.fail(ctx, readingID, err)
			return
		}
		if err := validatePersonalityJSON(personalityRaw); err != nil {
			r.fail(ctx, readingID, err)
			return
		}
		if _, err := r.Store.SaveAvatarReadingPersonality(ctx, readingID, personalityRaw); err != nil {
			r.fail(ctx, readingID, err)
			return
		}
	} else if err := validatePersonalityJSON(personalityRaw); err != nil {
		r.fail(ctx, readingID, err)
		return
	}

	totemsRaw := reading.TotemsJSON
	if len(totemsRaw) == 0 {
		const maxTotemAttempts = 3
		var lastErr error
		totemsPrompt := string(personalityRaw)
		for attempt := 1; attempt <= maxTotemAttempts; attempt++ {
			if r.readingAborted(ctx, readingID) {
				return
			}
			prompt := totemsPrompt
			if attempt > 1 {
				prompt = totemsPrompt + "\n\nYour previous totems were invalid: " + lastErr.Error() +
					". Fix the issues and return valid JSON."
			}
			totemsRaw, err = r.LLM.ChatJSON(ctx, totemsSystemPrompt, prompt)
			if err != nil {
				lastErr = err
				continue
			}
			if err := validateTotemsJSON(totemsRaw); err != nil {
				lastErr = err
				log.Printf("spiritanimal: reading %s totems attempt %d: %v", readingID, attempt, err)
				continue
			}
			lastErr = nil
			break
		}
		if lastErr != nil {
			r.fail(ctx, readingID, lastErr)
			return
		}
	}

	if r.readingAborted(ctx, readingID) {
		return
	}
	enrichedTotems, err := r.generateTotemImages(ctx, readingID, totemsRaw)
	if err != nil {
		r.fail(ctx, readingID, err)
		return
	}
	if _, err := r.Store.SaveAvatarReadingTotems(ctx, readingID, enrichedTotems); err != nil {
		r.fail(ctx, readingID, err)
		return
	}

	if len(reading.RankingJSON) > 0 {
		if reading.Status == store.ReadingStatusProcessing {
			if _, err := r.Store.UpdateAvatarReadingStatus(ctx, readingID, store.ReadingStatusReady, nil); err != nil {
				log.Printf("spiritanimal: mark ready %s: %v", readingID, err)
			}
		}
		RecordMascotsDuration(time.Since(started))
		return
	}

	rankPrompt := fmt.Sprintf("Personality Reading:\n\n%s\n\nMascots:\n\n%s", string(personalityRaw), string(enrichedTotems))
	if r.readingAborted(ctx, readingID) {
		return
	}
	rankingRaw, err := r.LLM.ChatJSON(ctx, rankingSystemPrompt, rankPrompt)
	if err != nil {
		r.fail(ctx, readingID, err)
		return
	}
	if err := validateRankingJSON(rankingRaw); err != nil {
		r.fail(ctx, readingID, err)
		return
	}
	if _, err := r.Store.SaveAvatarReadingRanking(ctx, readingID, rankingRaw); err != nil {
		log.Printf("spiritanimal: save ranking %s: %v", readingID, err)
	}
	RecordMascotsDuration(time.Since(started))
}

// RegenerateTotemImages re-renders mascot PNGs for a completed reading (e.g. after storage loss).
func (r *Runner) RegenerateTotemImages(ctx context.Context, readingID uuid.UUID) {
	reading, err := r.Store.GetAvatarReadingByID(ctx, readingID)
	if err != nil {
		log.Printf("spiritanimal: reading %s: %v", readingID, err)
		return
	}
	if readingArchived(reading) {
		return
	}
	if len(reading.TotemsJSON) == 0 {
		r.fail(ctx, readingID, fmt.Errorf("reading missing totems"))
		return
	}

	enrichedTotems, err := r.generateTotemImages(ctx, readingID, reading.TotemsJSON)
	if err != nil {
		r.fail(ctx, readingID, err)
		return
	}
	if _, err := r.Store.SaveAvatarReadingTotems(ctx, readingID, enrichedTotems); err != nil {
		r.fail(ctx, readingID, err)
		return
	}
	if _, err := r.Store.UpdateAvatarReadingStatus(ctx, readingID, store.ReadingStatusReady, nil); err != nil {
		log.Printf("spiritanimal: mark ready after regen %s: %v", readingID, err)
	}
}

// ImagesMissingForReading is true when totem URLs exist but PNG files are absent from storage.
func (r *Runner) ImagesMissingForReading(reading *store.AvatarReading) bool {
	if reading == nil || len(reading.TotemsJSON) == 0 || r.Storage == nil {
		return false
	}
	if reading.Status != store.ReadingStatusReady && reading.Status != store.ReadingStatusCompleted {
		return false
	}

	var payload struct {
		Totems []struct {
			Name     string `json:"name"`
			ImageURL string `json:"image_url"`
		} `json:"totems"`
	}
	if err := json.Unmarshal(reading.TotemsJSON, &payload); err != nil {
		return false
	}
	for _, totem := range payload.Totems {
		if strings.TrimSpace(totem.ImageURL) == "" {
			continue
		}
		if !r.Storage.TotemExists(reading.ID, slugify(totem.Name)) {
			return true
		}
	}
	return false
}

func (r *Runner) generateTotemImages(ctx context.Context, readingID uuid.UUID, totemsRaw json.RawMessage) (json.RawMessage, error) {
	var payload struct {
		Totems []map[string]any `json:"totems"`
	}
	if err := json.Unmarshal(totemsRaw, &payload); err != nil {
		return nil, err
	}
	for i, totem := range payload.Totems {
		if r.readingAborted(ctx, readingID) {
			return nil, fmt.Errorf("reading archived")
		}
		name, _ := totem["name"].(string)
		slug := slugify(name)
		if imageURL, _ := totem["image_url"].(string); strings.TrimSpace(imageURL) != "" && r.Storage.TotemExists(readingID, slug) {
			continue
		}
		animal, _ := totem["animal"].(string)
		pose, _ := totem["pose"].(string)
		expression, _ := totem["expression"].(string)
		imagePrompt, _ := totem["image_prompt"].(string)
		if strings.TrimSpace(imagePrompt) == "" {
			return nil, fmt.Errorf("totem %q missing image_prompt", name)
		}
		subject := strings.TrimSpace(animal)
		if subject == "" {
			subject = name
		}
		fullPrompt := imageSystemPrompt + "\n\n" +
			"Subject: " + subject + " animal mascot companion.\n\n" +
			imagePrompt
		if p := strings.TrimSpace(effectivePoseForImage(readingID, animal, pose)); p != "" {
			fullPrompt += "\n\nRequired pose: " + p + "."
		}
		if e := strings.TrimSpace(expression); e != "" {
			fullPrompt += "\n\nRequired expression: " + e + "."
		}
		fullPrompt += "\n\nDo not use a generic front-facing standing idle pose unless the required pose above explicitly calls for it."
		png, err := r.generateImageWithRetry(ctx, fullPrompt)
		if err != nil {
			return nil, err
		}
		publicURL, err := r.Storage.SavePNG(ctx, readingID, slugify(name), png)
		if err != nil {
			return nil, err
		}
		payload.Totems[i]["image_url"] = publicURL
		if _, err := r.Store.InsertAvatarRender(ctx, readingID, name, artDirectionVersion, publicURL, imagePrompt); err != nil {
			return nil, err
		}
	}
	return json.Marshal(payload)
}

func (r *Runner) generateImageWithRetry(ctx context.Context, prompt string) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		png, err := r.Images.GenerateImage(ctx, prompt)
		if err == nil {
			return png, nil
		}
		lastErr = err
		if !isRetryableImageError(err) || attempt == maxAttempts {
			return nil, err
		}
		log.Printf("spiritanimal: image attempt %d/%d failed, retrying: %v", attempt, maxAttempts, err)
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	return nil, lastErr
}

func isRetryableImageError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "http 500") ||
		strings.Contains(msg, "http 502") ||
		strings.Contains(msg, "http 503") ||
		strings.Contains(msg, "server_error") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset")
}

func (r *Runner) fail(ctx context.Context, readingID uuid.UUID, err error) {
	if err == nil {
		return
	}
	log.Printf("spiritanimal: reading %s failed: %v", readingID, err)
	if r.readingAborted(ctx, readingID) {
		log.Printf("spiritanimal: reading %s archived, skipping fail mark", readingID)
		return
	}
	userMsg := UserFacingError(err)
	if _, saveErr := r.Store.FailAvatarReading(ctx, readingID, userMsg); saveErr != nil {
		log.Printf("spiritanimal: mark failed %s: %v", readingID, saveErr)
	}
}

func intsFromDraw(draw []int32) []int {
	out := make([]int, len(draw))
	for i, n := range draw {
		out[i] = int(n)
	}
	return out
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugPattern.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// NewRunnerFromEnv wires real or mock providers based on environment.
func NewRunnerFromEnv(st *store.Store, _ string) *Runner {
	runner := &Runner{
		Store:   st,
		Storage: NewFileStorage(storageDirFromEnv()),
	}
	if client, ok := openai.NewClientFromEnv(); ok {
		runner.LLM = client
		runner.Images = client
	} else {
		runner.LLM = MockLLM{}
		runner.Images = MockImages{}
	}
	return runner
}

func storageDirFromEnv() string {
	dir := strings.TrimSpace(os.Getenv("SPIRIT_AVATAR_STORAGE_DIR"))
	if dir == "" {
		dir = "data/spirit-avatars"
	}
	return dir
}

// DrawCards draws tarot cards for a new reading.
func DrawCards() ([]int, error) {
	return tarot.Draw()
}
