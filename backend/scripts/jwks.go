package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

const yamlFormat = `apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  KID: %s
  PUB_X: %s
  PRIV_PEM: |
%s`

func main() {
	// Flags
	name := flag.String("name", "jwks", "Kubernetes Secret name")
	ns := flag.String("namespace", "playhub", "Kubernetes namespace")
	kid := flag.String("kid", "lobby-dev", "JWKS key id (kid)")
	yamlOut := flag.String("yamlout", "", "Write Secret YAML to file (default stdout)")
	jwksOut := flag.String("jwksout", "", "Write Public JSON to file (default stdout)")
	flag.Parse()

	pubX, privPEM, err := genKeyPair()
	if err != nil {
		os.Exit(1)
	}

	// Build Secret YAML (uses stringData so you can read it in env vars)
	yaml := fmt.Sprintf(yamlFormat, *name, *ns, *kid, pubX, indent(privPEM, "    "))
	err = writeToFile(yamlOut, yaml)
	if err != nil {
		os.Exit(1)
	}

	jwks, _ := json.MarshalIndent(map[string]any{
		"keys": []map[string]string{
			{
				"kty": "OKP",
				"crv": "Ed25519",
				"use": "sig",
				"alg": "EdDSA",
				"kid": *kid,
				"x":   pubX,
			},
		},
		// Not part of spec, but handy for humans:
		"_generated": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	err = writeToFile(jwksOut, string(jwks))
	if err != nil {
		os.Exit(1)
	}

	// Helpful reminder
	fmt.Fprintln(os.Stderr, "\nNOTE: Keep PRIV_PEM secret. PUB_X and KID are safe to expose in JWKS.")
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func genKeyPair() (string, string, error) {
	// Generate Ed25519 keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "keygen error: %v\n", err)
		return "", "", err
	}

	// Public key (x) for JWKS
	pubX := b64url(pub)

	// Private key (PKCS#8) in PEM for signing
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		return "", "", err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	// Make sure PEM ends with newline
	if !strings.HasSuffix(string(privPEM), "\n") {
		privPEM = append(privPEM, '\n')
	}

	return pubX, string(privPEM), nil
}

func writeToFile(out *string, data string) error {
	if *out != "" {
		if err := os.WriteFile(*out, []byte(data), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
	} else {
		fmt.Println(data)
	}
	return nil
}

func indent(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ln != "" {
			lines[i] = pad + ln
		}
	}
	return strings.Join(lines, "\n")
}
