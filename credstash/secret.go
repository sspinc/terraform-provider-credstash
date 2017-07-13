package credstash

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/kms"
)

type GetSecretRequest struct {
	Name              string
	Table             string
	Version           string
	Region            string
	Profile           string
	EncryptionContext map[string]string

	AWSSession *session.Session
}

func GetSecret(req GetSecretRequest) (string, error) {
	var sess *session.Session

	if req.AWSSession != nil {
		sess = req.AWSSession
	} else {
		var err error
		if req.Profile != "default" {
			log.Printf("[DEBUG] creates a session for profile: %s", req.Profile)
			sess, err = session.NewSessionWithOptions(session.Options{
				 Config: aws.Config{Region: aws.String(req.Region)},
				 Profile: req.Profile,
				 SharedConfigState: session.SharedConfigEnable,
			})			
			
		} else {
			sess, err = session.NewSession(&aws.Config{Region: aws.String(req.Region)})	
		}				
				
		if err != nil {
			return "", err
		}
	}

	s := secret{
		decrypter: kms.New(sess),
		dynamoDB:  dynamodb.New(sess),
	}

	return s.get(req)
}

type secret struct {
	dynamoDB  dynamoDB
	decrypter decrpyter
}

func (s secret) get(req GetSecretRequest) (string, error) {
	material, err := getKeyMaterial(s.dynamoDB, req.Name, req.Version, req.Table)
	if err != nil {
		return "", err
	}

	dataKey, hmacKey, err := decryptKey(s.decrypter, material.Key, req.EncryptionContext)
	if err != nil {
		return "", err
	}

	if err := checkHMAC(material, hmacKey); err != nil {
		return "", err
	}

	return decryptData(material, dataKey)
}

func decryptData(material keyMaterial, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", nil
	}
	stream := cipher.NewCTR(block, createNonce())
	secret := make([]byte, len(material.Content))
	stream.XORKeyStream(secret, material.Content)

	return string(secret), nil
}

// Create a nonce to be used with counter mode. The nonce always starts from 1.
// This is required to be compatible with the python implementation of
// credstash.
func createNonce() []byte {
	iv := make([]byte, aes.BlockSize)
	iv[len(iv)-1] = 1
	return iv
}

func checkHMAC(material keyMaterial, hmacKey []byte) error {
	f, err := getDigestFunc(material.Digest)
	if err != nil {
		return err
	}

	mac := hmac.New(f, hmacKey)
	mac.Write(material.Content)
	expectedMAC := mac.Sum(nil)

	if !hmac.Equal(expectedMAC, material.HMAC) {
		return fmt.Errorf("Computed HMAC on %s does not match stored HMAC", material.Name)
	}

	return nil
}

func getDigestFunc(digest string) (func() hash.Hash, error) {

	switch digest {
	case "SHA1":
		return sha1.New, nil
	case "SHA224":
		return sha256.New224, nil
	case "SHA256":
		return sha256.New, nil
	case "SHA384":
		return sha512.New384, nil
	case "SHA512":
		return sha512.New, nil
	case "MD5":
		return md5.New, nil
	}

	return nil, fmt.Errorf("digest %s is not supported", digest)
}

func decryptKey(svc decrpyter, ciphertext []byte, ctx map[string]string) (dataKey, hmacKey []byte, err error) {
	encCtx := make(map[string]*string, len(ctx))
	for k, v := range ctx {
		encCtx[k] = aws.String(v)
	}

	params := &kms.DecryptInput{
		CiphertextBlob:    ciphertext,
		EncryptionContext: encCtx,
	}

	out, err := svc.Decrypt(params)
	if err != nil {
		return
	}

	dataKey = out.Plaintext[:32]
	hmacKey = out.Plaintext[32:]
	return
}

func getKeyMaterial(db dynamoDB, name, version, table string) (keyMaterial, error) {
	if version == "" {
		return getLatestVersion(db, name, table)
	}
	return getSpecificVersion(db, name, version, table)
}

func getSpecificVersion(svc dynamoDB, name, version, table string) (keyMaterial, error) {
	params := &dynamodb.GetItemInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			"name":    {S: aws.String(name)},
			"version": {S: aws.String(version)},
		},
	}

	item, err := svc.GetItem(params)
	if err != nil {
		return keyMaterial{}, err
	}

	return keyMaterialFromDBItem(item.Item)
}

func getLatestVersion(svc dynamoDB, name, table string) (keyMaterial, error) {
	query := &dynamodb.QueryInput{
		TableName:              aws.String(table),
		ConsistentRead:         aws.Bool(true),
		Limit:                  aws.Int64(1),
		ScanIndexForward:       aws.Bool(false),
		KeyConditionExpression: aws.String("#N = :nameval"),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":nameval": {S: aws.String(name)},
		},
	}

	out, err := svc.Query(query)
	if err != nil {
		return keyMaterial{}, err
	}

	if *out.Count == 0 {
		return keyMaterial{}, fmt.Errorf("secret with name %s could not be found.", name)
	}

	return keyMaterialFromDBItem(out.Items[0])

}

type keyMaterial struct {
	Name    string
	Version string
	Digest  string

	Content []byte
	HMAC    []byte
	Key     []byte
}

func keyMaterialFromDBItem(item map[string]*dynamodb.AttributeValue) (keyMaterial, error) {
	var err error
	material := keyMaterial{Digest: getDigest(item)}

	if material.Name, err = getString(item, "name"); err != nil {
		return keyMaterial{}, err
	}

	if material.Version, err = getString(item, "version"); err != nil {
		return keyMaterial{}, err
	}

	if material.HMAC, err = getStringAndDecode(item, "hmac", hex.DecodeString); err != nil {
		return keyMaterial{}, err
	}

	if material.Key, err = getStringAndDecode(item, "key", base64.StdEncoding.DecodeString); err != nil {
		return keyMaterial{}, err
	}

	if material.Content, err = getStringAndDecode(item, "contents", base64.StdEncoding.DecodeString); err != nil {
		return keyMaterial{}, err
	}

	return material, nil
}

func getDigest(material map[string]*dynamodb.AttributeValue) string {
	digest := "SHA256"
	if dVal, ok := material["digest"]; ok {
		digest = *dVal.S
	}

	return digest
}

func getString(item map[string]*dynamodb.AttributeValue, key string) (string, error) {
	value, ok := item[key]
	if !ok {
		return "", fmt.Errorf("missing key: %s", key)
	}

	return *value.S, nil
}

func getStringAndDecode(item map[string]*dynamodb.AttributeValue, key string, f func(string) ([]byte, error)) ([]byte, error) {
	s, err := getString(item, key)
	if err != nil {
		return nil, err
	}
	return f(s)
}
