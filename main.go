package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

//AwsResources is an interface that describes AWS interactions
type AwsResources interface {
	GrabSecret(secretName string) (map[string]string, error)
	SetSecretToEnvironmentVariables(secretName string) error
}

//AWS - wrapper for aws sdk session.
type AWS struct {
	Session        *session.Session
	SecretsManager *secretsmanager.SecretsManager
}

//StructureAWS - creates an instance of an AWS handler.
func StructureAWS() AwsResources {
	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion("us-east-1")))

	return AWS{
		Session:        sess,
		SecretsManager: secretsmanager.New(sess),
	}
}

//GrabSecret gets a secret from the AWS secrets manager
func (a AWS) GrabSecret(secretName string) (map[string]string, error) {
	config := make(map[string]string)

	//Create a Secrets Manager client
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	// Create a context so that the request will timeout before the Lambda does.
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	// Grab the secret
	result, err := a.SecretsManager.GetSecretValueWithContext(ctx, input)

	if err != nil {
		return config, err
	}

	// Decrypts secret using the associated KMS CMK.
	// Depending on whether the secret is a string or binary, one of these fields will be populated.
	if result.SecretString != nil {
		secretString := []byte(*result.SecretString)
		err = json.Unmarshal(secretString, &config)

		if err != nil {
			return config, err
		}
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
		_, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary)
		if err != nil {
			return config, err
		}
		err = json.Unmarshal(decodedBinarySecretBytes, &config)

		if err != nil {
			return config, err
		}
	}

	return config, nil
}

func (a AWS) SetSecretToEnvironmentVariables(secretName string) error {
	config, err := a.GrabSecret(secretName)

	if err != nil {
		return err
	}

	for key, value := range config {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return nil
}
