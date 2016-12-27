# Terraform provider for credstash secrets

Read secrets stored with [credstash][credstash].

## Install

1. [Download the binary][provider_binary]
2. Create a terraformrc file

        # ~/.terraformrc
        providers {
            credstash = "/path/to/bin/terraform-provider-credstash"
        }
3. Profit

### From source

    $ go get -v -u github.com/sspinc/terraform-provider-credstash

## Usage

```hcl
provider "credstash" {
    table  = "credential-store"
    region = "us-east-1"
}

data "credstash_secret" "rds_password" {
    name = "rds_password"
}

data "credstash_secret" "my_secret" {
    name    = "some_secret"
    version = "0000000000000000001"
}

resource "aws_db_instance" "postgres" {
    password = "${data.credstash_secret.rds_password.value}"

    # other important attributes
}
```

## AWS credentials

AWS credentials are not directly set. Use one of the methods discussed
[here][awscred].

[credstash]: https://github.com/fugue/credstash
[awscred]: https://github.com/aws/aws-sdk-go#configuring-credentials
[provider_binary]: https://github.com/sspinc/terraform-provider-credstash/releases/latest
