package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"github.com/coder/coder/v2/codersdk"
)

const (
	offlineKeyID = "2022-08-12"
)

var (
	offlinePrivateKey ed25519.PrivateKey
	offlinePublicKey  ed25519.PublicKey
	OfflineKeys       map[string]ed25519.PublicKey
)

func init() {
	var err error
	offlinePublicKey, offlinePrivateKey, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	OfflineKeys = map[string]ed25519.PublicKey{
		"2022-08-12": offlinePublicKey,
	}
}

// GenerateOfflineLicense generates a 30-year license for offline usage
// userLimit: maximum number of users allowed (default 999999)
func GenerateOfflineLicense(userLimit int) (string, ed25519.PrivateKey, ed25519.PublicKey) {
	if userLimit <= 0 {
		userLimit = 999999
	}
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    "offline-license-generator",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 30)), // 30 years
			NotBefore: jwt.NewNumericDate(now.Add(-time.Hour)),                // 1 hour ago
			IssuedAt:  jwt.NewNumericDate(now),
		},
		LicenseExpires: jwt.NewNumericDate(now.Add(time.Hour * 24 * 365 * 30)), // 30 years
		AccountType:    AccountTypeSalesforce,
		AccountID:      "offline-account",
		Trial:          false,
		Version:        CurrentVersion,
		AllFeatures:    true,
		Features:       Features{},
	}

	// Enable all features
	for _, featureName := range codersdk.FeatureNames {
		if featureName == codersdk.FeatureManagedAgentLimit {
			claims.Features["managed_agent_limit_soft"] = 1000000
			claims.Features["managed_agent_limit_hard"] = 2000000
		} else if featureName == codersdk.FeatureUserLimit {
			// Set user limit to the specified value
			claims.Features[codersdk.FeatureUserLimit] = int64(userLimit)
		} else {
			claims.Features[featureName] = 1
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = offlineKeyID

	licenseString, _ := token.SignedString(offlinePrivateKey)

	return licenseString, offlinePrivateKey, offlinePublicKey
}

// GetOfflinePublicKey returns the public key used to verify offline licenses
func GetOfflinePublicKey() ed25519.PublicKey {
	return offlinePublicKey
}

// GetOfflineKeys returns a map with the offline key that can be used for parsing licenses
func GetOfflineKeys() map[string]ed25519.PublicKey {
	return map[string]ed25519.PublicKey{
		offlineKeyID: offlinePublicKey,
	}
}

// GenerateOfflineLicenseForTesting is a test version that can be used in tests
// Note: This function is kept for backward compatibility with existing tests
func GenerateOfflineLicenseForTesting() string {
	licenseString, _, _ := GenerateOfflineLicense(0) // 0 will use default 999999
	return licenseString
}

// SaveOfflinePublicKeyToFile saves the offline public key to a PEM file
func SaveOfflinePublicKeyToFile(filename string) error {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(offlinePublicKey)
	if err != nil {
		return err
	}

	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, publicKeyPEM)
}

// SaveOfflinePrivateKeyToFile saves the offline private key to a PEM file
func SaveOfflinePrivateKeyToFile(filename string) error {
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(offlinePrivateKey)
	if err != nil {
		return err
	}

	privateKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return pem.Encode(file, privateKeyPEM)
}

func GetOfflineKeyID() string {
	return offlineKeyID
}
