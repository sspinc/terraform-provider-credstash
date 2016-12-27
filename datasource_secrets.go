package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/sspinc/terraform-provider-credstash/credstash"
)

func dataSourceSecret() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceSecretRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "name of the secret",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "version of the secrets",
				Default:     "",
			},
			"context": {
				Type:        schema.TypeMap,
				Optional:    true,
				Description: "encryption context for the secret",
			},
			"value": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "value of the secret",
				Sensitive:   true,
			},
		},
	}
}

func dataSourceSecretRead(d *schema.ResourceData, meta interface{}) error {
	cfg := meta.(config)

	context := make(map[string]string)
	for k, v := range d.Get("context").(map[string]interface{}) {
		context[k] = fmt.Sprintf("%v", v)
	}

	req := credstash.GetSecretRequest{
		Name:              d.Get("name").(string),
		Version:           d.Get("version").(string),
		EncryptionContext: context,
		Table:             cfg.table,
		Region:            cfg.region,
	}

	log.Printf("[DEBUG] Read secret: %+v", req)
	value, err := credstash.GetSecret(req)
	if err != nil {
		return err
	}

	d.Set("value", value)
	d.SetId(hash(value))

	return nil
}

func hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
}
