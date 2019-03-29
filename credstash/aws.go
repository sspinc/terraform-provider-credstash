package credstash

import (
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
)

type dynamoDB interface {
	GetItem(*dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error)
	Query(*dynamodb.QueryInput) (*dynamodb.QueryOutput, error)
}

type decrypter interface {
	Decrypt(*kms.DecryptInput) (*kms.DecryptOutput, error)
}

var _ dynamoDB = (*dynamodb.DynamoDB)(nil)
var _ decrypter = (*kms.KMS)(nil)
