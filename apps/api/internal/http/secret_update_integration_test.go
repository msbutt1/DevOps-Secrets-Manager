package http_test

import (
	"net/http"
	"testing"
	"time"
)

func revealValue(t *testing.T, f *fixture, secretID string) string {
	t.Helper()
	var out struct {
		Value string `json:"value"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/secrets/"+secretID+"/reveal", f.owner.Token, nil).Decode(t, &out)
	return out.Value
}

func TestUpdatingMetadataKeepsSecretValue(t *testing.T) {
	f := newFixture(t)

	// The web form omits the value when the user only edits the description.
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"description": "Primary database"})
	if got := revealValue(t, f, f.secretID); got != "postgres://fixture" {
		t.Fatalf("value changed to %q by a metadata-only update", got)
	}

	// An explicit new value, including an empty one, is still applied.
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"value": "postgres://rotated"})
	if got := revealValue(t, f, f.secretID); got != "postgres://rotated" {
		t.Fatalf("value not updated, got %q", got)
	}
}

type secretMetadata struct {
	LastUpdatedBy   string  `json:"last_updated_by"`
	LastUpdatedByID *string `json:"last_updated_by_id"`
	LastUpdatedAt   string  `json:"last_updated_at"`
	RotationPolicy  *struct {
		IntervalDays   int        `json:"interval_days"`
		LastRotatedAt  *time.Time `json:"last_rotated_at"`
		NextRotationAt time.Time  `json:"next_rotation_at"`
	} `json:"rotation_policy"`
	CreatedAt time.Time `json:"created_at"`
}

func TestSecretResponseIncludesEditorAndRotationPolicy(t *testing.T) {
	f := newFixture(t)
	editor := f.member(t, "Eddie Editor", "developer", f.vaultID, "developer")

	var created secretMetadata
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/secrets", f.owner.Token,
		map[string]any{"key_name": "ROTATED", "value": "v1", "rotation_interval_days": 30}).Decode(t, &created)
	if created.LastUpdatedBy != "Olive Owner" {
		t.Errorf("creator should be the last editor, got %q", created.LastUpdatedBy)
	}
	if created.RotationPolicy == nil || created.RotationPolicy.IntervalDays != 30 || created.RotationPolicy.LastRotatedAt != nil {
		t.Fatalf("unexpected rotation policy: %+v", created.RotationPolicy)
	}
	if want := created.CreatedAt.AddDate(0, 0, 30); !created.RotationPolicy.NextRotationAt.Equal(want) {
		t.Errorf("next rotation: want %v, got %v", want, created.RotationPolicy.NextRotationAt)
	}

	var secrets []struct {
		ID      string `json:"id"`
		KeyName string `json:"key_name"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/envs/"+f.envID+"/secrets", f.owner.Token, nil).Decode(t, &secrets)
	var id string
	for _, s := range secrets {
		if s.KeyName == "ROTATED" {
			id = s.ID
		}
	}

	var updated secretMetadata
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+id, editor.Token,
		map[string]any{"value": "v2", "rotation_interval_days": 30}).Decode(t, &updated)
	if updated.LastUpdatedBy != "Eddie Editor" || updated.LastUpdatedByID == nil || *updated.LastUpdatedByID != editor.ID.String() {
		t.Errorf("editor not recorded: %+v", updated)
	}
	if updated.RotationPolicy == nil || updated.RotationPolicy.LastRotatedAt == nil {
		t.Fatalf("rotation not recorded: %+v", updated.RotationPolicy)
	}
	if want := updated.RotationPolicy.LastRotatedAt.AddDate(0, 0, 30); !updated.RotationPolicy.NextRotationAt.Equal(want) {
		t.Errorf("next rotation after update: want %v, got %v", want, updated.RotationPolicy.NextRotationAt)
	}

	var plain secretMetadata
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"description": "no rotation"}).Decode(t, &plain)
	if plain.RotationPolicy != nil {
		t.Errorf("secret without interval should have no rotation policy, got %+v", plain.RotationPolicy)
	}
}
