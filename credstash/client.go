package credstash

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws"
)

type Client struct {
	table string

	dynamoDB  dynamoDB
	decrypter decrypter
}

func New(table string, sess *session.Session) *Client {
	return &Client{
		table:     table,
		decrypter: kms.New(sess),
		dynamoDB:  dynamodb.New(sess),
	}
}

func NewWithAssumedRole(table string, sess *session.Session, creds *credentials.Credentials) *Client {
	return &Client{
		table:     table,
		decrypter: kms.New(sess, aws.NewConfig().WithCredentials(creds)),
		dynamoDB:  dynamodb.New(sess, aws.NewConfig().WithCredentials(creds)),
	}
}

func (c *Client) GetSecret(name, table, version string, ctx map[string]string) (string, error) {
	if table == "" {
		table = c.table
	}
	material, err := getKeyMaterial(c.dynamoDB, name, version, table)
	if err != nil {
		return "", err
	}

	dataKey, hmacKey, err := decryptKey(c.decrypter, material.Key, ctx)
	if err != nil {
		return "", err
	}

	if err := checkHMAC(material, hmacKey); err != nil {
		return "", err
	}

	return decryptData(material, dataKey)
}
