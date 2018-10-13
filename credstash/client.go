package credstash

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
)

type Client struct {
	table string

	dynamoDB  dynamoDB
	decrpyter decrpyter
}

func New(table string, sess *session.Session) *Client {
	return &Client{
		table:     table,
		decrpyter: kms.New(sess),
		dynamoDB:  dynamodb.New(sess),
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

	dataKey, hmacKey, err := decryptKey(c.decrpyter, material.Key, ctx)
	if err != nil {
		return "", err
	}

	if err := checkHMAC(material, hmacKey); err != nil {
		return "", err
	}

	return decryptData(material, dataKey)
}
