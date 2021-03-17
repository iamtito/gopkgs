package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/sqs"
)

//AwsInterface is an interface that describes AWS interactions
type AwsInterface interface {
	GetSecret(secretName string) (map[string]string, error)
	GetSingleSecret(secretName string, value string) (string, error)
	SetSecretToEnvironmentVariables(secretName string) error
	GetQueueUrl(v string) string
	SendStringMessageToSqs(payload string, qURL string) (*string, error)
	SendStringMessageWithAttributesToSqs(payload string, qURL string, attributes map[string]interface{}) (*string, error)
}

//AWS is a wrapper for a real aws sdk session.
type AWS struct {
	Session        *session.Session
	SecretsManager *secretsmanager.SecretsManager
}

func ConstructAWS() AwsInterface {
	sess := session.Must(session.NewSession(aws.NewConfig().WithRegion("us-east-1")))

	return AWS{
		Session:        sess,
		SecretsManager: secretsmanager.New(sess),
	}
}

func (a AWS) GetSecret(secretName string) (map[string]string, error) {
	config := make(map[string]string)
	// secretName := "/deployment/qalort/staging"
	// region := "us-east-1"
	/// Incoming changes
	// Create a context so that the request will timeout before the Lambda does.
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	/////// Incoming ends

	// //Create a Secrets Manager client
	// svc := secretsmanager.New(session.New(),
	// 	aws.NewConfig().WithRegion(region))

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	// In this sample we only handle the specific exceptions for the 'GetSecretValue' API.
	// See https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html

	// result, err := svc.GetSecretValue(input)
	// if err != nil {
	// 	if aerr, ok := err.(awserr.Error); ok {
	// 		switch aerr.Code() {
	// 		case secretsmanager.ErrCodeDecryptionFailure:
	// 			// Secrets Manager can't decrypt the protected secret text using the provided KMS key.
	// 			fmt.Println(secretsmanager.ErrCodeDecryptionFailure, aerr.Error())

	// 		case secretsmanager.ErrCodeInternalServiceError:
	// 			// An error occurred on the server side.
	// 			fmt.Println(secretsmanager.ErrCodeInternalServiceError, aerr.Error())

	// 		case secretsmanager.ErrCodeInvalidParameterException:
	// 			// You provided an invalid value for a parameter.
	// 			fmt.Println(secretsmanager.ErrCodeInvalidParameterException, aerr.Error())

	// 		case secretsmanager.ErrCodeInvalidRequestException:
	// 			// You provided a parameter value that is not valid for the current state of the resource.
	// 			fmt.Println(secretsmanager.ErrCodeInvalidRequestException, aerr.Error())

	// 		case secretsmanager.ErrCodeResourceNotFoundException:
	// 			// We can't find the resource that you asked for.
	// 			fmt.Println(secretsmanager.ErrCodeResourceNotFoundException, aerr.Error())
	// 		}
	// 	} else {
	// 		// Print the error, cast err to awserr.Error to get the Code and
	// 		// Message from an error.
	// 		fmt.Println(err.Error())
	// 	}
	// 	// return nil
	// }
	// Grab the secret
	result, err := a.SecretsManager.GetSecretValueWithContext(ctx, input)

	if err != nil {
		return config, err
	}

	// fmt.Println("First printing is here", result)

	// Decrypts secret using the associated KMS CMK.
	// Depending on whether the secret is a string or binary, one of these fields will be populated.
	var secretString, decodedBinarySecret string
	if result.SecretString != nil {
		secretString = *result.SecretString
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
		len, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary)
		if err != nil {
			fmt.Println("Base64 Decode Error:", err)
			// return nil
		}
		decodedBinarySecret = string(decodedBinarySecretBytes[:len])
		fmt.Println(decodedBinarySecret)
	}

	json.Unmarshal([]byte(secretString), &config)
	return config, nil

}

//GetSingleSecret obtains a single secret from secretmanager without having to fetch all the secrets
func (a AWS) GetSingleSecret(secretName string, value string) (string, error) {
	config := make(map[string]string)
	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := a.SecretsManager.GetSecretValueWithContext(ctx, input)

	if err != nil {
		return config[value], err
	}

	// Decrypts secret using the associated KMS CMK.
	// Depending on whether the secret is a string or binary, one of these fields will be populated.
	var secretString, decodedBinarySecret string
	if result.SecretString != nil {
		secretString = *result.SecretString
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
		len, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary)
		if err != nil {
			fmt.Println("Base64 Decode Error:", err)
			// return nil
		}
		decodedBinarySecret = string(decodedBinarySecretBytes[:len])
		fmt.Println(decodedBinarySecret)
	}

	json.Unmarshal([]byte(secretString), &config)
	return config[value], nil

}

//SetSecretToEnvironmentVariables set the secret environment variable
func (a AWS) SetSecretToEnvironmentVariables(secretName string) error {
	config, err := a.GetSecret(secretName)

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

//GetQueueUrl obtains a SQS url
func (a AWS) GetQueueUrl(v string) string {
	result, err := a.SQS.GetQueueUrl(&sqs.GetQueueUrlInput{
		QueueName: aws.String(v), // Required
	})
	// resp, err := svc.GetQueueURL(result)
	if err != nil {
		fmt.Println(err)
	}
	return *result.QueueUrl
}

//SendStringMessageToSqs enqueue a message to AWS SQS Queue
func (a AWS) SendStringMessageToSqs(payload string, qURL string) (*string, error) {
	var err error
	result, err := a.SQS.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(payload),
		QueueUrl:    &qURL,
	})

	if err != nil {
		return nil, err
	}

	if result.MessageId == nil {
		err = errors.New("Message was not sent. Payload" + payload)
	}

	return result.MessageId, err
}

//SendStringMessageWithAttributesToSqs enqueue a message via SQS, but also you can set the message attributes
func (a AWS) SendStringMessageWithAttributesToSqs(payload string, qURL string, attributes map[string]interface{}) (*string, error) {
	var err error

	messageAttributes := map[string]*sqs.MessageAttributeValue{}

	// convert the attribute map to aws format
	for k, v := range attributes {
		messageAttributes[k] = &sqs.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(fmt.Sprintf("%s", v)),
		}
	}

	result, err := a.SQS.SendMessage(&sqs.SendMessageInput{
		MessageBody:       aws.String(payload),
		MessageAttributes: messageAttributes,
		QueueUrl:          &qURL,
	})

	if err != nil {
		return nil, err
	}

	if result.MessageId == nil {
		err = errors.New("Message was not sent. Payload" + payload)
	}

	return result.MessageId, err
}
