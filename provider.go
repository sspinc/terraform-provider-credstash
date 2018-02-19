package main

import (
	"log"
    "time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/BitPan/terraform-provider-credstash/credstash"
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
			},
			"table": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The DynamoDB table where the secrets are stored.",
				Default:     "credential-store",
			},
			"profile": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     defaultAWSProfile,
				Description: "The profile that should be used to connect to AWS",
			},
			"assume_role": {
        		Type:     schema.TypeSet,
        		Optional: true,
        		MaxItems: 1,
        		Elem: &schema.Resource{
        			Schema: map[string]*schema.Schema{
        				"role_arn": {
        					Type:        schema.TypeString,
        					Optional:    true,
        					Description: "assume_role_role_arn",
        				},

        				"session_name": {
        					Type:        schema.TypeString,
        					Optional:    true,
        					Description: "assume_role_session_name",
        				},

        				"external_id": {
        					Type:        schema.TypeString,
        					Optional:    true,
        					Description: "assume_role_external_id",
        				},

        				"policy": {
        					Type:        schema.TypeString,
        					Optional:    true,
        					Description: "assume_role_policy",
        				},
        			},
        		},
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
	var creds *credentials.Credentials

	assumeRoleList := d.Get("assume_role").(*schema.Set).List()

    if len(assumeRoleList) == 1 {

        assumeRole := assumeRoleList[0].(map[string]interface{})
        sess, err = session.NewSession(&aws.Config{Region: aws.String(region)})

        creds := stscreds.NewCredentials(sess, assumeRole["role_arn"].(string), func(arp *stscreds.AssumeRoleProvider) {
            arp.RoleSessionName = assumeRole["session_name"].(string)
            arp.Duration = 60 * time.Minute
            arp.ExpiryWindow = 30 * time.Second
          })

        dynamoDbClient := dynamodb.New(sess, aws.NewConfig().WithCredentials(creds))

    } else if profile != defaultAWSProfile {
		log.Printf("[DEBUG] creating a session for profile: %s", profile)
		sess, err = session.NewSessionWithOptions(session.Options{
		    AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
			Config:                  aws.Config{Region: aws.String(region)},
			Profile:                 profile,
			SharedConfigState:       session.SharedConfigEnable,
		})
	} else {
		sess, err = session.NewSession(&aws.Config{Region: aws.String(region)})
	}
	if err != nil {
		return nil, err
	}

	log.Printf("[DEBUG] configured credstash for table %s", table)
	return credstash.New(table, sess, creds), nil
}
