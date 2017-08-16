package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/convox/rack/api/crypt"
)

var (
	// bucket, key, no region
	regexpS3UrlStyle1 = regexp.MustCompile(`^https?://([^.]+).s3.amazonaws.com(/?$|/(.*))`)
	// bucket, region, key
	regexpS3UrlStyle2 = regexp.MustCompile(`^https?://([^.]+).s3-([^.]+).amazonaws.com(/?$|/(.*))`)
	// bucket, key, no region
	regexpS3UrlStyle3 = regexp.MustCompile(`^https?://s3.amazonaws.com/([^\/]+)(/?$|/(.*))`)
	// region, bucket, key
	regexpS3UrlStyle4 = regexp.MustCompile(`^https?://s3-([^.]+).amazonaws.com/([^\/]+)(/?$|/(.*))`)
)

// ParseS3Url - Parse all styles of the s3 buckets so requests
// can be made through the api.
//
// For Reference: http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html
//
// returns bucket, key, region
func ParseS3Url(url string) (string, string, string, error) {
	matches := regexpS3UrlStyle1.FindStringSubmatch(url)

	if len(matches) > 0 {
		return matches[1], matches[3], "", nil
	}

	matches = regexpS3UrlStyle2.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[1], matches[4], matches[2], nil
	}

	matches = regexpS3UrlStyle3.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[1], matches[3], "us-east-1", nil
	}

	matches = regexpS3UrlStyle4.FindStringSubmatch(url)
	if len(matches) > 0 {
		return matches[2], matches[4], matches[1], nil
	}

	return "", "", "", errors.New("not an s3 url")
}

// NewCipher - Creates a new Crypt object (a cipher) for encryption/decryption
func NewCipher() (*crypt.Crypt, error) {
	sess, err := session.NewSession()

	if err != nil {
		return nil, err
	}

	creds, err := sess.Config.Credentials.Get()
	if err != nil {
		return nil, err
	}

	return &crypt.Crypt{
		AwsRegion: *sess.Config.Region,
		AwsToken:  creds.SessionToken,
		AwsAccess: creds.AccessKeyID,
		AwsSecret: creds.SecretAccessKey,
	}, nil
}

func s3Svc() (*s3.S3, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	return s3.New(sess), nil
}

func s3GetObject(url string) ([]byte, error) {
	s3Bucket, s3Key, _, err := ParseS3Url(url)
	if err != nil {
		return nil, err
	}

	svc, err := s3Svc()
	if err != nil {
		return nil, err
	}
	input := s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3Key),
	}
	resp, err := svc.GetObject(&input)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return ioutil.ReadAll(resp.Body)
}

func s3PutObject(url string, data []byte) error {
	s3Bucket, s3Key, _, err := ParseS3Url(url)
	if err != nil {
		return err
	}

	svc, err := s3Svc()
	if err != nil {
		return err
	}

	input := s3.PutObjectInput{
		Bucket:      aws.String(s3Bucket),
		Key:         aws.String(s3Key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	}

	_, err = svc.PutObject(&input)
	return err
}

// Escapes single quotes for bash
func escapeSingleQuote(s string) string {
	return strings.Replace(s, "'", "'\"'\"'", -1)
}

func decryptEnv(url, key string, env *[]string, escape bool) error {
	if url == "" || key == "" {
		log.Debug("Not configured to load secrets")
		// Intentionally do not fail. This is not required software to run. It needs to fail silent if it's not configured on an export.
		return nil
	}

	log.WithFields(log.Fields{
		"secureEnvironmentURL": url,
	}).Debug("Attempting to load secure environment")

	if key == "" {
		log.Debug("Cannot load secrets. No SECURE_ENVIRONMENT_KEY set")
		os.Exit(1)
		return nil
	}

	data, err := s3GetObject(url)
	if err != nil {
		return err
	}

	log.Debug("Connecting to KMS")
	cipher, err := NewCipher()
	if err != nil {
		return nil
	}

	// Decrypt
	decryptedBytes, err := cipher.Decrypt(key, data)
	if err != nil {
		return err
	}

	// Process file and export the variables
	decrypted := string(decryptedBytes)

	decryptedLines := strings.Split(decrypted, "\n")

	for lineNumber, line := range decryptedLines {
		line = strings.TrimSpace(line)
		if line == "" {
			log.Debugf("Empty line: %d", lineNumber)
			continue
		}
		if !envFileLineRegex.MatchString(line) {
			log.Debugf("Invalid line: %d: %s", lineNumber, line)
			continue
		}
		if line[0] == '#' {
			log.Debug("Comment line found")
			continue
		}
		splitLine := strings.Split(line, "=")
		key := splitLine[0]
		value := strings.Join(splitLine[1:], "=")
		if escape {
			value = "'" + escapeSingleQuote(value) + "'"
		}
		*env = append(*env, fmt.Sprintf("%s=%s", key, value))
	}

	return nil
}
