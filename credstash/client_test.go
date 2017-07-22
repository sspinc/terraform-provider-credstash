package credstash

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
)

func TestClient_GetSecret(t *testing.T) {
	password := "test secret"
	item, key := createItem(t, password)
	decrypt := fakeDecrypter{plaintext: key}
	db := fakeDynamoDB{
		item: item,
		testQueryInput: func(in *dynamodb.QueryInput) {
			if aws.StringValue(in.TableName) != "test_table" {
				t.Error("table was not set up for query")
			}
		},
		testGetItemInput: func(in *dynamodb.GetItemInput) {
			t.Fatal("version was not specified but GetItem was used")
		},
	}

	c := &Client{
		decrpyter: decrypt,
		dynamoDB:  db,
		table:     "test_table",
	}

	result, err := c.GetSecret("test_key", "", nil)
	assertNoError(t, err)

	if result != password {
		t.Errorf("decrpyt failed.\nexpected result: %s\ngot: %s", password, result)
	}
}

type fakeDynamoDB struct {
	testQueryInput   func(*dynamodb.QueryInput)
	testGetItemInput func(*dynamodb.GetItemInput)
	item             map[string]*dynamodb.AttributeValue
}

func (db fakeDynamoDB) GetItem(in *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	if db.testGetItemInput != nil {
		db.testGetItemInput(in)
	}

	return &dynamodb.GetItemOutput{
		Item: db.item,
	}, nil
}

func (db fakeDynamoDB) Query(in *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	if db.testQueryInput != nil {
		db.testQueryInput(in)
	}

	return &dynamodb.QueryOutput{
		Count: aws.Int64(1),
		Items: []map[string]*dynamodb.AttributeValue{db.item},
	}, nil
}

type fakeDecrypter struct {
	testInput func(*kms.DecryptInput)
	plaintext []byte
}

func (d fakeDecrypter) Decrypt(in *kms.DecryptInput) (*kms.DecryptOutput, error) {
	if d.testInput != nil {
		d.testInput(in)
	}

	return &kms.DecryptOutput{
		Plaintext: d.plaintext,
	}, nil
}

func createItem(t *testing.T, plaintext string) (map[string]*dynamodb.AttributeValue, []byte) {
	hmacKey := bytes.Repeat([]byte("a"), 32)
	dataKey := bytes.Repeat([]byte("b"), 32)
	plaintextData := []byte(plaintext)
	plainKey := append(dataKey, hmacKey...)
	encryptedData := encrypt(t, dataKey, createNonce(), plaintextData)

	h := hmac.New(sha256.New, hmacKey)
	h.Write(encryptedData)
	hmac := hex.EncodeToString(h.Sum(nil))

	content := base64.StdEncoding.EncodeToString(encryptedData)
	item := map[string]*dynamodb.AttributeValue{
		"name":     attrValueString("test_key"),
		"version":  attrValueString("0000000000000000001"),
		"digest":   attrValueString("SHA256"),
		"hmac":     attrValueString(hmac),
		"contents": attrValueString(content),
		"key":      attrValueB64String(plainKey),
	}

	return item, plainKey
}
