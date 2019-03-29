package credstash

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
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

func TestDecryptData(t *testing.T) {
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
		{
			desc: "binary HMAC field",
			item: dummyItemWithBinaryHMAC("01020304"),
			km: keyMaterial{
				Name:    "test_key",
				Version: "0000000000000000001",
				Digest:  "SHA256",
				Key:     []byte{1, 2, 3, 4},
				Content: []byte{1, 2, 3, 4},
				HMAC:    []byte{1, 2, 3, 4},
			},
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

func TestDecryptKey(t *testing.T) {
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

func dummyItemWithBinaryHMAC(hmac string) map[string]*dynamodb.AttributeValue {
	item := dummyItemWithAllFields()
	item["hmac"] = &dynamodb.AttributeValue{B: []byte(hmac)}
	return item
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
