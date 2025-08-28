// ===============================
// internal/services/firebase.go - Centralized Firebase Service
// ===============================

package services

import (
	"context"
	"weibaobe/internal/config"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

type FirebaseService struct {
	app        *firebase.App
	authClient *auth.Client
}

// NewFirebaseService creates and initializes a new Firebase service
func NewFirebaseService(cfg *config.Config) (*FirebaseService, error) {
	// Initialize Firebase Admin SDK
	opt := option.WithCredentialsFile(cfg.FirebaseCredentials)

	firebaseApp, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: cfg.FirebaseProjectID,
	}, opt)
	if err != nil {
		return nil, err
	}

	authClient, err := firebaseApp.Auth(context.Background())
	if err != nil {
		return nil, err
	}

	return &FirebaseService{
		app:        firebaseApp,
		authClient: authClient,
	}, nil
}

// GetAuthClient returns the Firebase Auth client
func (fs *FirebaseService) GetAuthClient() *auth.Client {
	return fs.authClient
}

// VerifyIDToken verifies a Firebase ID token and returns the token claims
func (fs *FirebaseService) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return fs.authClient.VerifyIDToken(ctx, idToken)
}

// GetUser gets a Firebase user by UID
func (fs *FirebaseService) GetUser(ctx context.Context, uid string) (*auth.UserRecord, error) {
	return fs.authClient.GetUser(ctx, uid)
}
