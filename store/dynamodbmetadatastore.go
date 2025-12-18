package store

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ensure DynamodbMetadataStore confirms to MetadataStore interface
var _ MetadataStore = &DynamodbMetadataStore{}

// SSMStore implements the Store interface for storing secrets in SSM Parameter
// Store
type DynamodbMetadataStore struct {
	svc    apiDynamoDB
	config aws.Config
}

// NewSSMStore creates a new SSMStore
func NewDynamodbMetadataStore(ctx context.Context, numRetries int) (*DynamodbMetadataStore, error) {
	return dynamodbMetadataStoreUsingRetryer(ctx, numRetries, DefaultRetryMode)
}

// NewDynamoDBMetadataStoreWithMinThrottleDelay creates a new MetadataStore with the aws sdk max retries and min throttle delay are configured.
//
// Deprecated: The AWS SDK no longer supports specifying a minimum throttle delay. Instead, use
// NewDynamoDBMetadataStoreWithRetryMode.
func NewDynamodbMetadataStoreWithMinThrottleDelay(ctx context.Context, numRetries int, minThrottleDelay time.Duration) (*DynamodbMetadataStore, error) {
	return dynamodbMetadataStoreUsingRetryer(ctx, numRetries, DefaultRetryMode)
}

// NewDynamoDBMetadataStoreWithRetryMode creates a new MetadataStore, configuring the underlying AWS SDK with the
// given maximum number of retries and retry mode.
func NewDynamodbMetadataStoreWithRetryMode(ctx context.Context, numRetries int, retryMode aws.RetryMode) (*DynamodbMetadataStore, error) {
	return dynamodbMetadataStoreUsingRetryer(ctx, numRetries, retryMode)
}

func dynamodbMetadataStoreUsingRetryer(ctx context.Context, numRetries int, retryMode aws.RetryMode) (*DynamodbMetadataStore, error) {
	cfg, _, err := getConfig(ctx, numRetries, retryMode)
	if err != nil {
		return nil, err
	}

	svc := dynamodb.NewFromConfig(cfg)

	return &DynamodbMetadataStore{
		svc:    svc,
		config: cfg,
	}, nil
}

func (d *DynamodbMetadataStore) Create(ctx context.Context, service string) (Metadata, error) {
	tableName := os.Getenv("CHAMBER_METADATA_TABLE_NAME")
	if tableName == "" {
		return Metadata{}, fmt.Errorf("CHAMBER_METADATA_TABLE_NAME must be set")
	}

	out, err := d.svc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("CHAMBER_METADATA_TABLE_NAME")),
		Item: map[string]types.AttributeValue{
			"service": &types.AttributeValueMemberS{Value: service},
		},
		ConditionExpression: aws.String("attribute_not_exists(service)"),
	})
	if err != nil {
		return Metadata{}, err
	}

	var item Metadata
	if err := attributevalue.UnmarshalMap(out.Attributes, &item); err != nil {
		return Metadata{}, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return item, nil
}

func (d *DynamodbMetadataStore) Read(ctx context.Context, service string) (Metadata, error) {
	tableName := os.Getenv("CHAMBER_METADATA_TABLE_NAME")
	if tableName == "" {
		return Metadata{}, fmt.Errorf("CHAMBER_METADATA_TABLE_NAME must be set")
	}

	out, err := d.svc.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"service": &types.AttributeValueMemberS{
				Value: service,
			},
		},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read metadata for service %q: %w", service, err)
	}

	if out.Item == nil {
		return Metadata{}, fmt.Errorf("no metadata found for service %q", service)
	}

	var item Metadata
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return Metadata{}, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return item, nil
}

func (d *DynamodbMetadataStore) SetInherits(
	ctx context.Context,
	service string,
	inherits []string,
) error {
	tableName := os.Getenv("CHAMBER_METADATA_TABLE_NAME")
	if tableName == "" {
		return fmt.Errorf("CHAMBER_METADATA_TABLE_NAME must be set")
	}

	if len(inherits) == 0 {
		_, err := d.svc.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"service": &types.AttributeValueMemberS{Value: service},
			},
			UpdateExpression: aws.String("REMOVE inherits"),
		})
		if err != nil {
			return fmt.Errorf(
				"failed to remove inherits for service %q: %w",
				service,
				err,
			)
		}
		return nil
	}

	_, err := d.svc.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"service": &types.AttributeValueMemberS{Value: service},
		},
		UpdateExpression: aws.String("SET inherits = :vals"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":vals": &types.AttributeValueMemberSS{
				Value: inherits,
			},
		},
	})
	if err != nil {
		return fmt.Errorf(
			"failed to set inherits for service %q: %w",
			service,
			err,
		)
	}

	return nil
}

func (d *DynamodbMetadataStore) AddInherits(
	ctx context.Context,
	service string,
	inherits []string,
) error {
	tableName := os.Getenv("CHAMBER_METADATA_TABLE_NAME")
	if tableName == "" {
		return fmt.Errorf("CHAMBER_METADATA_TABLE_NAME must be set")
	}

	_, err := d.svc.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"service": &types.AttributeValueMemberS{
				Value: service,
			},
		},
		UpdateExpression: aws.String("ADD inherits :vals"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":vals": &types.AttributeValueMemberSS{
				Value: inherits,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add inherits for %q: %w", service, err)
	}

	return nil
}

func (d *DynamodbMetadataStore) DeleteInherits(
	ctx context.Context,
	service string,
	inherits []string,
) error {
	tableName := os.Getenv("CHAMBER_METADATA_TABLE_NAME")
	if tableName == "" {
		return fmt.Errorf("CHAMBER_METADATA_TABLE_NAME must be set")
	}

	_, err := d.svc.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"service": &types.AttributeValueMemberS{
				Value: service,
			},
		},
		UpdateExpression: aws.String("DELETE inherits :vals"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":vals": &types.AttributeValueMemberSS{
				Value: inherits,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete inherits for %q: %w", service, err)
	}

	return nil
}
