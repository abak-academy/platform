package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"akademi-bimbel/internal/model"
)

type googleTokenInfo struct {
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
}

func (s *Service) GoogleLogin(ctx context.Context, idToken string) (accessToken string, refreshToken string, err error) {
	info, err := s.verifyGoogleToken(ctx, idToken)
	if err != nil {
		return "", "", err
	}
	if info.Aud != s.cfg.GoogleClientID || info.EmailVerified != "true" {
		return "", "", ErrInvalidToken
	}

	email := normalizeEmail(info.Email)
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}

	if user == nil {
		username, err := s.generateUniqueUsername(ctx, info.Name)
		if err != nil {
			return "", "", err
		}
		newUser := &model.User{
			Email:        &email,
			Username:     &username,
			Role:         RoleStudent,
			Name:         info.Name,
			Status:       "active",
			OTPEnabled:   false,
			AuthProvider: "google",
		}
		if err := s.repo.CreateUser(ctx, newUser); err != nil {
			return "", "", err
		}
		user = newUser
	} else if user.Role != RoleStudent {
		return "", "", ErrGoogleNotStudent
	} else if user.Status != "active" {
		return "", "", ErrAccountDeactivated
	}

	return s.mintSession(ctx, user)
}

func (s *Service) verifyGoogleToken(ctx context.Context, idToken string) (*googleTokenInfo, error) {
	endpoint := "https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + url.QueryEscape(idToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: tokeninfo status %d", ErrInvalidToken, resp.StatusCode)
	}
	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
