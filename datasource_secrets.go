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
			"table": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "name of DynamoDB table where the secrets are stored",
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
	client := meta.(*credstash.Client)

	name := d.Get("name").(string)
	version := d.Get("version").(string)
	table := d.Get("table").(string)

	context := make(map[string]string)
	for k, v := range d.Get("context").(map[string]interface{}) {
		context[k] = fmt.Sprintf("%v", v)
	}

	log.Printf("[DEBUG] Getting secret for name=%q table=%q version=%q context=%+v", name, table, version, context)
	value, err := client.GetSecret(name, table, version, context)
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
