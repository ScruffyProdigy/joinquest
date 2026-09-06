package spiritanimal

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/scruffyprodigy/playhub/internal/tarot"
)

type questionCard struct {
	Slot                 string           `json:"slot"`
	SlotName             string           `json:"slot_name"`
	Card                 string           `json:"card"`
	CardMeaningInGeneral string           `json:"card_meaning_in_general"`
	CardMeaningForSlot   string           `json:"card_meaning_for_slot"`
	Question             string           `json:"question"`
	Answers              []questionAnswer `json:"answers"`
}

type questionAnswer struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func normalizeQuestionsJSON(raw json.RawMessage) (json.RawMessage, error) {
	var payload struct {
		Cards []json.RawMessage `json:"cards"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid questions json: %w", err)
	}
	if len(payload.Cards) != 5 {
		return nil, fmt.Errorf("expected 5 cards, got %d", len(payload.Cards))
	}

	out := make([]questionCard, 0, 5)
	for i, cardRaw := range payload.Cards {
		card, err := parseQuestionCard(cardRaw, i)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tarot.JourneySlots[i].Name, err)
		}
		out = append(out, card)
	}

	normalized, err := json.Marshal(map[string]any{"cards": out})
	if err != nil {
		return nil, err
	}
	if err := validateQuestionsJSON(normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func parseQuestionCard(raw json.RawMessage, index int) (questionCard, error) {
	if index < 0 || index >= len(tarot.JourneySlots) {
		return questionCard{}, fmt.Errorf("invalid card index %d", index)
	}
	slot := tarot.JourneySlots[index]

	var flex map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flex); err != nil {
		return questionCard{}, err
	}

	answers, err := extractAnswers(flex)
	if err != nil {
		return questionCard{}, err
	}

	card := questionCard{
		Slot:     slot.Key,
		SlotName: slot.Name,
		Answers:  answers,
	}
	card.Card = stringField(flex, "card")
	card.CardMeaningInGeneral = stringField(flex, "card_meaning_in_general")
	card.CardMeaningForSlot = stringField(flex, "card_meaning_for_slot")
	card.Question = stringField(flex, "question")
	if name := stringField(flex, "slot_name"); name != "" {
		card.SlotName = name
	}
	if card.Question == "" {
		return questionCard{}, fmt.Errorf("missing question")
	}
	return card, nil
}

func stringField(fields map[string]json.RawMessage, key string) string {
	raw, ok := fields[key]
	if !ok || len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(raw))
}

func extractAnswers(fields map[string]json.RawMessage) ([]questionAnswer, error) {
	for _, key := range []string{"answers", "options", "choices"} {
		if raw, ok := fields[key]; ok && len(raw) > 0 {
			if answers, err := parseAnswers(raw); err == nil {
				return answers, nil
			}
		}
	}
	if answers, ok := answersFromSiblingFields(fields); ok {
		return answers, nil
	}
	return nil, fmt.Errorf("expected 5 answers")
}

func answersFromSiblingFields(fields map[string]json.RawMessage) ([]questionAnswer, bool) {
	ids := []string{"A", "B", "C", "D", "E"}
	out := make([]questionAnswer, 0, 5)
	for _, id := range ids {
		for _, key := range []string{
			"answer_" + strings.ToLower(id),
			"answer" + id,
			"option_" + strings.ToLower(id),
			"choice_" + strings.ToLower(id),
		} {
			if raw, ok := fields[key]; ok {
				label := strings.TrimSpace(string(raw))
				var parsed string
				if json.Unmarshal(raw, &parsed) == nil {
					label = strings.TrimSpace(parsed)
				}
				if label != "" {
					out = append(out, questionAnswer{ID: id, Label: label})
					break
				}
			}
		}
	}
	return out, len(out) == 5
}

func parseAnswers(raw json.RawMessage) ([]questionAnswer, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing answers")
	}

	var objects []map[string]any
	if err := json.Unmarshal(raw, &objects); err == nil && len(objects) > 0 {
		return normalizeAnswerMaps(objects)
	}

	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err == nil && len(asMap) > 0 {
		return answersFromStringMap(asMap)
	}

	var labels []string
	if err := json.Unmarshal(raw, &labels); err == nil && len(labels) > 0 {
		return answersFromLabels(labels)
	}

	return nil, fmt.Errorf("expected 5 answers")
}

func normalizeAnswerMaps(objects []map[string]any) ([]questionAnswer, error) {
	ids := []string{"A", "B", "C", "D", "E"}
	out := make([]questionAnswer, 0, 5)
	for i, obj := range objects {
		label := firstString(obj, "label", "text", "option", "value", "answer")
		if label == "" {
			continue
		}
		id := strings.ToUpper(firstString(obj, "id", "key", "option"))
		if id == "" && i < len(ids) {
			id = ids[i]
		}
		if n, err := strconv.Atoi(id); err == nil && n >= 1 && n <= 5 {
			id = ids[n-1]
		}
		out = append(out, questionAnswer{ID: id, Label: label})
	}
	if len(out) != 5 {
		return nil, fmt.Errorf("expected 5 answers, got %d", len(out))
	}
	return out, nil
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := obj[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.Itoa(int(typed))
		}
	}
	return ""
}

func answersFromStringMap(asMap map[string]string) ([]questionAnswer, error) {
	ids := []string{"A", "B", "C", "D", "E"}
	out := make([]questionAnswer, 0, 5)
	for _, id := range ids {
		label, ok := asMap[id]
		if !ok {
			label, ok = asMap[strings.ToLower(id)]
		}
		if !ok || strings.TrimSpace(label) == "" {
			continue
		}
		out = append(out, questionAnswer{ID: id, Label: strings.TrimSpace(label)})
	}
	if len(out) != 5 {
		return nil, fmt.Errorf("expected 5 answers, got %d", len(out))
	}
	return out, nil
}

func answersFromLabels(labels []string) ([]questionAnswer, error) {
	if len(labels) != 5 {
		return nil, fmt.Errorf("expected 5 answers, got %d", len(labels))
	}
	ids := []string{"A", "B", "C", "D", "E"}
	out := make([]questionAnswer, 5)
	for i, label := range labels {
		out[i] = questionAnswer{ID: ids[i], Label: strings.TrimSpace(label)}
	}
	return out, nil
}
