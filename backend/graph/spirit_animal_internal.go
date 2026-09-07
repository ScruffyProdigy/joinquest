package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/graph/model"
	"github.com/scruffyprodigy/joinquest/internal/auth"
	"github.com/scruffyprodigy/joinquest/internal/spiritanimal"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func (r *Resolver) requireSpiritAnimalRunner() (*spiritanimal.Runner, error) {
	if r == nil || r.SpiritAnimal == nil {
		return nil, fmt.Errorf("spirit animal service is not configured")
	}
	return r.SpiritAnimal, nil
}

func spiritAnimalTotemImageURL(reading *store.AvatarReading, totemName string) (string, error) {
	var payload struct {
		Totems []struct {
			Name     string `json:"name"`
			ImageURL string `json:"image_url"`
		} `json:"totems"`
	}
	if err := json.Unmarshal(reading.TotemsJSON, &payload); err != nil {
		return "", err
	}
	want := strings.TrimSpace(totemName)
	for _, totem := range payload.Totems {
		if strings.EqualFold(totem.Name, want) && strings.TrimSpace(totem.ImageURL) != "" {
			return normalizeSpiritAvatarURL(totem.ImageURL), nil
		}
	}
	return "", fmt.Errorf("totem not found")
}

func toGraphQLSpiritAnimalReadingWithOptions(reading *store.AvatarReading, imagesMissing bool) (*model.SpiritAnimalReading, error) {
	if reading == nil {
		return nil, nil
	}
	out := &model.SpiritAnimalReading{
		ID:            reading.ID.String(),
		Status:        toGraphQLReadingStatus(reading.Status),
		Draw:          int32sToInts(reading.Draw),
		ImagesMissing: imagesMissing,
	}
	if reading.Status == store.ReadingStatusGeneratingQuestions || reading.Status == store.ReadingStatusProcessing {
		started := reading.UpdatedAt
		if reading.PhaseStartedAt != nil {
			started = *reading.PhaseStartedAt
		}
		out.PhaseStartedAt = &started
		estimate := spiritanimal.EstimateSecondsForStatus(reading.Status)
		out.EstimatedPhaseSeconds = &estimate
	}
	if reading.SelectedTotemName != nil {
		out.SelectedTotemName = reading.SelectedTotemName
	}
	if reading.ErrorMessage != nil {
		out.ErrorMessage = reading.ErrorMessage
	}
	if len(reading.QuestionsJSON) > 0 {
		out.CardQuestions = parseCardQuestions(reading.QuestionsJSON)
	}
	if len(reading.PersonalityJSON) > 0 {
		out.Personality = parsePersonality(reading.PersonalityJSON)
	}
	if len(reading.TotemsJSON) > 0 || len(reading.RankingJSON) > 0 {
		out.Totems = mergeTotems(reading.TotemsJSON, reading.RankingJSON)
	}
	if len(reading.RankingJSON) > 0 {
		var ranking struct {
			Overview string `json:"overview"`
		}
		_ = json.Unmarshal(reading.RankingJSON, &ranking)
		if ranking.Overview != "" {
			out.MascotOverview = &ranking.Overview
		}
	}
	return out, nil
}

