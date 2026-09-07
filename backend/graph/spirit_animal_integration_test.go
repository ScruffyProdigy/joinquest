package graph

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scruffyprodigy/joinquest/internal/store"
)

func TestSpiritAnimalReadingFlow(t *testing.T) {
	env := newQueueIntegrationEnv(t)
	cleaner := env.newCleaner(t)
	ctx := context.Background()

	t.Setenv("LOBBY_PUBLIC_URL", "https://joinquest.test")

	user, err := env.Store.CreateUser(ctx, store.CreateUserParams{
		Email: "spirit-" + uuid.NewString() + "@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cleaner.TrackUser(user.ID)
	_, cookie := createTestUserSessionForUser(t, env, user.ID)

	beginQuery := `mutation { beginSpiritAnimalReading { id status draw } }`
	beginBody := postGraphQL(t, env.Handler, beginQuery, nil, cookie)
	var beginResp struct {
		Data struct {
			BeginSpiritAnimalReading struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Draw   []int  `json:"draw"`
			} `json:"beginSpiritAnimalReading"`
		} `json:"data"`
	}
	if err := json.Unmarshal(beginBody, &beginResp); err != nil {
		t.Fatalf("decode begin: %v body=%s", err, beginBody)
	}
	if len(beginResp.Data.BeginSpiritAnimalReading.Draw) != 5 {
		t.Fatalf("expected draw of 5 cards")
	}

	statusQuery := `query { mySpiritAnimalReading { status cardQuestions { question answers { id label } } } }`
	var status string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		body := postGraphQL(t, env.Handler, statusQuery, nil, cookie)
		var resp struct {
			Data struct {
				MySpiritAnimalReading *struct {
					Status        string `json:"status"`
					CardQuestions []struct {
						Question string `json:"question"`
						Answers  []struct {
							ID string `json:"id"`
						} `json:"answers"`
					} `json:"cardQuestions"`
				} `json:"mySpiritAnimalReading"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if resp.Data.MySpiritAnimalReading != nil {
			status = resp.Data.MySpiritAnimalReading.Status
			if status == "AWAITING_ANSWERS" && len(resp.Data.MySpiritAnimalReading.CardQuestions) == 5 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "AWAITING_ANSWERS" {
		t.Fatalf("expected AWAITING_ANSWERS, got %q", status)
	}

	submitQuery := `mutation Submit($answers: [ID!]!) {
		submitSpiritAnimalAnswers(answers: $answers) { status }
	}`
	postGraphQL(t, env.Handler, submitQuery, map[string]any{
		"answers": []string{"A", "B", "C", "D", "E"},
	}, cookie)

	deadline = time.Now().Add(10 * time.Second)
	var totemName string
	for time.Now().Before(deadline) {
		body := postGraphQL(t, env.Handler, `query {
			mySpiritAnimalReading {
				status
				totems { name imageUrl }
			}
		}`, nil, cookie)
		var resp struct {
			Data struct {
				MySpiritAnimalReading *struct {
					Status string `json:"status"`
					Totems []struct {
						Name     string  `json:"name"`
						ImageURL *string `json:"imageUrl"`
					} `json:"totems"`
				} `json:"mySpiritAnimalReading"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatalf("decode ready: %v", err)
		}
		reading := resp.Data.MySpiritAnimalReading
		if reading != nil && reading.Status == "READY" && len(reading.Totems) == 5 {
			totemName = reading.Totems[0].Name
			if reading.Totems[0].ImageURL == nil {
				t.Fatal("expected imageUrl on totem")
			}
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if totemName == "" {
		t.Fatal("reading never became READY with totems")
	}

	selectQuery := `mutation Select($name: String!) {
		selectSpiritAnimalTotem(totemName: $name) {
			avatarSource
			avatarUrl
		}
	}`
	selectBody := postGraphQL(t, env.Handler, selectQuery, map[string]any{"name": totemName}, cookie)
	var selectResp struct {
		Data struct {
			SelectSpiritAnimalTotem struct {
				AvatarSource string  `json:"avatarSource"`
				AvatarURL    *string `json:"avatarUrl"`
			} `json:"selectSpiritAnimalTotem"`
		} `json:"data"`
	}
	if err := json.Unmarshal(selectBody, &selectResp); err != nil {
		t.Fatalf("decode select: %v body=%s", err, selectBody)
	}
	if selectResp.Data.SelectSpiritAnimalTotem.AvatarSource != "SPIRIT_ANIMAL" {
		t.Fatalf("avatar source: %+v", selectResp.Data.SelectSpiritAnimalTotem.AvatarSource)
	}
	if selectResp.Data.SelectSpiritAnimalTotem.AvatarURL == nil {
		t.Fatal("expected avatarUrl")
	}

	eligibilityBody := postGraphQL(t, env.Handler, `query {
		mySpiritAnimalJourneyEligibility { canBegin daysRemaining }
	}`, nil, cookie)
	var eligibilityResp struct {
		Data struct {
			MySpiritAnimalJourneyEligibility struct {
				CanBegin      bool `json:"canBegin"`
				DaysRemaining *int `json:"daysRemaining"`
			} `json:"mySpiritAnimalJourneyEligibility"`
		} `json:"data"`
	}
	if err := json.Unmarshal(eligibilityBody, &eligibilityResp); err != nil {
		t.Fatalf("decode eligibility: %v body=%s", err, eligibilityBody)
	}
	if eligibilityResp.Data.MySpiritAnimalJourneyEligibility.CanBegin {
		t.Fatal("expected journey cooldown after completion")
	}
	if eligibilityResp.Data.MySpiritAnimalJourneyEligibility.DaysRemaining == nil || *eligibilityResp.Data.MySpiritAnimalJourneyEligibility.DaysRemaining <= 0 {
		t.Fatalf("expected positive daysRemaining, got %+v", eligibilityResp.Data.MySpiritAnimalJourneyEligibility.DaysRemaining)
	}

	retryBody := postGraphQL(t, env.Handler, beginQuery, nil, cookie)
	var retryResp struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(retryBody, &retryResp); err != nil {
		t.Fatalf("decode retry begin: %v body=%s", err, retryBody)
	}
	if len(retryResp.Errors) == 0 {
		t.Fatalf("expected begin to fail during cooldown, got %s", retryBody)
	}
}
