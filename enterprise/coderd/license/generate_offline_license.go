package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/coder/coder/v2/codersdk"
)

// GenerateOfflineLicense generates a 10-year license for offline usage
func GenerateOfflineLicense() (string, ed25519.PrivateKey, ed25519.PublicKey) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    "offline-license-generator",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 10)), // 10 years
			NotBefore: jwt.NewNumericDate(now.Add(-time.Hour)),               // 1 hour ago
			IssuedAt:  jwt.NewNumericDate(now),
		},
		LicenseExpires: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 10)), // 10 years
		AccountType:   AccountTypeSalesforce,
		AccountID:     "offline-account",
		Trial:         false,
		Version:       CurrentVersion,
		AllFeatures:   true,
		Features:      Features{},
	}

	// Enable all features
	for _, featureName := range codersdk.FeatureNames {
		if featureName == codersdk.FeatureManagedAgentLimit {
			claims.Features["managed_agent_limit_soft"] = 1000000
			claims.Features["managed_agent_limit_hard"] = 2000000
		} else {
			claims.Features[featureName] = 1
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = "offline-key"
	
	licenseString, _ := token.SignedString(privateKey)
	
	return licenseString, privateKey, publicKey
}

// GenerateOfflineLicenseForTesting is a test version that can be used in tests
// Note: This function is kept for backward compatibility with existing tests
func GenerateOfflineLicenseForTesting() string {
	licenseString, _, _ := GenerateOfflineLicense()
	return licenseString
}