func spiritAnimalJourneyEligibility(ctx context.Context, st *store.Store, user *store.User) (*model.SpiritAnimalJourneyEligibility, error) {
	lastCompleted, err := st.GetLastCompletedSpiritAnimalAt(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	eligibility := spiritanimal.JourneyEligibilityAt(lastCompleted, time.Now(), auth.IsAdminEmail(user.Email))
	out := &model.SpiritAnimalJourneyEligibility{
		CanBegin: eligibility.CanBegin,
	}
	if eligibility.CooldownEndsAt != nil {
		out.CooldownEndsAt = eligibility.CooldownEndsAt
	}
	if eligibility.DaysRemaining > 0 {
		days := eligibility.DaysRemaining
		out.DaysRemaining = &days
	}
	return out, nil
}

func ensureSpiritAnimalStartAllowed(ctx context.Context, st *store.Store, userID uuid.UUID) error {
	count, err := st.CountAvatarReadingsSince(ctx, userID, time.Now().Add(-spiritanimal.ReadingStartWindow()))
	if err != nil {
		return err
	}
	if count >= spiritanimal.MaxReadingStartsPerHour {
		return fmt.Errorf("Too many reading starts — wait a little while, then try again")
	}
	return nil
}

func ensureSpiritAnimalJourneyAllowed(ctx context.Context, st *store.Store, user *store.User) error {
	eligibility, err := spiritAnimalJourneyEligibility(ctx, st, user)
	if err != nil {
		return err
	}
	if eligibility.CanBegin {
		return nil
	}
	days := 0
	if eligibility.DaysRemaining != nil {
		days = *eligibility.DaysRemaining
	}
	if days <= 0 {
		days = spiritanimal.JourneyCooldownDays
	}
	return fmt.Errorf("Your next spirit animal journey opens in %d days", days)
}

func normalizeSpiritAvatarURL(raw string) string {
	url := strings.TrimSpace(raw)
	if url == "" {
		return ""
	}
	if strings.HasPrefix(url, "/spirit-avatars/") {
		return url
	}
	if idx := strings.Index(url, "/spirit-avatars/"); idx >= 0 {
		return url[idx:]
	}
	return ""
}

func graphQLSpiritAnimalReading(runner *spiritanimal.Runner, reading *store.AvatarReading) (*model.SpiritAnimalReading, error) {
	if reading == nil {
		return nil, nil
	}
	imagesMissing := false
	if runner != nil {
		imagesMissing = runner.ImagesMissingForReading(reading)
	}
	return toGraphQLSpiritAnimalReadingWithOptions(reading, imagesMissing)
}

func toGraphQLReadingStatus(status string) model.SpiritAnimalReadingStatus {
	switch status {
	case store.ReadingStatusGeneratingQuestions:
		return model.SpiritAnimalReadingStatusGeneratingQuestions
	case store.ReadingStatusAwaitingAnswers:
		return model.SpiritAnimalReadingStatusAwaitingAnswers
	case store.ReadingStatusProcessing:
		return model.SpiritAnimalReadingStatusProcessing
	case store.ReadingStatusReady:
		return model.SpiritAnimalReadingStatusReady
	case store.ReadingStatusCompleted:
		return model.SpiritAnimalReadingStatusCompleted
	case store.ReadingStatusFailed:
		return model.SpiritAnimalReadingStatusFailed
	default:
		return model.SpiritAnimalReadingStatusFailed
	}
}

func int32sToInts(in []int32) []int {
	out := make([]int, len(in))
	for i, n := range in {
		out[i] = int(n)
	}
	return out
}

func parseCardQuestions(raw json.RawMessage) []*model.SpiritAnimalCardQuestion {
	var payload struct {
		Cards []struct {
			Slot                 string `json:"slot"`
			SlotName             string `json:"slot_name"`
			Card                 string `json:"card"`
			CardMeaningInGeneral string `json:"card_meaning_in_general"`
			CardMeaningForSlot   string `json:"card_meaning_for_slot"`
			Question             string `json:"question"`
			Answers              []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"answers"`
		} `json:"cards"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	out := make([]*model.SpiritAnimalCardQuestion, 0, len(payload.Cards))
	for _, card := range payload.Cards {
		answers := make([]*model.SpiritAnimalAnswer, 0, len(card.Answers))
		for _, answer := range card.Answers {
			answers = append(answers, &model.SpiritAnimalAnswer{
				ID:    answer.ID,
				Label: answer.Label,
			})
		}
		out = append(out, &model.SpiritAnimalCardQuestion{
			Slot:                 card.Slot,
			SlotName:             card.SlotName,
			Card:                 card.Card,
			CardMeaningInGeneral: card.CardMeaningInGeneral,
			CardMeaningForSlot:   card.CardMeaningForSlot,
			Question:             card.Question,
			Answers:              answers,
		})
	}
	return out
}

func parsePersonality(raw json.RawMessage) *model.SpiritAnimalPersonality {
	var payload struct {
		Overview       string            `json:"overview"`
		JourneySummary map[string]string `json:"journey_summary"`
		CoreThemes     []string          `json:"core_themes"`
		Strengths      []string          `json:"strengths"`
		Tensions       []string          `json:"tensions"`
		SocialIdentity string            `json:"social_identity"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return &model.SpiritAnimalPersonality{
		Overview: payload.Overview,
		JourneySummary: &model.SpiritAnimalJourneySummary{
			Compass:  payload.JourneySummary["compass"],
			Coin:     payload.JourneySummary["coin"],
			Storm:    payload.JourneySummary["storm"],
			Campfire: payload.JourneySummary["campfire"],
			Beacon:   payload.JourneySummary["beacon"],
		},
		CoreThemes:     payload.CoreThemes,
		Strengths:      payload.Strengths,
		Tensions:       payload.Tensions,
		SocialIdentity: payload.SocialIdentity,
	}
}

func mergeTotems(totemsRaw, rankingRaw json.RawMessage) []*model.SpiritAnimalTotem {
	var totemsPayload struct {
		Totems []struct {
			Name               string   `json:"name"`
			Animal             string   `json:"animal"`
			SocialArchetype    string   `json:"social_archetype"`
			CoreConcept        string   `json:"core_concept"`
			ColorPalette       []string `json:"color_palette"`
			PersonalitySummary string   `json:"personality_summary"`
			WhyThisAnimal      string   `json:"why_this_animal"`
			OriginStory        string   `json:"origin_story"`
			ImageURL           string   `json:"image_url"`
		} `json:"totems"`
	}
	if err := json.Unmarshal(totemsRaw, &totemsPayload); err != nil {
		return nil
	}

	rankByName := map[string]struct {
		FitScore                int
		Affinity                string
		ReadingEmphasis         string
		WhyThisAnimalMakesSense string
		WhyChooseThisAvatar     string
	}{}
	if len(rankingRaw) > 0 {
		var ranking struct {
			Avatars []struct {
				Name                             string `json:"name"`
				FitScore                         int    `json:"fit_score"`
				Affinity                         string `json:"affinity"`
				WhatPartOfTheReadingItEmphasizes string `json:"what_part_of_the_reading_it_emphasizes"`
				WhyThisAnimalMakesSense          string `json:"why_this_animal_makes_sense"`
				WhySomeoneMightChooseThisAvatar  string `json:"why_someone_might_choose_this_avatar"`
			} `json:"avatars"`
		}
		if err := json.Unmarshal(rankingRaw, &ranking); err == nil {
			for _, avatar := range ranking.Avatars {
				rankByName[strings.ToLower(avatar.Name)] = struct {
					FitScore                int
					Affinity                string
					ReadingEmphasis         string
					WhyThisAnimalMakesSense string
					WhyChooseThisAvatar     string
				}{
					FitScore:                avatar.FitScore,
					Affinity:                avatar.Affinity,
					ReadingEmphasis:         avatar.WhatPartOfTheReadingItEmphasizes,
					WhyThisAnimalMakesSense: avatar.WhyThisAnimalMakesSense,
					WhyChooseThisAvatar:     avatar.WhySomeoneMightChooseThisAvatar,
				}
			}
		}
	}

	out := make([]*model.SpiritAnimalTotem, 0, len(totemsPayload.Totems))
	for _, totem := range totemsPayload.Totems {
		item := &model.SpiritAnimalTotem{
			Name:               totem.Name,
			Animal:             totem.Animal,
			SocialArchetype:    totem.SocialArchetype,
			CoreConcept:        totem.CoreConcept,
			ColorPalette:       totem.ColorPalette,
			PersonalitySummary: totem.PersonalitySummary,
			WhyThisAnimal:      totem.WhyThisAnimal,
			OriginStory:        totem.OriginStory,
		}
		if totem.ImageURL != "" {
			url := normalizeSpiritAvatarURL(totem.ImageURL)
			item.ImageURL = &url
		}
		if rank, ok := rankByName[strings.ToLower(totem.Name)]; ok {
			score := rank.FitScore
			item.FitScore = &score
			if rank.Affinity != "" {
				affinity := rank.Affinity
				item.Affinity = &affinity
			}
			if rank.ReadingEmphasis != "" {
				emphasis := rank.ReadingEmphasis
				item.ReadingEmphasis = &emphasis
			}
			if rank.WhyThisAnimalMakesSense != "" {
				v := rank.WhyThisAnimalMakesSense
				item.WhyThisAnimalMakesSense = &v
			}
			if rank.WhyChooseThisAvatar != "" {
				v := rank.WhyChooseThisAvatar
				item.WhyChooseThisAvatar = &v
			}
		}
		out = append(out, item)
	}
	return out
}
