package credstash

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

type Client struct {
	table string

	dynamoDB  dynamoDB
	decrpyter decrpyter
}

func New(table string, sess *session.Session, creds *credentials.Credentials) *Client {
	return &Client{
		table:     table,
		decrpyter: kms.New(sess, aws.NewConfig().WithCredentials(creds)),
		dynamoDB:  dynamodb.New(sess, aws.NewConfig().WithCredentials(creds)),
	}
}

func (c *Client) GetSecret(name, version string, ctx map[string]string) (string, error) {
	material, err := getKeyMaterial(c.dynamoDB, name, version, c.table)
	if err != nil {
		return "", err
	}

	dataKey, hmacKey, err := decryptKey(c.decrpyter, material.Key, ctx)
	if err != nil {
		return "", err
	}

	if err := checkHMAC(material, hmacKey); err != nil {
		return "", err
	}

	return decryptData(material, dataKey)
}
