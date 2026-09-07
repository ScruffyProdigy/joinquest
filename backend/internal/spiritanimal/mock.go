package spiritanimal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/tarot"
)

// FileStorage saves PNG files on disk and builds public URLs.
type FileStorage struct {
	Dir string
}

func NewFileStorage(dir string) *FileStorage {
	return &FileStorage{Dir: dir}
}

// PublicSpiritAvatarPath returns the browser-facing path for a stored totem PNG.
func PublicSpiritAvatarPath(readingID uuid.UUID, totemSlug string) string {
	return fmt.Sprintf("/spirit-avatars/%s/%s.png", readingID.String(), totemSlug)
}

func (s *FileStorage) totemFilePath(readingID uuid.UUID, totemSlug string) string {
	return filepath.Join(s.Dir, readingID.String(), totemSlug+".png")
}

// TotemExists reports whether a totem PNG is present on disk.
func (s *FileStorage) TotemExists(readingID uuid.UUID, totemSlug string) bool {
	info, err := os.Stat(s.totemFilePath(readingID, totemSlug))
	return err == nil && info != nil && !info.IsDir()
}

func (s *FileStorage) SavePNG(_ context.Context, readingID uuid.UUID, totemSlug string, data []byte) (string, error) {
	dir := filepath.Join(s.Dir, readingID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := s.totemFilePath(readingID, totemSlug)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return PublicSpiritAvatarPath(readingID, totemSlug), nil
}

// MockLLM returns deterministic JSON for local dev and tests.
type MockLLM struct{}

func (MockLLM) ChatJSON(_ context.Context, systemPrompt, userPrompt string) (json.RawMessage, error) {
	switch {
	case strings.Contains(systemPrompt, "Generate 5 mascot"):
		return mockTotemsJSON(), nil
	case strings.Contains(systemPrompt, "Explain how each mascot"):
		return mockRankingJSON(), nil
	case strings.Contains(systemPrompt, "interpreting a tarot"):
		return mockPersonalityJSON(), nil
	default:
		return mockQuestionsJSON(userPrompt), nil
	}
}

// MockImages returns a tiny PNG without calling OpenAI.
type MockImages struct{}

func (MockImages) GenerateImage(_ context.Context, prompt string) ([]byte, error) {
	return MockPNG("mock")
}

func MockPNG(label string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	fill := color.RGBA{R: 90, G: 120, B: 180, A: 255}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mockQuestionsJSON(userPrompt string) json.RawMessage {
	var input struct {
		Draw []int `json:"draw"`
	}
	_ = json.Unmarshal([]byte(userPrompt), &input)
	cards := make([]map[string]any, 0, 5)
	for i, slot := range tarot.JourneySlots {
		idx := 0
		if i < len(input.Draw) {
			idx = input.Draw[i]
		}
		name, _ := tarot.CardName(idx)
		cards = append(cards, map[string]any{
			"slot":                    slot.Key,
			"slot_name":               slot.Name,
			"card":                    name,
			"card_meaning_in_general": "A symbolic turning point.",
			"card_meaning_for_slot":   fmt.Sprintf("In the %s position, %s speaks to the journey.", slot.Name, name),
			"question":                fmt.Sprintf("When the path bends toward %s, what do you trust first?", slot.Name),
			"answers": []map[string]string{
				{"id": "A", "label": "Instinct"},
				{"id": "B", "label": "Memory"},
				{"id": "C", "label": "Curiosity"},
				{"id": "D", "label": "Duty"},
				{"id": "E", "label": "Wonder"},
			},
		})
	}
	out, _ := json.Marshal(map[string]any{"cards": cards})
	return out
}

func mockPersonalityJSON() json.RawMessage {
	out, _ := json.Marshal(map[string]any{
		"overview": "You move through groups with warmth and steady curiosity.",
		"journey_summary": map[string]string{
			"compass":  "You follow signals that feel alive.",
			"coin":     "You bring calm clarity to others.",
			"storm":    "You are tested by impatience.",
			"campfire": "You become the welcoming spark.",
			"beacon":   "You are known for quiet courage.",
		},
		"core_themes":     []string{"warmth", "curiosity"},
		"strengths":       []string{"listening", "playfulness"},
		"tensions":        []string{"restlessness"},
		"social_identity": "The friend who makes room for everyone.",
		"avatar_signals": map[string]any{
			"leadership_style":          "collaborative",
			"group_role":                "connector",
			"decision_style":            "intuitive",
			"relationship_to_change":    "adaptive",
			"creative_style":            "expressive",
			"social_energy":             "bright but gentle",
			"candidate_animals":         []string{"fox", "otter", "owl", "rabbit", "deer"},
			"candidate_palettes":        []string{"amber", "seafoam", "violet", "rose", "sage"},
			"candidate_symbols":         []string{"lantern", "leaf", "star"},
			"candidate_magical_effects": []string{"soft lantern glow aura", "floating leaf sparkle motes", "gentle star halo ring"},
			"shadow_traits":             []string{"overthinking"},
			"beacon_themes":             []string{"guidance"},
		},
	})
	return out
}

func mockTotemsJSON() json.RawMessage {
	names := []string{"Ember Fox", "Tide Otter", "Moss Owl", "Spark Rabbit", "Dawn Deer"}
	animals := []string{"fox", "otter", "owl", "rabbit", "deer"}
	poses := []string{
		"seated with tail curled around paws",
		"leaning forward on one elbow, one paw raised in greeting",
		"perched on haunches with wings half-spread",
		"playful pounce crouch with ears perked",
		"mid-stride step with head turned over shoulder",
	}
	expressions := []string{"mischievous grin", "warm eager eyes", "serene half-lidded gaze", "bright surprised delight", "gentle wry smirk"}
	totems := make([]map[string]any, 0, 5)
	for i, name := range names {
		totems = append(totems, map[string]any{
			"name":                name,
			"animal":              animals[i],
			"social_archetype":    fmt.Sprintf("Archetype %d", i+1),
			"core_concept":        "A playful companion for the journey.",
			"color_palette":       []string{"amber", "cream"},
			"pose":                poses[i],
			"expression":          expressions[i],
			"accessory":           "scarf",
			"shadow_element":      "a tiny cloud",
			"beacon_ornament":     "star charm",
			"personality_summary": "Warm, curious, and easy to spot in a crowd.",
			"why_this_animal":     "Matches the reading's social energy.",
			"origin_story":        "Found at a crossroads of lantern light.",
			"image_prompt":        fmt.Sprintf("Cute chibi %s mascot with large expressive eyes, simplified shapes, strong silhouette, oversized head and tiny body. %s with %s. Friendly approachable game avatar style.", animals[i], poses[i], expressions[i]),
		})
	}
	out, _ := json.Marshal(map[string]any{"totems": totems})
	return out
}

func mockRankingJSON() json.RawMessage {
	names := []string{"Ember Fox", "Tide Otter", "Moss Owl", "Spark Rabbit", "Dawn Deer"}
	avatars := make([]map[string]any, 0, 5)
	for i, name := range names {
		avatars = append(avatars, map[string]any{
			"name":                                   name,
			"fit_score":                              95 - i*5,
			"affinity":                               "Strong match",
			"what_part_of_the_reading_it_emphasizes": "Warmth and curiosity",
			"why_this_animal_makes_sense":            "Reflects your group role.",
			"why_someone_might_choose_this_avatar":   "Feels welcoming at a table.",
		})
	}
	out, _ := json.Marshal(map[string]any{
		"overview": "Five paths emerged from the same reading.",
		"avatars":  avatars,
	})
	return out
}
