package store

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockItem struct {
	Item  map[string]types.AttributeValue
	Error error
}

func keyFromDynamoKey(key map[string]types.AttributeValue) string {
	if v, ok := key["pk"].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func NewTestDynamodbMetadataStore(items map[string]mockItem) *DynamodbMetadataStore {
	return &DynamodbMetadataStore{
		svc: &apiDynamoDBMock{
			GetItemFunc: func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				return mockGetItem(params, items)
			},
			PutItemFunc: func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
				return mockPutItem(params, items)
			},
			UpdateItemFunc: func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
				return mockUpdateItem(params, items)
			},
		},
	}
}

func mockGetItem(i *dynamodb.GetItemInput, items map[string]mockItem) (*dynamodb.GetItemOutput, error) {

	key := keyFromDynamoKey(i.Key)

	item, ok := items[key]
	if !ok {
		// DynamoDB "not found" behavior
		return &dynamodb.GetItemOutput{
			Item: nil,
		}, nil
	}

	if item.Error != nil {
		return nil, item.Error
	}

	return &dynamodb.GetItemOutput{
		Item: item.Item,
	}, nil
}

func mockPutItem(i *dynamodb.PutItemInput, items map[string]mockItem) (*dynamodb.PutItemOutput, error) {

	key := keyFromDynamoKey(i.Item)

	// Simulate conditional write
	if i.ConditionExpression != nil &&
		*i.ConditionExpression == "attribute_not_exists(service)" {

		if _, exists := items[key]; exists {
			return nil, &types.ConditionalCheckFailedException{
				Message: aws.String("item already exists"),
			}
		}
	}

	items[key] = mockItem{
		Item: i.Item,
	}

	return &dynamodb.PutItemOutput{
		Attributes: i.Item, // important!
	}, nil
}

func mockUpdateItem(i *dynamodb.UpdateItemInput, items map[string]mockItem) (*dynamodb.UpdateItemOutput, error) {

	key := keyFromDynamoKey(i.Key)

	item, ok := items[key]
	if !ok {
		item = mockItem{Item: map[string]types.AttributeValue{}}
	}

	// VERY simplified update behavior
	for k, v := range i.ExpressionAttributeValues {
		item.Item[k] = v
	}

	items[key] = item

	return &dynamodb.UpdateItemOutput{
		Attributes: item.Item,
	}, nil
}

func TestDynamodbMetadataStore_Create(t *testing.T) {
	t.Run("Using region override should take precedence over other settings", func(t *testing.T) {
		os.Setenv("CHAMBER_AWS_REGION", "us-east-1")
		defer os.Unsetenv("CHAMBER_AWS_REGION")
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")
		os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
		defer os.Unsetenv("AWS_DEFAULT_REGION")

		s, err := NewDynamodbMetadataStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-east-1", s.config.Region)
	})

	t.Run("Should use AWS_REGION if it is set", func(t *testing.T) {
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")

		s, err := NewDynamodbMetadataStore(context.Background(), 1)
		assert.Nil(t, err)
		assert.Equal(t, "us-west-1", s.config.Region)
	})

	// t.Run("Should create item successfully", func(t *testing.T) {
	// 	os.Setenv("CHAMBER_METADATA_TABLE_NAME", "test-table")
	// 	defer os.Unsetenv("CHAMBER_METADATA_TABLE_NAME")
	//
	// 	items := map[string]mockItem{}
	// 	store, err := NewDynamodbMetadataStore(context.Background(), 1)
	// 	assert.Nil(t, err)
	//
	// 	meta, err := store.Create(context.Background(), "service-a")
	// 	assert.Nil(t, err)
	// 	assert.Equal(t, "service-a", meta.Service)
	//
	// 	// Verify item stored in mock
	// 	stored, ok := items["service-a"]
	// 	assert.True(t, ok)
	// 	assert.Equal(t, "service-a", stored.Item["service"].(*types.AttributeValueMemberS).Value)
	// })
	//
	// t.Run("Should fail when item already exists", func(t *testing.T) {
	// 	os.Setenv("CHAMBER_METADATA_TABLE_NAME", "test-table")
	// 	defer os.Unsetenv("CHAMBER_METADATA_TABLE_NAME")
	//
	// 	store, err := NewDynamodbMetadataStore(context.Background(), 1)
	// 	assert.Nil(t, err)
	//
	// 	_, err = store.Create(context.Background(), "service-a")
	// 	assert.NotNil(t, err)
	//
	// 	// Check for conditional failure type
	// 	var condErr *types.ConditionalCheckFailedException
	// 	assert.ErrorAs(t, err, &condErr)
	// })
}

func TestCreateSuccess(t *testing.T) {

	t.Run("Should fail if table name env is not set", func(t *testing.T) {
		os.Unsetenv("CHAMBER_METADATA_TABLE_NAME")

		items := map[string]mockItem{}
		store := NewTestDynamodbMetadataStore(items)
		_, err := store.Create(context.Background(), "service-a")
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "CHAMBER_METADATA_TABLE_NAME must be set")
	})

	t.Run("Create should return the created dynamodb item for a service", func(t *testing.T) {
		os.Setenv("CHAMBER_METADATA_TABLE_NAME", "test-table")
		os.Setenv("AWS_REGION", "us-west-1")
		defer os.Unsetenv("AWS_REGION")

		items := map[string]mockItem{}
		store := NewTestDynamodbMetadataStore(items)

		meta, err := store.Create(context.Background(), "service-a")
		fmt.Println(meta.Service)
		require.NoError(t, err)
		require.Equal(t, "service-a", meta.Service)
	})
}
