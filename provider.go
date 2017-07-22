package main

import (
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/sspinc/terraform-provider-credstash/credstash"
)

var _ terraform.ResourceProvider = provider()

const defaultAWSProfile = "default"

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
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: func() (interface{}, error) {
					return defaultAWSProfile, nil
				},
				Description:  "The profile that should be used to connect to AWS",
				InputDefault: "default",
			},
		},
		ConfigureFunc: providerConfig,
	}
}

func providerConfig(d *schema.ResourceData) (interface{}, error) {
	region := d.Get("region").(string)
	table := d.Get("table").(string)
	profile := d.Get("profile").(string)

	var sess *session.Session
	var err error
	if profile != defaultAWSProfile {
		log.Printf("[DEBUG] Creating a session for profile: %s", profile)
		sess, err = session.NewSessionWithOptions(session.Options{
			Config:            aws.Config{Region: aws.String(region)},
			Profile:           profile,
			SharedConfigState: session.SharedConfigEnable,
		})
	} else {
		sess, err = session.NewSession(&aws.Config{Region: aws.String(region)})
	}
	if err != nil {
		return nil, err
	}

	log.Printf("[DEBUG] Configured credstash for table %s", table)
	return credstash.New(table, sess), nil
}
