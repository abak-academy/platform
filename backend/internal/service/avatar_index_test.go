package service

import (
	"context"
	"errors"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestUpdateAvatarIndex(t *testing.T) {
	ptr := func(i int) *int { return &i }
	tests := []struct {
		name    string
		idx     *int
		wantErr bool
	}{
		{"lowest index", ptr(0), false},
		{"highest index", ptr(34), false},
		{"one past the sprite", ptr(35), true},
		{"negative", ptr(-1), true},
		{"nil clears", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeUserRepo()
			repo.seed(&model.User{ID: "u1", Name: "Peserta A", AvatarIndex: ptr(7)})
			svc, _ := newTestService(t, repo)

			got, err := svc.UpdateAvatarIndex(context.Background(), "u1", tt.idx)
			if tt.wantErr {
				if !errors.Is(err, ErrValidation) {
					t.Fatalf("UpdateAvatarIndex() err = %v, want ErrValidation", err)
				}
				if repo.byID["u1"].AvatarIndex == nil || *repo.byID["u1"].AvatarIndex != 7 {
					t.Fatalf("stored avatar_index changed on rejected input: %v", repo.byID["u1"].AvatarIndex)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateAvatarIndex() err = %v", err)
			}
			if (got.AvatarIndex == nil) != (tt.idx == nil) || (tt.idx != nil && *got.AvatarIndex != *tt.idx) {
				t.Fatalf("avatar_index = %v, want %v", got.AvatarIndex, tt.idx)
			}
		})
	}
}
