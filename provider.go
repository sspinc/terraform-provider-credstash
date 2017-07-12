package main

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

type config struct {
	region string
	table  string
	profile string
}

var _ terraform.ResourceProvider = provider()

func provider() terraform.ResourceProvider {
	return &schema.Provider{
		DataSourcesMap: map[string]*schema.Resource{
			"credstash_secret": dataSourceSecret(),
		},
		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: schema.MultiEnvDefaultFunc([]string{
					"AWS_REGION",
					"AWS_DEFAULT_REGION",
				}, nil),
				Description: "The region where AWS operations will take place. Examples\n" +
					"are us-east-1, us-west-2, etc.",
				InputDefault: "us-east-1",
			},
			"table": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The DynamoDB table where the secrets are stored.",
				DefaultFunc: func() (interface{}, error) {
					return "credential-store", nil
				},
			},
			"profile": {
				Type:        schema.TypeString,
				Required:    false,
				Description: "The profile that should be used to connect to AWS",	
			},			
		},
		ConfigureFunc: providerConfig,
	}
}

func providerConfig(d *schema.ResourceData) (interface{}, error) {
	cfg := config{
		region: d.Get("region").(string),
		table:  d.Get("table").(string),
	}

	return cfg, nil
}
