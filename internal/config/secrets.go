package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// ResolvePassword sets c.Password from MYSQL_PASSWORD or AWS Secrets Manager.
// Local MYSQL_PASSWORD always wins so the harness can run without AWS.
func (c *Config) ResolvePassword(ctx context.Context) error {
	if c.HasPassword() {
		return nil
	}
	if c.SecretARN == "" {
		return fmt.Errorf("no database password: set MYSQL_PASSWORD for local/dev, or MYSQL_SECRET_ARN to fetch from AWS Secrets Manager")
	}
	pw, err := fetchSecretPassword(ctx, c.AWSRegion, c.SecretARN)
	if err != nil {
		return err
	}
	c.Password = pw
	return nil
}

func fetchSecretPassword(ctx context.Context, region, arn string) (string, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("aws config (region %s): %w", region, err)
	}
	out, err := secretsmanager.NewFromConfig(cfg).GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		return "", fmt.Errorf("secrets manager GetSecretValue %s: %w", arn, err)
	}
	if out.SecretString == nil || *out.SecretString == "" {
		return "", fmt.Errorf("secret %s has empty SecretString", arn)
	}
	pw, err := passwordFromSecretString(*out.SecretString)
	if err != nil {
		return "", fmt.Errorf("secret %s: %w", arn, err)
	}
	return pw, nil
}

func passwordFromSecretString(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty secret string")
	}
	if s[0] != '{' {
		return s, nil
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return "", fmt.Errorf("parse secret JSON: %w", err)
	}
	for _, key := range []string{"password", "Password", "PASSWORD"} {
		if v, ok := obj[key]; ok {
			if pw, ok := v.(string); ok && pw != "" {
				return pw, nil
			}
		}
	}
	return "", fmt.Errorf("JSON secret has no password field (looked for password/Password/PASSWORD)")
}
