package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/google/uuid"
)

func main() {
	var (
		userIDStr = flag.String("user", "", "User-UUID fuer das JWT")
		secret    = flag.String("secret", "", "JWT-Secret (default: JWT_SECRET aus Umgebung)")
		ttlMin    = flag.Int("ttl", 60, "Gueltigkeitsdauer in Minuten")
	)
	flag.Parse()

	jwtSecret := *secret
	if jwtSecret == "" {
		jwtSecret = os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "4labscloud-jwt-secret-min32chars-xyz"
		}
	}

	var userID uuid.UUID
	var err error
	if *userIDStr != "" {
		userID, err = uuid.Parse(*userIDStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ungueltige user-uuid: %v\n", err)
			os.Exit(1)
		}
	} else {
		userID = uuid.New()
	}

	token, err := auth.GenerateToken(userID, jwtSecret, time.Duration(*ttlMin)*time.Minute)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fehler beim generieren des tokens: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(token)
}
