// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

// Command e2etools prepares emulator fixtures for scripts/e2e.sh. It is a
// separate module so the MinIO admin client never enters the provider's
// dependency graph.
//
//	e2etools azurite-container <container>                    (AZURE_STORAGE_CONNECTION_STRING)
//	e2etools s3-versioning <endpoint> <bucket>                (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY)
//	e2etools minio-user <endpoint> <user> <secret> <policy.json> (root credentials from AWS_* env)
//	e2etools s3-put-denied <endpoint> <bucket> <key>...      (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY)
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/minio/madmin-go/v3"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := run(ctx, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "e2etools: %v\n", err)

		cancel()
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing subcommand")
	}

	switch {
	case args[0] == "azurite-container" && len(args) == 2:
		return azuriteContainer(ctx, args[1])
	case args[0] == "s3-versioning" && len(args) == 3:
		return s3Versioning(ctx, args[1], args[2])
	case args[0] == "minio-user" && len(args) == 5:
		return minioUser(ctx, args[1], args[2], args[3], args[4])
	case args[0] == "s3-put-denied" && len(args) >= 4:
		return s3PutDenied(ctx, args[1], args[2], args[3:])
	default:
		return fmt.Errorf("bad usage: %v", args)
	}
}

func azuriteContainer(ctx context.Context, name string) error {
	client, err := azblob.NewClientFromConnectionString(os.Getenv("AZURE_STORAGE_CONNECTION_STRING"), nil)
	if err != nil {
		return err
	}

	_, err = client.CreateContainer(ctx, name, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return err
	}

	fmt.Printf("azurite container %s ready\n", name)

	return nil
}

func s3Client(endpoint string) *s3.Client {
	return s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(endpoint),
		UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(
			os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
	})
}

func s3Versioning(ctx context.Context, endpoint, bucket string) error {
	client := s3Client(endpoint)

	_, err := client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket:                  aws.String(bucket),
		VersioningConfiguration: &s3types.VersioningConfiguration{Status: s3types.BucketVersioningStatusEnabled},
	})
	if err != nil {
		return err
	}

	out, err := client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		return err
	}

	if out.Status != s3types.BucketVersioningStatusEnabled {
		return fmt.Errorf("bucket %s versioning is %q after enabling", bucket, out.Status)
	}

	fmt.Printf("s3 bucket %s versioning Enabled\n", bucket)

	return nil
}

func minioUser(ctx context.Context, endpoint, user, secret, policyPath string) error {
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		return err
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	admin, err := madmin.NewWithOptions(u.Host, &madmin.Options{
		Creds:  miniocreds.NewStaticV4(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), ""),
		Secure: u.Scheme == "https",
	})
	if err != nil {
		return err
	}

	policyName := user + "-policy"

	if err := admin.AddCannedPolicy(ctx, policyName, policy); err != nil {
		return fmt.Errorf("add policy: %w", err)
	}

	if err := admin.AddUser(ctx, user, secret); err != nil {
		return fmt.Errorf("add user: %w", err)
	}

	if _, err := admin.AttachPolicy(ctx, madmin.PolicyAssociationReq{Policies: []string{policyName}, User: user}); err != nil {
		return fmt.Errorf("attach policy: %w", err)
	}

	fmt.Printf("minio user %s created with policy %s\n", user, policyName)

	return nil
}

// s3PutDenied requires that the calling identity can read each existing key
// but is refused a direct write to it with 403 AccessDenied. The write is
// conditional on the key being absent, so if the policy wrongly allows it the
// existing document fails the precondition (412) instead of being replaced;
// authorization is evaluated before preconditions.
func s3PutDenied(ctx context.Context, endpoint, bucket string, keys []string) error {
	client := s3Client(endpoint)

	var failed []string

	for _, key := range keys {
		// The Head is load-bearing: MinIO also answers a put signed with a wrong
		// secret with 403 AccessDenied, so only a successful Head proves the
		// credentials are valid and the later 403 is an IAM denial.
		if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}); err != nil {
			return fmt.Errorf("head %s: the document must exist before its write is probed: %w", key, err)
		}

		_, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(key),
			Body:        strings.NewReader(`{"e2e":"restricted write probe"}`),
			IfNoneMatch: aws.String("*"),
		})

		status, code := denial(err)
		if status != http.StatusForbidden || code != "AccessDenied" {
			failed = append(failed, fmt.Sprintf("put %s: got status %d code %q (err %v), want 403 AccessDenied", key, status, code, err))

			continue
		}

		fmt.Printf("put %s: denied (%d %s)\n", key, status, code)
	}

	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "; "))
	}

	return nil
}

// denial returns the HTTP status and S3 error code of err; 200 when err is nil.
func denial(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}

	status, code := 0, ""

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		status = respErr.HTTPStatusCode()
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code = apiErr.ErrorCode()
	}

	return status, code
}
