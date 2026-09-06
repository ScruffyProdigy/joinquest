package spiritanimal

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateQuestionsJSON(raw json.RawMessage) error {
	var payload struct {
		Cards []struct {
			Slot     string `json:"slot"`
			SlotName string `json:"slot_name"`
			Question string `json:"question"`
			Answers  []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"answers"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("invalid questions json: %w", err)
	}
	if len(payload.Cards) != 5 {
		return fmt.Errorf("expected 5 cards, got %d", len(payload.Cards))
	}
	for _, card := range payload.Cards {
		if len(card.Answers) != 5 {
			name := card.SlotName
			if name == "" {
				name = card.Slot
			}
			return fmt.Errorf("%s expected 5 answers, got %d", name, len(card.Answers))
		}
		for _, answer := range card.Answers {
			if looksLikeQuestion(answer.Label) {
				name := card.SlotName
				if name == "" {
					name = card.Slot
				}
				return fmt.Errorf("%s answer %s looks like a question, not a choice", name, answer.ID)
			}
		}
		if msg := answersTooSimilar(card.Answers); msg != "" {
			name := card.SlotName
			if name == "" {
				name = card.Slot
			}
			return fmt.Errorf("%s answers lack contrast: %s", name, msg)
		}
	}
	return nil
}

func validatePersonalityJSON(raw json.RawMessage) error {
	var payload struct {
		Overview       string            `json:"overview"`
		JourneySummary map[string]string `json:"journey_summary"`
		AvatarSignals  map[string]any    `json:"avatar_signals"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("invalid personality json: %w", err)
	}
	if payload.Overview == "" || payload.AvatarSignals == nil {
		return fmt.Errorf("personality json missing required fields")
	}
	return nil
}

func validateTotemsJSON(raw json.RawMessage) error {
	var payload struct {
		Totems []struct {
			Name        string `json:"name"`
			Animal      string `json:"animal"`
			Pose        string `json:"pose"`
			Expression  string `json:"expression"`
			ImagePrompt string `json:"image_prompt"`
		} `json:"totems"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("invalid totems json: %w", err)
	}
	if len(payload.Totems) != 5 {
		return fmt.Errorf("expected 5 totems, got %d", len(payload.Totems))
	}
	for _, totem := range payload.Totems {
		if totem.Name == "" || totem.ImagePrompt == "" {
			return fmt.Errorf("totem missing name or image_prompt")
		}
		if strings.TrimSpace(totem.Pose) == "" {
			return fmt.Errorf("totem %q missing pose", totem.Name)
		}
		if strings.TrimSpace(totem.Expression) == "" {
			return fmt.Errorf("totem %q missing expression", totem.Name)
		}
		if !isLikelyAnimalSpecies(totem.Animal) {
			return fmt.Errorf("totem %q must have a recognizable animal species, got %q", totem.Name, totem.Animal)
		}
	}
	return nil
}

// ValidateSubmittedAnswers checks answer IDs against the stored question set.
func ValidateSubmittedAnswers(questionsJSON json.RawMessage, answers []string) error {
	return validateSubmittedAnswers(questionsJSON, answers)
}

func validateSubmittedAnswers(questionsJSON json.RawMessage, answers []string) error {
	var payload struct {
		Cards []struct {
			Answers []struct {
				ID string `json:"id"`
			} `json:"answers"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(questionsJSON, &payload); err != nil {
		return fmt.Errorf("invalid questions json: %w", err)
	}
	if len(payload.Cards) != len(answers) {
		return fmt.Errorf("invalid answer count: expected %d, got %d", len(payload.Cards), len(answers))
	}
	for i, card := range payload.Cards {
		valid := map[string]struct{}{}
		for _, answer := range card.Answers {
			valid[strings.ToUpper(strings.TrimSpace(answer.ID))] = struct{}{}
		}
		choice := strings.ToUpper(strings.TrimSpace(answers[i]))
		if _, ok := valid[choice]; !ok {
			return fmt.Errorf("invalid answer %q for card %d", answers[i], i+1)
		}
	}
	return nil
}

func validateRankingJSON(raw json.RawMessage) error {
	var payload struct {
		Overview string `json:"overview"`
		Avatars  []struct {
			Name     string `json:"name"`
			FitScore int    `json:"fit_score"`
		} `json:"avatars"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("invalid ranking json: %w", err)
	}
	if len(payload.Avatars) != 5 {
		return fmt.Errorf("expected 5 ranked avatars, got %d", len(payload.Avatars))
	}
	return nil
}
