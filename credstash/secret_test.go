package credstash

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"

	"encoding/base64"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
)

func TestCreateNonce(t *testing.T) {
	expected := []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
	}
	actual := createNonce()

	if !bytes.Equal(expected, actual) {
		t.Errorf("unexecpected nonce.\nexpected: %v\ngot: %v", expected, actual)
	}
}

func TestDecrpytData(t *testing.T) {
	plaintext := []byte("hello world")
	key := bytes.Repeat([]byte("a"), 32)
	km := keyMaterial{Content: encrypt(t, key, createNonce(), plaintext)}

	data, err := decryptData(km, key)
	assertNoError(t, err)

	if !bytes.Equal([]byte(data), plaintext) {
		t.Errorf("decrypting data failed")
	}
}

func TestDecryptWithWrongNonce(t *testing.T) {
	plaintext := []byte("hello world")
	key := bytes.Repeat([]byte("a"), 32)
	nonce := make([]byte, aes.BlockSize)
	rand.Read(nonce)
	km := keyMaterial{Content: encrypt(t, key, nonce, plaintext)}

	// decryptData always uses the same nonce for decryption
	data, err := decryptData(km, key)
	assertNoError(t, err)

	if bytes.Equal([]byte(data), plaintext) {
		t.Errorf("decrypting should have failed")
	}
}

func TestGetDigest(t *testing.T) {
	testCases := []struct {
		desc   string
		digest string
		item   map[string]*dynamodb.AttributeValue
	}{
		{
			digest: "SHA256",
			item:   map[string]*dynamodb.AttributeValue{},
			desc:   "default value",
		},
		{
			digest: "SHA512",
			item: map[string]*dynamodb.AttributeValue{
				"digest": attrValueString("SHA512"),
			},
			desc: "exact value",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			actual := getDigest(tt.item)
			if actual != tt.digest {
				t.Errorf("wrong digest. expected: %s, got: %s", tt.digest, actual)
			}
		})
	}
}

func TestKeyMaterialFromDDBResult(t *testing.T) {
	testCases := []struct {
		desc       string
		item       map[string]*dynamodb.AttributeValue
		km         keyMaterial
		shouldFail bool
	}{
		{
			desc: "all fields",
			item: dummyItemWithAllFields(),
			km: keyMaterial{
				Name:    "test_key",
				Version: "0000000000000000001",
				Digest:  "SHA256",
				Key:     []byte{1, 2, 3, 4},
				Content: []byte{1, 2, 3, 4},
				HMAC:    []byte{1, 2, 3, 4},
			},
		},
		{
			desc:       "wrong key field",
			item:       dummyItemWithWrongKey(),
			shouldFail: true,
		},
		{
			desc:       "missing key field",
			item:       dummyItemWithMissingKey(),
			shouldFail: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			actual, err := keyMaterialFromDBItem(tt.item)

			if tt.shouldFail {
				assertError(t, err)
			} else {
				assertNoError(t, err)
			}

			if !reflect.DeepEqual(tt.km, actual) {
				t.Errorf("unexpected key material\nexpected: %+v\ngot: %+v", tt.km, actual)
			}
		})
	}
}

func TestDecrpytKey(t *testing.T) {
	hmacKey := bytes.Repeat([]byte("a"), 32)
	dataKey := bytes.Repeat([]byte("b"), 32)
	ciphertext := []byte("test blob")
	plainKey := append(dataKey, hmacKey...)

	d := fakeDecrypter{
		plaintext: plainKey,
		testInput: func(in *kms.DecryptInput) {
			if !bytes.Equal(in.CiphertextBlob, ciphertext) {
				t.Error("unexpected ciphertext blob")
			}
		},
	}

	actualDataKey, actualHMACKey, err := decryptKey(d, ciphertext, nil)
	assertNoError(t, err)

	if !bytes.Equal(hmacKey, actualHMACKey) {
		t.Error("wrong hmac key")
	}

	if !bytes.Equal(dataKey, actualDataKey) {
		t.Error("wrong data key")
	}
}

func TestSecret(t *testing.T) {
	password := "test secret"
	req := GetSecretRequest{
		Name:  "test_key",
		Table: "test_table",
	}
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

	s := secret{
		dynamoDB:  db,
		decrypter: decrypt,
	}

	result, err := s.get(req)
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

func dummyItemWithAllFields() map[string]*dynamodb.AttributeValue {
	return map[string]*dynamodb.AttributeValue{
		"name":     attrValueString("test_key"),
		"version":  attrValueString("0000000000000000001"),
		"digest":   attrValueString("SHA256"),
		"hmac":     attrValueHexString([]byte{1, 2, 3, 4}),
		"contents": attrValueB64String([]byte{1, 2, 3, 4}),
		"key":      attrValueB64String([]byte{1, 2, 3, 4}),
	}
}

func dummyItemWithWrongKey() map[string]*dynamodb.AttributeValue {
	item := dummyItemWithAllFields()
	item["key"] = attrValueString("not base64")
	return item
}

func dummyItemWithMissingKey() map[string]*dynamodb.AttributeValue {
	item := dummyItemWithAllFields()
	delete(item, "key")
	return item
}

func attrValueString(v string) *dynamodb.AttributeValue {
	return &dynamodb.AttributeValue{S: aws.String(v)}
}

func attrValueHexString(d []byte) *dynamodb.AttributeValue {
	return attrValueString(hex.EncodeToString(d))
}

func attrValueB64String(d []byte) *dynamodb.AttributeValue {
	return attrValueString(base64.StdEncoding.EncodeToString(d))
}

func encrypt(t *testing.T, key, nonce, data []byte) []byte {
	b, err := aes.NewCipher(key)
	assertNoError(t, err)

	s := cipher.NewCTR(b, nonce)

	result := make([]byte, len(data))
	s.XORKeyStream(result, data)

	return result
}

func assertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}
func assertError(t *testing.T, err error) {
	if err == nil {
		t.Fatalf("should have been an error")
	}
}
